package integration

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/mailer"
	"github.com/WhereverApp2026/wherever-backend/internal/privacidad"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
)

// Pruebas del correo real (spec nv-06 "Integration/Mailer", "Exportación" y "Respuesta con
// servidor lento") con Mailpit en Testcontainers y la app real.

const linkBase = "http://localhost:8080"

var tokenInLink = regexp.MustCompile(`#token=([A-Za-z0-9_\-]+)`)

// smtpTo arma el adaptador real hacia host:port sin TLS (Mailpit, solo fuera de producción).
func smtpTo(host string, port int, log *slog.Logger) *mailer.SMTP {
	return mailer.NewSMTP(mailer.SMTPConfig{Host: host, Port: port, TLSMode: mailer.TLSNone, From: "no-responder@wherever.test"},
		mailer.NewLinks(linkBase), log)
}

// mailEnv arma la app con la cola real hacia `host:port` y el envío directo para el trabajo.
// `sync` permite cambiar el mailer del trabajo nocturno (por ejemplo, uno que falla).
func mailEnv(t *testing.T, host string, port int, timeout time.Duration, sync func(direct mailer.Mailer) mailer.Mailer) *authtest.Env {
	t.Helper()
	require.NoError(t, stack.Mailpit.Clear(context.Background()))
	return authtest.NewWithMail(t, mongoClient, redisClient, testutil.Config(config.EnvDev), func(log *slog.Logger) (mailer.Mailer, mailer.Mailer) {
		direct := mailer.NewRetrying(smtpTo(host, port, log), mailer.DefaultAttempts, 10*time.Millisecond, timeout)
		queue := mailer.NewAsync(direct, 2, 50, log)
		t.Cleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			_ = queue.Shutdown(ctx)
		})
		jobMailer := mailer.Mailer(direct)
		if sync != nil {
			jobMailer = sync(direct)
		}
		return queue, jobMailer
	})
}

func mailpitEnv(t *testing.T) *authtest.Env {
	t.Helper()
	return mailEnv(t, stack.Mailpit.SMTPHost, stack.Mailpit.Port(), 5*time.Second, nil)
}

func waitMail(t *testing.T, to string, n int) []testutil.MailpitMessage {
	t.Helper()
	msgs, err := stack.Mailpit.WaitFor(context.Background(), to, n, 10*time.Second)
	require.NoError(t, err)
	require.Len(t, msgs, n, "correos para %s", to)
	return msgs
}

// withLink elige el correo cuyo enlace lleva esa ruta (Mailpit los lista del más nuevo al más viejo).
func withLink(t *testing.T, msgs []testutil.MailpitMessage, path string) testutil.MailpitMessage {
	t.Helper()
	for _, m := range msgs {
		if regexp.MustCompile(regexp.QuoteMeta(linkBase + path + "#token=")).MatchString(m.Text) {
			return m
		}
	}
	require.Fail(t, "no llegó el correo con "+path)
	return testutil.MailpitMessage{}
}

func linkToken(t *testing.T, msg testutil.MailpitMessage) string {
	t.Helper()
	m := tokenInLink.FindStringSubmatch(msg.Text)
	require.Len(t, m, 2, "el correo trae el enlace con #token=")
	return m[1]
}

func TestMailer_RegisterSendsVerification(t *testing.T) {
	env := mailpitEnv(t)
	app := env.App()
	registerHTTP(t, app, lu)

	msg := waitMail(t, lu, 1)[0]

	assert.Equal(t, "Confirma tu correo en Wherever", msg.Subject)
	assert.Contains(t, msg.Text, "Hola, Lú:")
	assert.Contains(t, msg.Text, linkBase+"/verificar#token=")
	assert.NotContains(t, msg.Text, "?token=", "el token nunca va en la consulta")
	verify := sendJSON(t, app, http.MethodPost, "/api/v1/auth/verificar-correo", map[string]string{"token": linkToken(t, msg)}, "")
	assert.Equal(t, http.StatusOK, verify.StatusCode)
}

func TestMailer_RecoverForExistingEmail(t *testing.T) {
	env := mailpitEnv(t)
	env.CreateUser(t, ana)
	app := env.App()

	require.Equal(t, http.StatusAccepted, sendJSON(t, app, http.MethodPost, "/api/v1/auth/recuperar", map[string]string{"correo": ana}, "").StatusCode)

	msg := waitMail(t, ana, 1)[0]
	assert.Contains(t, msg.Text, linkBase+"/restablecer#token=")
	reset := sendJSON(t, app, http.MethodPost, "/api/v1/auth/restablecer", map[string]any{"token": linkToken(t, msg), "password_nuevo": "NuevaClave9"}, "")
	assert.Equal(t, http.StatusOK, reset.StatusCode)
}

func TestMailer_RecoverForUnknownEmailSendsNothing(t *testing.T) {
	env := mailpitEnv(t)
	env.CreateUser(t, ana)
	app := env.App()

	known := sendJSON(t, app, http.MethodPost, "/api/v1/auth/recuperar", map[string]string{"correo": ana}, "")
	unknown := sendJSON(t, app, http.MethodPost, "/api/v1/auth/recuperar", map[string]string{"correo": "nadie@wherever.test"}, "")

	assert.Equal(t, known.StatusCode, unknown.StatusCode)
	assert.Equal(t, readAll(t, known), readAll(t, unknown))
	waitMail(t, ana, 1)
	msgs, err := stack.Mailpit.Messages(context.Background(), "nadie@wherever.test")
	require.NoError(t, err)
	assert.Empty(t, msgs)
}

func TestMailer_ChangeEmailVerifiesTheNewAddress(t *testing.T) {
	env := mailpitEnv(t)
	app := env.App()
	access := registerHTTP(t, app, lu)
	waitMail(t, lu, 1)

	resp := sendJSON(t, app, http.MethodPut, "/api/v1/usuarios/yo/correo", map[string]string{"correo_nuevo": "nueva@wherever.test", "password": authtest.Password}, access)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	msg := waitMail(t, "nueva@wherever.test", 1)[0]
	assert.Contains(t, msg.Text, "/verificar#token=")
}

func exportFor(t *testing.T, env *authtest.Env, email string) bson.ObjectID {
	t.Helper()
	_, claims := register(t, env, email)
	require.NoError(t, env.Account.RequestExport(context.Background(), claims))
	return stored(t, env, claims).ID
}

func exportDoc(t *testing.T, env *authtest.Env, userID bson.ObjectID) privacidad.Export {
	t.Helper()
	var doc privacidad.Export
	require.NoError(t, env.Mongo.DB().Collection(privacidad.ExportsCollection).FindOne(context.Background(), bson.D{{Key: "usuario_id", Value: userID}}).Decode(&doc))
	return doc
}

func TestMailer_ExportNoticeSentAnd24hFromNotice(t *testing.T) {
	env := mailpitEnv(t)
	userID := exportFor(t, env, lu)

	res, err := env.Job.RunOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, res.Exported)
	assert.Equal(t, 1, res.Notified)

	msg := withLink(t, waitMail(t, lu, 2), "/exportacion")
	doc := exportDoc(t, env, userID)
	require.NotNil(t, doc.NoticeSentAt)
	assert.WithinDuration(t, doc.NoticeSentAt.Add(privacidad.DownloadWindow), doc.ExpiresAt, time.Second)
	data, err := env.Account.DownloadExport(context.Background(), linkToken(t, msg))
	require.NoError(t, err)
	assert.Contains(t, data, lu)
}

// recording falla mientras `fail` esté prendido y guarda el token de cada intento.
type recording struct {
	inner  mailer.Mailer
	fail   atomic.Bool
	mu     sync.Mutex
	tokens []string
}

func (r *recording) Send(ctx context.Context, msg mailer.Message) error {
	r.mu.Lock()
	r.tokens = append(r.tokens, msg.Token)
	r.mu.Unlock()
	if r.fail.Load() {
		return errors.New("servidor de correo caído")
	}
	return r.inner.Send(ctx, msg)
}

// Con el servidor caído el aviso no sale; la corrida siguiente lo reintenta con un enlace
// nuevo y el del intento fallido ya no descarga.
func TestMailer_ExportNoticeRetriedWithNewToken(t *testing.T) {
	rec := &recording{}
	rec.fail.Store(true)
	env := mailEnv(t, stack.Mailpit.SMTPHost, stack.Mailpit.Port(), 5*time.Second, func(direct mailer.Mailer) mailer.Mailer {
		rec.inner = direct
		return rec
	})
	userID := exportFor(t, env, lu)

	first, err := env.Job.RunOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, first.NoticeFailed)
	assert.Nil(t, exportDoc(t, env, userID).NoticeSentAt)
	require.Len(t, rec.tokens, 1)
	failedToken := rec.tokens[0]

	rec.fail.Store(false)
	second, err := env.Job.RunOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, second.Notified)
	msg := withLink(t, waitMail(t, lu, 2), "/exportacion")
	newToken := linkToken(t, msg)
	assert.NotEqual(t, failedToken, newToken)

	_, err = env.Account.DownloadExport(context.Background(), failedToken)
	assert.Error(t, err, "el enlace del intento fallido quedó invalidado")
	_, err = env.Account.DownloadExport(context.Background(), newToken)
	assert.NoError(t, err)
}

func TestExport_WithoutNoticeExpiresAt72h(t *testing.T) {
	rec := &recording{}
	rec.fail.Store(true)
	env := mailEnv(t, stack.Mailpit.SMTPHost, stack.Mailpit.Port(), 5*time.Second, func(direct mailer.Mailer) mailer.Mailer {
		rec.inner = direct
		return rec
	})
	userID := exportFor(t, env, lu)

	_, err := env.Job.RunOnce(context.Background())
	require.NoError(t, err)

	doc := exportDoc(t, env, userID)
	require.NotNil(t, doc.ReadyAt)
	assert.Nil(t, doc.NoticeSentAt)
	assert.WithinDuration(t, doc.ReadyAt.Add(privacidad.ReadyMaxAge), doc.ExpiresAt, time.Second)
}

// Si el aviso sale tarde (60 h después de armarse), la descarga no pasa de las 72 h.
func TestExport_NoticeWindowNeverPasses72h(t *testing.T) {
	rec := &recording{}
	rec.fail.Store(true)
	env := mailEnv(t, stack.Mailpit.SMTPHost, stack.Mailpit.Port(), 5*time.Second, func(direct mailer.Mailer) mailer.Mailer {
		rec.inner = direct
		return rec
	})
	userID := exportFor(t, env, lu)
	_, err := env.Job.RunOnce(context.Background())
	require.NoError(t, err)
	readyAt := time.Now().UTC().Add(-60 * time.Hour).Truncate(time.Millisecond)
	_, err = env.Mongo.DB().Collection(privacidad.ExportsCollection).UpdateOne(context.Background(),
		bson.D{{Key: "usuario_id", Value: userID}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "lista_en", Value: readyAt}, {Key: "expira_en", Value: readyAt.Add(privacidad.ReadyMaxAge)}}}})
	require.NoError(t, err)

	rec.fail.Store(false)
	res, err := env.Job.RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, res.Notified)

	doc := exportDoc(t, env, userID)
	assert.WithinDuration(t, readyAt.Add(privacidad.ReadyMaxAge), doc.ExpiresAt, time.Second, "12 h y no 24 h")
}

// slowSMTP acepta conexiones y nunca saluda: simula un servidor que tarda.
func slowSMTP(t *testing.T) (string, int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	var conns []net.Conn
	var mu sync.Mutex
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			conns = append(conns, c)
			mu.Unlock()
		}
	}()
	t.Cleanup(func() {
		_ = ln.Close()
		mu.Lock()
		defer mu.Unlock()
		for _, c := range conns {
			_ = c.Close()
		}
	})
	addr, ok := ln.Addr().(*net.TCPAddr)
	require.True(t, ok)
	return "127.0.0.1", addr.Port
}

func TestMailer_SlowOrDownServerDoesNotChangeResponses(t *testing.T) {
	slowHost, slowPort := slowSMTP(t)
	for name, target := range map[string]struct {
		host string
		port int
	}{
		"lento": {slowHost, slowPort},
		"caído": {"127.0.0.1", 1},
	} {
		t.Run(name, func(t *testing.T) {
			env := mailEnv(t, target.host, target.port, 5*time.Second, nil)
			env.CreateUser(t, ana)
			app := env.App()

			start := time.Now()
			reg := sendJSON(t, app, http.MethodPost, "/api/v1/usuarios", registerBody(lu), "")
			regTook := time.Since(start)
			start = time.Now()
			rec := sendJSON(t, app, http.MethodPost, "/api/v1/auth/recuperar", map[string]string{"correo": ana}, "")
			recTook := time.Since(start)

			assert.Equal(t, http.StatusCreated, reg.StatusCode)
			assert.Equal(t, http.StatusAccepted, rec.StatusCode)
			assert.Less(t, regTook, 500*time.Millisecond)
			assert.Less(t, recTook, 500*time.Millisecond)
		})
	}
}

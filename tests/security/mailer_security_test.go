package security

import (
	"bufio"
	"context"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/mailer"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
)

// Pruebas de seguridad del correo (spec nv-06 "Security/Mailer 4").

var linkToken = regexp.MustCompile(`#token=([A-Za-z0-9_\-]+)`)

func smtpMailer(host string, port int, user string, log *slog.Logger) *mailer.SMTP {
	return mailer.NewSMTP(mailer.SMTPConfig{
		Host: host, Port: port, User: user, Password: "no-se-debe-enviar", TLSMode: mailer.TLSNone, From: "no-responder@wherever.test",
	}, mailer.NewLinks("http://localhost:8080"), log)
}

func mailSecurityEnv(t *testing.T, host string, port int) *authtest.Env {
	t.Helper()
	require.NoError(t, mailpit.Clear(context.Background()))
	return authtest.NewWithMail(t, mongoClient, redisClient, testutil.Config(config.EnvStaging), func(log *slog.Logger) (mailer.Mailer, mailer.Mailer) {
		direct := mailer.NewRetrying(smtpMailer(host, port, "", log), mailer.DefaultAttempts, 10*time.Millisecond, 2*time.Second)
		queue := mailer.NewAsync(direct, 2, 50, log)
		t.Cleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = queue.Shutdown(ctx)
		})
		return queue, direct
	})
}

func TestMailerSecurity_NoTokenLinkOrBodyInLogs(t *testing.T) {
	for name, target := range map[string]struct {
		host string
		port int
	}{
		"servidor arriba": {mailpit.SMTPHost, mailpit.Port()},
		"servidor caído":  {"127.0.0.1", 1},
	} {
		t.Run(name, func(t *testing.T) {
			env := mailSecurityEnv(t, target.host, target.port)
			env.CreateUser(t, victim)
			app := env.App()
			require.Equal(t, http.StatusAccepted, send(t, app, http.MethodPost, "/api/v1/auth/recuperar", map[string]string{"correo": victim}, "").status)
			require.Eventually(t, func() bool { return strings.Contains(env.Logs.String(), "correo ") }, 10*time.Second, 50*time.Millisecond,
				"el resultado del envío queda registrado")

			logs := env.Logs.String()
			assert.NotContains(t, logs, "#token=")
			assert.NotContains(t, logs, "/restablecer")
			assert.NotContains(t, logs, "Pediste cambiar tu contraseña", "nunca el cuerpo")
			assert.NotContains(t, logs, victim, "el destinatario va enmascarado")
			if msgs, err := mailpit.Messages(context.Background(), victim); err == nil && len(msgs) > 0 {
				m := linkToken.FindStringSubmatch(msgs[0].Text)
				require.Len(t, m, 2)
				assert.NotContains(t, logs, m[1])
			}
		})
	}
}

// plainServer es un servidor SMTP mínimo sin TLS que ofrece AUTH y anota lo que recibe.
func plainServer(t *testing.T) (int, func() []string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	var mu sync.Mutex
	var lines []string
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer func() { _ = c.Close() }()
				w := bufio.NewWriter(c)
				r := bufio.NewReader(c)
				reply := func(s string) { _, _ = w.WriteString(s + "\r\n"); _ = w.Flush() }
				reply("220 prueba")
				for {
					line, err := r.ReadString('\n')
					if err != nil {
						return
					}
					mu.Lock()
					lines = append(lines, strings.TrimSpace(line))
					mu.Unlock()
					switch cmd := strings.ToUpper(strings.Fields(line + " x")[0]); cmd {
					case "EHLO", "HELO":
						reply("250-prueba")
						reply("250 AUTH PLAIN LOGIN")
					case "QUIT":
						reply("221 adiós")
						return
					default:
						reply("250 ok")
					}
				}
			}(c)
		}
	}()
	t.Cleanup(func() { _ = ln.Close() })
	addr, ok := ln.Addr().(*net.TCPAddr)
	require.True(t, ok)
	return addr.Port, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), lines...)
	}
}

// Aunque el servidor ofrezca AUTH y esté en localhost (donde PlainAuth sí mandaría la
// contraseña), sin TLS el adaptador no autentica ni envía nada.
func TestMailerSecurity_NoAuthOverPlainChannel(t *testing.T) {
	port, received := plainServer(t)
	log, _ := testutil.Logger()
	m := smtpMailer("127.0.0.1", port, "usuario", log)

	err := m.Send(context.Background(), mailer.Message{To: victim, Kind: mailer.KindVerifyEmail, Token: "t"})

	require.Error(t, err)
	for _, line := range received() {
		assert.NotContains(t, strings.ToUpper(line), "AUTH")
		assert.NotContains(t, line, "MAIL FROM")
	}
}

func TestMailerSecurity_HeaderInjectionRejected(t *testing.T) {
	require.NoError(t, mailpit.Clear(context.Background()))
	log, _ := testutil.Logger()
	m := smtpMailer(mailpit.SMTPHost, mailpit.Port(), "", log)

	err := m.Send(context.Background(), mailer.Message{To: "a@wherever.test\r\nBcc: espia@wherever.test", Kind: mailer.KindVerifyEmail, Token: "t"})

	assert.ErrorIs(t, err, mailer.ErrHeaderInjection)
	time.Sleep(200 * time.Millisecond)
	msgs, err := mailpit.Messages(context.Background(), "espia@wherever.test")
	require.NoError(t, err)
	assert.Empty(t, msgs)
}

func TestMailerSecurity_TemplatesEscapeHTML(t *testing.T) {
	env := mailSecurityEnv(t, mailpit.SMTPHost, mailpit.Port())
	app := env.App()
	body := registration(victim, "1995-05-17")
	body["nombre"] = `<img src=x onerror=alert(1)>`
	require.Equal(t, http.StatusCreated, send(t, app, http.MethodPost, "/api/v1/usuarios", body, "").status)

	msgs, err := mailpit.WaitFor(context.Background(), victim, 1, 10*time.Second)
	require.NoError(t, err)
	require.Len(t, msgs, 1)

	assert.NotContains(t, msgs[0].HTML, "<img src=x")
	assert.Contains(t, msgs[0].HTML, "&lt;img")
}

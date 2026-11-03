package security

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/mailer"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
)

// Pruebas de seguridad del módulo auth (spec nv-04 "Security/Auth 8").

const victim = "victima@wherever.test"

var deviceJSON = map[string]string{"modelo": "Pixel 8", "sistema": "Android 15"}

func authEnv(t *testing.T) *authtest.Env {
	t.Helper()
	return authtest.New(t, mongoClient, redisClient, testutil.Config(config.EnvProd))
}

type reply struct {
	status  int
	body    string
	headers http.Header
}

func send(t *testing.T, app *fiber.App, method, path string, body any, token string) reply {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	if token != "" {
		req.Header.Set(fiber.HeaderAuthorization, "Bearer "+token)
	}
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return reply{status: resp.StatusCode, body: string(raw), headers: resp.Header}
}

func login(t *testing.T, app *fiber.App, email, password string) reply {
	t.Helper()
	return send(t, app, http.MethodPost, "/api/v1/auth/login", map[string]any{"correo": email, "password": password, "dispositivo": deviceJSON}, "")
}

func tokens(t *testing.T, r reply) (access, refresh string) {
	t.Helper()
	require.Equal(t, http.StatusOK, r.status, r.body)
	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(r.body), &body))
	return str(t, body["access_token"]), str(t, body["refresh_token"])
}

// withoutTrace quita el traza_id, lo único que cambia entre dos respuestas iguales.
func withoutTrace(t *testing.T, body string) map[string]any {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(body), &out))
	delete(out, "traza_id")
	return out
}

func TestAuthSecurity_RecoverRespondsTheSameWithOrWithoutAccount(t *testing.T) {
	env := authEnv(t)
	env.CreateUser(t, victim)
	app := env.App()

	known := send(t, app, http.MethodPost, "/api/v1/auth/recuperar", map[string]string{"correo": victim}, "")
	unknown := send(t, app, http.MethodPost, "/api/v1/auth/recuperar", map[string]string{"correo": "nadie@wherever.test"}, "")

	assert.Equal(t, http.StatusAccepted, known.status)
	assert.Equal(t, known.status, unknown.status)
	assert.Equal(t, known.body, unknown.body)
	assert.Len(t, env.Mail.Sent(), 1, "solo la cuenta existente recibe correo")
}

func TestAuthSecurity_LoginDoesNotRevealUnknownEmail(t *testing.T) {
	env := authEnv(t)
	env.CreateUser(t, victim)
	app := env.App()

	wrong := login(t, app, victim, "Incorrecta1")
	unknown := login(t, app, "nadie@wherever.test", "Incorrecta1")

	assert.Equal(t, http.StatusUnauthorized, wrong.status)
	assert.Equal(t, wrong.status, unknown.status)
	assert.Equal(t, withoutTrace(t, wrong.body), withoutTrace(t, unknown.body))
}

func TestAuthSecurity_NoTokensOrPasswordsInLogs(t *testing.T) {
	env := authEnv(t)
	env.CreateUser(t, victim)
	app := env.App()
	access, refresh := tokens(t, login(t, app, victim, authtest.Password))
	login(t, app, victim, "Incorrecta1")
	_, rotated := tokens(t, send(t, app, http.MethodPost, "/api/v1/auth/refresh", map[string]string{"refresh_token": refresh}, ""))
	send(t, app, http.MethodPost, "/api/v1/auth/refresh", map[string]string{"refresh_token": refresh}, "") // reutilización
	send(t, app, http.MethodPost, "/api/v1/auth/recuperar", map[string]string{"correo": victim}, "")
	link, ok := env.Mail.Last(victim, mailer.KindPasswordReset)
	require.True(t, ok)
	send(t, app, http.MethodPost, "/api/v1/auth/restablecer", map[string]any{"token": link.Token, "password_nuevo": "NuevaClave9"}, "")
	send(t, app, http.MethodPost, "/api/v1/auth/logout", nil, access)

	logs := env.Logs.String()
	require.NotEmpty(t, logs)
	for name, secret := range map[string]string{
		"access": access, "refresh": refresh, "refresh rotado": rotated, "enlace": link.Token,
		"contraseña": authtest.Password, "contraseña mala": "Incorrecta1", "contraseña nueva": "NuevaClave9",
	} {
		assert.NotContains(t, logs, secret, name)
	}
}

func TestAuthSecurity_LinkWorksOnlyOnce(t *testing.T) {
	env := authEnv(t)
	user := env.CreateUser(t, victim)
	app := env.App()
	require.NoError(t, env.Service.SendVerification(t.Context(), user))
	link, ok := env.Mail.Last(victim, mailer.KindVerifyEmail)
	require.True(t, ok)

	first := send(t, app, http.MethodPost, "/api/v1/auth/verificar-correo", map[string]string{"token": link.Token}, "")
	second := send(t, app, http.MethodPost, "/api/v1/auth/verificar-correo", map[string]string{"token": link.Token}, "")
	forged := send(t, app, http.MethodPost, "/api/v1/auth/verificar-correo", map[string]string{"token": strings.Repeat("A", 43)}, "")

	assert.Equal(t, http.StatusOK, first.status)
	assert.Equal(t, http.StatusBadRequest, second.status)
	assert.Equal(t, http.StatusBadRequest, forged.status)
}

func TestAuthSecurity_AuthRoutesHaveSecurityHeaders(t *testing.T) {
	env := authEnv(t)
	env.CreateUser(t, victim)
	app := env.App()

	for name, r := range map[string]reply{
		"login ok":    login(t, app, victim, authtest.Password),
		"login 401":   login(t, app, victim, "Incorrecta1"),
		"sin sesión":  send(t, app, http.MethodGet, "/api/v1/auth/sesiones", nil, ""),
		"recuperar":   send(t, app, http.MethodPost, "/api/v1/auth/recuperar", map[string]string{"correo": victim}, ""),
		"cuerpo malo": send(t, app, http.MethodPost, "/api/v1/auth/refresh", "{", ""),
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, "nosniff", r.headers.Get("X-Content-Type-Options"))
			assert.Equal(t, "DENY", r.headers.Get("X-Frame-Options"))
			assert.Equal(t, "default-src 'none'; frame-ancestors 'none'", r.headers.Get("Content-Security-Policy"))
			assert.Contains(t, r.headers.Get("Strict-Transport-Security"), "max-age=31536000")
		})
	}
}

func TestAuthSecurity_LoginRateLimitIs429WithRetryAfter(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	for range 5 {
		require.NotEqual(t, http.StatusTooManyRequests, login(t, app, "nadie@wherever.test", "Incorrecta1").status)
	}

	limited := login(t, app, "otro@wherever.test", "Incorrecta1")

	assert.Equal(t, http.StatusTooManyRequests, limited.status)
	assert.NotEmpty(t, limited.headers.Get("Retry-After"))
}

func TestAuthSecurity_RevokedJWTIs401(t *testing.T) {
	env := authEnv(t)
	env.CreateUser(t, victim)
	app := env.App()
	access, _ := tokens(t, login(t, app, victim, authtest.Password))
	require.Equal(t, http.StatusOK, send(t, app, http.MethodGet, "/api/v1/auth/sesiones", nil, access).status)

	require.Equal(t, http.StatusNoContent, send(t, app, http.MethodPost, "/api/v1/auth/logout", nil, access).status)

	assert.Equal(t, http.StatusUnauthorized, send(t, app, http.MethodGet, "/api/v1/auth/sesiones", nil, access).status)
	assert.Equal(t, http.StatusUnauthorized, send(t, app, http.MethodPost, "/api/v1/auth/logout", nil, access).status)
}

func TestAuthSecurity_MalformedAuthorizationIs401(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	for name, header := range map[string]string{
		"Basic":         "Basic dXN1YXJpbzpjbGF2ZQ==",
		"Bearer vacío":  "Bearer ",
		"sin firma":     "Bearer " + testutil.UnsignedToken(t),
		"alg none":      "Bearer " + testutil.AlgNoneToken(t),
		"sub no es id":  "Bearer " + testutil.ValidToken(t, "jti-sub-raro"),
		"dos segmentos": "Bearer a.b",
	} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/sesiones", nil)
			req.Header.Set(fiber.HeaderAuthorization, header)
			resp, err := app.Test(req, -1)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		})
	}
}

// Afirmaciones de tipo comprobadas (errcheck con check-type-assertions).

func str(t *testing.T, v any) string {
	t.Helper()
	s, ok := v.(string)
	require.True(t, ok, "se esperaba string: %v", v)
	return s
}

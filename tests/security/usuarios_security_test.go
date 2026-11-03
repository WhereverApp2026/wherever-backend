package security

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/platform/mailer"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
)

// Pruebas de seguridad de usuarios (spec nv-05 "Security/Usuarios 8").

var sensitive = []string{"password_hash", "telefono_hash", "google_id", "argon2id", authtest.Password}

func registration(email, birth string) map[string]any {
	return map[string]any{
		"nombre": "Ana", "fecha_nacimiento": birth, "correo": email, "password": authtest.Password,
		"acepta_terminos": true, "version_terminos": "2026-10",
	}
}

func signUp(t *testing.T, app *fiber.App, email string) string {
	t.Helper()
	r := send(t, app, http.MethodPost, "/api/v1/usuarios", registration(email, "1995-05-17"), "")
	require.Equal(t, http.StatusCreated, r.status, r.body)
	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(r.body), &body))
	return str(t, body["access_token"])
}

func TestUsuariosSecurity_NoSensitiveFieldInResponses(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	r := send(t, app, http.MethodPost, "/api/v1/usuarios", registration(victim, "1995-05-17"), "")
	require.Equal(t, http.StatusCreated, r.status)
	access := signUp(t, app, "otra@wherever.test")
	send(t, app, http.MethodPut, "/api/v1/usuarios/yo/telefono", map[string]string{"telefono": "+5215512345678"}, access)

	for name, resp := range map[string]reply{
		"registro":   r,
		"yo":         send(t, app, http.MethodGet, "/api/v1/usuarios/yo", nil, access),
		"correo":     send(t, app, http.MethodPut, "/api/v1/usuarios/yo/correo", map[string]string{"correo_nuevo": "nueva@wherever.test", "password": authtest.Password}, access),
		"telefono":   send(t, app, http.MethodPut, "/api/v1/usuarios/yo/telefono", map[string]string{"telefono": "+5215512345679"}, access),
		"gustos":     send(t, app, http.MethodPut, "/api/v1/usuarios/yo/gustos", map[string]any{"categorias": []string{"a", "b", "c"}}, access),
		"privacidad": send(t, app, http.MethodPatch, "/api/v1/usuarios/yo/privacidad", map[string]bool{"datos_uso": true}, access),
	} {
		for _, s := range sensitive {
			assert.NotContains(t, resp.body, s, name)
		}
	}
}

func TestUsuariosSecurity_PhoneAndPasswordNeverInLogs(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	access := signUp(t, app, victim)
	send(t, app, http.MethodPut, "/api/v1/usuarios/yo/telefono", map[string]string{"telefono": "+5215598765432"}, access)
	send(t, app, http.MethodPut, "/api/v1/usuarios/yo/telefono", map[string]string{"telefono": "5598765432"}, access)

	logs := env.Logs.String()
	require.NotEmpty(t, logs)
	assert.NotContains(t, logs, "5598765432")
	assert.NotContains(t, logs, authtest.Password)
}

func TestUsuariosSecurity_RegisterAndEmailCheckAreRateLimited(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	for i := range 5 {
		require.Equal(t, http.StatusCreated, send(t, app, http.MethodPost, "/api/v1/usuarios", registration("r"+string(rune('a'+i))+"@wherever.test", "1995-05-17"), "").status)
	}
	sixth := send(t, app, http.MethodPost, "/api/v1/usuarios", registration("sexto@wherever.test", "1995-05-17"), "")
	assert.Equal(t, http.StatusTooManyRequests, sixth.status)
	assert.NotEmpty(t, sixth.headers.Get("Retry-After"))

	var last reply
	for range 21 {
		last = send(t, app, http.MethodGet, "/api/v1/usuarios/correo-disponible?correo=x%40wherever.test", nil, "")
	}
	assert.Equal(t, http.StatusTooManyRequests, last.status, "20 por minuto por IP")
}

// No hay ruta que reciba el id de otra persona: todo es /yo y sale del `sub` firmado.
func TestUsuariosSecurity_OneUserCannotEditAnother(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	ana := signUp(t, app, victim)
	beto := signUp(t, app, "beto@wherever.test")

	r := send(t, app, http.MethodPatch, "/api/v1/usuarios/yo/privacidad", map[string]any{"ubicacion": true, "id": "otro", "usuario_id": "otro"}, beto)
	require.Equal(t, http.StatusOK, r.status)

	var anaProfile map[string]any
	require.NoError(t, json.Unmarshal([]byte(send(t, app, http.MethodGet, "/api/v1/usuarios/yo", nil, ana).body), &anaProfile))
	assert.Equal(t, victim, anaProfile["correo"])
	privacy, ok := anaProfile["privacidad"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, privacy["ubicacion"], "el cambio de Beto no toca a Ana")
}

func TestUsuariosSecurity_ExportLinkWorksOnce(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	access := signUp(t, app, victim)
	require.Equal(t, http.StatusAccepted, send(t, app, http.MethodPost, "/api/v1/usuarios/yo/exportar", nil, access).status)
	_, err := env.Job.RunOnce(t.Context())
	require.NoError(t, err)
	link, ok := env.Mail.Last(victim, mailer.KindExport)
	require.True(t, ok)

	first := send(t, app, http.MethodPost, "/api/v1/usuarios/exportaciones/descargar", map[string]string{"token": link.Token}, "")
	second := send(t, app, http.MethodPost, "/api/v1/usuarios/exportaciones/descargar", map[string]string{"token": link.Token}, "")
	forged := send(t, app, http.MethodPost, "/api/v1/usuarios/exportaciones/descargar", map[string]string{"token": strings.Repeat("B", 43)}, "")

	assert.Equal(t, http.StatusOK, first.status)
	assert.Equal(t, http.StatusBadRequest, second.status)
	assert.Equal(t, http.StatusBadRequest, forged.status)
	assert.NotContains(t, env.Logs.String(), link.Token)
}

// Al pedir la eliminación se cierran las sesiones: el JWT y el refresh de antes ya no sirven.
func TestUsuariosSecurity_PendingDeletionInvalidatesOldTokens(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	r := send(t, app, http.MethodPost, "/api/v1/usuarios", registration(victim, "1995-05-17"), "")
	access, refresh := tokens(t, reply{status: http.StatusOK, body: r.body})

	require.Equal(t, http.StatusAccepted, send(t, app, http.MethodDelete, "/api/v1/usuarios/yo", map[string]string{"confirmacion": "ELIMINAR"}, access).status)

	assert.Equal(t, http.StatusUnauthorized, send(t, app, http.MethodGet, "/api/v1/usuarios/yo", nil, access).status)
	assert.Equal(t, http.StatusUnauthorized, send(t, app, http.MethodPost, "/api/v1/auth/refresh", map[string]string{"refresh_token": refresh}, "").status)
}

// Con la cuenta en eliminación pendiente se puede entrar, pero solo para ver el perfil, cancelar
// y cerrar sesión; todo lo demás con sesión responde 403 hasta cancelar.
func TestUsuariosSecurity_PendingDeletionOnlyReachesProfileAndCancel(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	access := signUp(t, app, victim)
	require.Equal(t, http.StatusAccepted, send(t, app, http.MethodDelete, "/api/v1/usuarios/yo", map[string]string{"confirmacion": "ELIMINAR"}, access).status)
	env.ClearRateLimits(t)
	pending, _ := tokens(t, login(t, app, victim, authtest.Password))
	other, _ := tokens(t, login(t, app, victim, authtest.Password))

	assert.Equal(t, http.StatusOK, send(t, app, http.MethodGet, "/api/v1/usuarios/yo", nil, pending).status)
	for name, r := range map[string]reply{
		"PATCH /yo":     send(t, app, http.MethodPatch, "/api/v1/usuarios/yo", map[string]string{"nombre": "Otra"}, pending),
		"PUT gustos":    send(t, app, http.MethodPut, "/api/v1/usuarios/yo/gustos", map[string]any{"categorias": []string{"a", "b", "c"}}, pending),
		"POST exportar": send(t, app, http.MethodPost, "/api/v1/usuarios/yo/exportar", nil, pending),
		"DELETE yo":     send(t, app, http.MethodDelete, "/api/v1/usuarios/yo", map[string]string{"confirmacion": "ELIMINAR"}, pending),
		"GET sesiones":  send(t, app, http.MethodGet, "/api/v1/auth/sesiones", nil, pending),
		"PUT password":  send(t, app, http.MethodPut, "/api/v1/auth/password", map[string]string{"password_actual": authtest.Password, "password_nuevo": "NuevaClave9"}, pending),
		"POST reenviar": send(t, app, http.MethodPost, "/api/v1/auth/reenviar-verificacion", nil, pending),
	} {
		assert.Equal(t, http.StatusForbidden, r.status, name)
	}
	assert.Equal(t, http.StatusNoContent, send(t, app, http.MethodPost, "/api/v1/auth/logout", nil, other).status, "cerrar sesión sí se permite")
	assert.Equal(t, http.StatusUnauthorized, send(t, app, http.MethodGet, "/api/v1/usuarios/yo", nil, other).status)

	require.Equal(t, http.StatusOK, send(t, app, http.MethodPost, "/api/v1/usuarios/yo/cancelar-eliminacion", nil, pending).status)
	assert.Equal(t, http.StatusOK, send(t, app, http.MethodGet, "/api/v1/auth/sesiones", nil, pending).status, "al cancelar todo vuelve a abrirse")
}

func TestUsuariosSecurity_AdulthoodByUTCDate(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	today := time.Now().UTC()

	minor := send(t, app, http.MethodPost, "/api/v1/usuarios", registration(victim, today.AddDate(-18, 0, 1).Format(validation.DateLayout)), "")
	adult := send(t, app, http.MethodPost, "/api/v1/usuarios", registration(victim, today.AddDate(-18, 0, 0).Format(validation.DateLayout)), "")

	assert.Equal(t, http.StatusUnprocessableEntity, minor.status)
	assert.Contains(t, minor.body, "fecha_nacimiento")
	assert.Equal(t, http.StatusCreated, adult.status, "cumple 18 hoy (UTC)")
}

func TestUsuariosSecurity_InjectionInQueryAndEmailIsHarmless(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	signUp(t, app, victim)

	operator := send(t, app, http.MethodPost, "/api/v1/usuarios", map[string]any{
		"nombre": "X", "fecha_nacimiento": "1995-05-17", "correo": map[string]any{"$ne": nil}, "password": authtest.Password,
		"acepta_terminos": true, "version_terminos": "1",
	}, "")
	assert.Equal(t, http.StatusBadRequest, operator.status, "un objeto en lugar de texto no llega a MongoDB")

	for _, path := range []string{
		"/api/v1/usuarios/correo-disponible?correo[$ne]=x",
		"/api/v1/usuarios/correo-disponible?correo=%7B%22%24ne%22%3Anull%7D",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		_ = resp.Body.Close()
		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode, path)
	}
	q := send(t, app, http.MethodGet, "/api/v1/ciudades/autocompletar?q=.%2A%7C%28", nil, "")
	assert.Equal(t, http.StatusOK, q.status)
	assert.Equal(t, "[]", strings.TrimSpace(q.body), "el texto se busca literal, no como regex")
}

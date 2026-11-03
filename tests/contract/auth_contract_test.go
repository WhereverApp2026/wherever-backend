package contract

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/mailer"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
)

// Contrato de auth (spec nv-04 §Contratos y docs/endpoints.md): código y forma de cada
// respuesta. Los errores siguen el cuerpo uniforme de nv-01.

const email = "contrato@wherever.test"

var device = map[string]string{"modelo": "Pixel 8", "sistema": "Android 15", "app_version": "1.0.0"}

func login(t *testing.T, password string) map[string]any {
	return map[string]any{"correo": email, "password": password, "dispositivo": device}
}

func TestContract_Auth(t *testing.T) {
	env := authtest.New(t, mongoClient, redisClient, testutil.Config(config.EnvProd))
	env.CreateUser(t, email)
	app := env.App()

	status, body := call(t, app, http.MethodPost, "/api/v1/auth/login", login(t, authtest.Password), "")
	check(t, "auth/login_200", status, body)
	tokens := obj(t, body)
	require.Equal(t, []string{"access_token", "expira_en", "refresh_token", "usuario"}, keys(tokens))
	assert.Equal(t, []string{"correo", "correo_verificado", "id", "nombre", "roles"}, keys(obj(t, tokens["usuario"])),
		"el usuario nunca trae hashes ni google_id")
	refresh := str(t, tokens["refresh_token"])

	status, body = call(t, app, http.MethodPost, "/api/v1/auth/login", login(t, "Incorrecta1"), "")
	check(t, "auth/login_401", status, body)
	status, body = call(t, app, http.MethodPost, "/api/v1/auth/login", map[string]any{"correo": "x"}, "")
	check(t, "auth/login_422", status, body)

	status, body = call(t, app, http.MethodPost, "/api/v1/auth/refresh", map[string]string{"refresh_token": refresh}, "")
	check(t, "auth/refresh_200", status, body)
	access := str(t, obj(t, body)["access_token"])
	status, body = call(t, app, http.MethodPost, "/api/v1/auth/refresh", map[string]string{"refresh_token": "inventado"}, "")
	check(t, "auth/refresh_401", status, body)

	status, body = call(t, app, http.MethodGet, "/api/v1/auth/sesiones", nil, access)
	check(t, "auth/sesiones_200", status, body)
	status, body = call(t, app, http.MethodDelete, "/api/v1/auth/sesiones/000000000000000000000000", nil, access)
	check(t, "auth/sesiones_delete_404", status, body)

	status, body = call(t, app, http.MethodPost, "/api/v1/auth/reenviar-verificacion", nil, access)
	check(t, "auth/reenviar_verificacion_202", status, body)
	status, body = call(t, app, http.MethodPost, "/api/v1/auth/reenviar-verificacion", nil, access)
	check(t, "auth/reenviar_verificacion_429", status, body)
	verify, ok := env.Mail.Last(email, mailer.KindVerifyEmail)
	require.True(t, ok)
	status, body = call(t, app, http.MethodPost, "/api/v1/auth/verificar-correo", map[string]string{"token": verify.Token}, "")
	check(t, "auth/verificar_correo_200", status, body)
	status, body = call(t, app, http.MethodPost, "/api/v1/auth/verificar-correo", map[string]string{"token": verify.Token}, "")
	check(t, "auth/verificar_correo_400", status, body)

	status, body = call(t, app, http.MethodPut, "/api/v1/auth/password",
		map[string]string{"password_actual": authtest.Password, "password_nuevo": "NuevaClave9"}, access)
	check(t, "auth/password_204", status, body)

	status, body = call(t, app, http.MethodPost, "/api/v1/auth/recuperar", map[string]string{"correo": email}, "")
	check(t, "auth/recuperar_202", status, body)
	reset, ok := env.Mail.Last(email, mailer.KindPasswordReset)
	require.True(t, ok)
	status, body = call(t, app, http.MethodPost, "/api/v1/auth/restablecer",
		map[string]any{"token": reset.Token, "password_nuevo": "OtraClave99", "dispositivo": device}, "")
	check(t, "auth/restablecer_200", status, body)
	access = str(t, obj(t, body)["access_token"])

	status, body = call(t, app, http.MethodPost, "/api/v1/auth/logout-todos", nil, access)
	check(t, "auth/logout_todos_204", status, body)
	env.ClearRateLimits(t)
	_, body = call(t, app, http.MethodPost, "/api/v1/auth/login", login(t, "OtraClave99"), "")
	status, body = call(t, app, http.MethodPost, "/api/v1/auth/logout", nil, str(t, obj(t, body)["access_token"]))
	check(t, "auth/logout_204", status, body)
	status, body = call(t, app, http.MethodPost, "/api/v1/auth/logout", nil, "")
	check(t, "auth/logout_401", status, body)
}

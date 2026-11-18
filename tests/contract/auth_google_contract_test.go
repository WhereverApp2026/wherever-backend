package contract

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/googletest"
)

// Contrato de POST /auth/google (spec nv-72): 201 al crear, 200 al volver a entrar y 422
// `registro_incompleto` sin los datos de registro. La respuesta es la misma de login.
func TestContract_AuthGoogle(t *testing.T) {
	env := authtest.New(t, mongoClient, redisClient, testutil.Config(config.EnvProd))
	app := env.App()
	token := env.Google.Token(t, googletest.Identidad{Sub: "g-contrato", Correo: "google.contrato@wherever.test", Nombre: "Contrato"})
	bare := map[string]any{"id_token": token, "dispositivo": device}

	status, body := call(t, app, http.MethodPost, "/api/v1/auth/google", bare, "")
	check(t, "auth/google_422", status, body)
	assert.Equal(t, "registro_incompleto", obj(t, body)["mensaje"])

	status, body = call(t, app, http.MethodPost, "/api/v1/auth/google", map[string]any{
		"id_token": token, "dispositivo": device,
		"fecha_nacimiento": "1994-07-21", "acepta_terminos": true, "version_terminos": "2026-10",
	}, "")
	check(t, "auth/google_201", status, body)
	assert.Equal(t, []string{"correo", "correo_verificado", "id", "nombre", "roles"}, keys(obj(t, obj(t, body)["usuario"])),
		"el usuario nunca trae google_id")

	status, body = call(t, app, http.MethodPost, "/api/v1/auth/google", bare, "")
	check(t, "auth/google_200", status, body)
}

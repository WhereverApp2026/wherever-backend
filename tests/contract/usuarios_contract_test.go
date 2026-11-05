package contract

import (
	"net/http"
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

// Contrato de usuarios y ciudades (spec nv-05 §Contratos y docs/endpoints.md).

const userEmail = "usuaria@wherever.test"

func registration() map[string]any {
	return map[string]any{
		"nombre": "Usuaria", "fecha_nacimiento": "1996-08-20", "correo": userEmail, "password": authtest.Password,
		"acepta_terminos": true, "version_terminos": "2026-10", "promociones_correo": true, "dispositivo": device,
	}
}

func TestContract_Usuarios(t *testing.T) {
	env := authtest.New(t, mongoClient, redisClient, testutil.Config(config.EnvProd))
	app := env.App()

	status, body := call(t, app, http.MethodPost, "/api/v1/usuarios", registration(), "")
	check(t, "usuarios/registro_201", status, body)
	access := str(t, obj(t, body)["access_token"])
	status, body = call(t, app, http.MethodPost, "/api/v1/usuarios", registration(), "")
	check(t, "usuarios/registro_409", status, body)
	status, body = call(t, app, http.MethodPost, "/api/v1/usuarios", map[string]any{"correo": "x"}, "")
	check(t, "usuarios/registro_422", status, body)

	status, body = call(t, app, http.MethodGet, "/api/v1/usuarios/correo-disponible?correo=libre%40wherever.test", nil, "")
	check(t, "usuarios/correo_disponible_200", status, body)

	status, body = call(t, app, http.MethodGet, "/api/v1/usuarios/yo", nil, access)
	check(t, "usuarios/yo_200", status, body)
	profile := obj(t, body)
	for _, forbidden := range []string{"password_hash", "telefono_hash", "google_id"} {
		assert.NotContains(t, profile, forbidden)
	}
	assert.Contains(t, profile, "version")

	status, body = call(t, app, http.MethodPatch, "/api/v1/usuarios/yo", map[string]string{"nombre": "Usu"}, access)
	check(t, "usuarios/yo_patch_400", status, body)
	status, body = callWith(t, app, http.MethodPatch, "/api/v1/usuarios/yo", map[string]string{"nombre": "Usu"}, access, `"1"`)
	check(t, "usuarios/yo_patch_200", status, body)
	status, body = callWith(t, app, http.MethodPatch, "/api/v1/usuarios/yo", map[string]string{"nombre": "Otra"}, access, `"1"`)
	check(t, "usuarios/yo_patch_409", status, body)

	status, _ = call(t, app, http.MethodPatch, "/api/v1/usuarios/yo/privacidad", map[string]any{"datos_sensibles": true}, access)
	require.Equal(t, http.StatusOK, status)
	status, body = call(t, app, http.MethodPut, "/api/v1/usuarios/yo/gustos", map[string]any{
		"categorias": []string{"cafe", "tacos", "museos"}, "presupuesto_min": 100, "presupuesto_max": 300, "restricciones": []string{"vegano"}, "alergias": "nueces",
	}, access)
	check(t, "usuarios/gustos_200", status, body)
	status, body = call(t, app, http.MethodPatch, "/api/v1/usuarios/yo/preferencias", map[string]any{
		"tema": "oscuro", "horario_silencioso": map[string]string{"desde": "22:00", "hasta": "07:00"},
	}, access)
	check(t, "usuarios/preferencias_200", status, body)
	status, body = call(t, app, http.MethodPatch, "/api/v1/usuarios/yo/privacidad", map[string]bool{"ubicacion": true}, access)
	check(t, "usuarios/privacidad_200", status, body)
	status, body = call(t, app, http.MethodPut, "/api/v1/usuarios/yo/telefono", map[string]string{"telefono": "+5215512345678"}, access)
	check(t, "usuarios/telefono_200", status, body)
	status, body = call(t, app, http.MethodPost, "/api/v1/usuarios/yo/modo", map[string]string{"modo": "explorar"}, access)
	check(t, "usuarios/modo_200", status, body)
	status, body = call(t, app, http.MethodPost, "/api/v1/usuarios/yo/modo", map[string]string{"modo": "negocio"}, access)
	check(t, "usuarios/modo_403", status, body)

	status, body = call(t, app, http.MethodPost, "/api/v1/usuarios/yo/exportar", nil, access)
	check(t, "usuarios/exportar_202", status, body)
	status, body = call(t, app, http.MethodPost, "/api/v1/usuarios/yo/exportar", nil, access)
	check(t, "usuarios/exportar_429", status, body)
	_, err := env.Job.RunOnce(t.Context())
	require.NoError(t, err)
	link, ok := env.Mail.Last(userEmail, mailer.KindExport)
	require.True(t, ok)
	status, body = call(t, app, http.MethodPost, "/api/v1/usuarios/exportaciones/descargar", map[string]string{"token": link.Token}, "")
	check(t, "usuarios/exportacion_descargar_200", status, body)
	status, body = call(t, app, http.MethodPost, "/api/v1/usuarios/exportaciones/descargar", map[string]string{"token": link.Token}, "")
	check(t, "usuarios/exportacion_descargar_400", status, body)

	status, body = call(t, app, http.MethodDelete, "/api/v1/usuarios/yo/historial", nil, access)
	check(t, "usuarios/historial_204", status, body)
	status, body = call(t, app, http.MethodDelete, "/api/v1/usuarios/yo", map[string]string{"confirmacion": "ELIMINAR"}, access)
	check(t, "usuarios/eliminar_202", status, body)
	env.ClearRateLimits(t)
	_, body = call(t, app, http.MethodPost, "/api/v1/auth/login", map[string]any{"correo": userEmail, "password": authtest.Password, "dispositivo": device}, "")
	status, body = call(t, app, http.MethodPost, "/api/v1/usuarios/yo/cancelar-eliminacion", nil, str(t, obj(t, body)["access_token"]))
	check(t, "usuarios/cancelar_eliminacion_200", status, body)
}

func TestContract_Ciudades(t *testing.T) {
	env := authtest.New(t, mongoClient, redisClient, testutil.Config(config.EnvProd))
	_, err := env.Cities.Import(t.Context(), strings.NewReader("nombre,estado,latitud,longitud,con_cobertura\nTula de Allende,Hidalgo,20.05,-99.34,true\n"), "csv")
	require.NoError(t, err)
	app := env.App()

	status, body := call(t, app, http.MethodGet, "/api/v1/ciudades/autocompletar?q=tula", nil, "")
	check(t, "ciudades/autocompletar_200", status, body)
	status, body = call(t, app, http.MethodGet, "/api/v1/ciudades/autocompletar?q=t", nil, "")
	check(t, "ciudades/autocompletar_422", status, body)
}

// callWith es call con cabecera If-Match.
func callWith(t *testing.T, app *fiber.App, method, path string, body any, token, ifMatch string) (int, any) {
	t.Helper()
	return callHeaders(t, app, method, path, body, token, map[string]string{fiber.HeaderIfMatch: ifMatch})
}

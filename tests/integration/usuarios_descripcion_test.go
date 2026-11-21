package integration

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/moderacion"
)

// version lee el ETag de GET /usuarios/yo para el If-Match.
func versionYo(t *testing.T, app *fiber.App, token string) string {
	t.Helper()
	resp := sendJSON(t, app, http.MethodGet, "/api/v1/usuarios/yo", nil, token)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	return resp.Header.Get(fiber.HeaderETag)
}

func patchYo(t *testing.T, app *fiber.App, token string, body map[string]any) (int, map[string]any) {
	t.Helper()
	resp := sendWithHeaders(t, app, http.MethodPatch, "/api/v1/usuarios/yo", body, token, map[string]string{fiber.HeaderIfMatch: versionYo(t, app, token)})
	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, nil
	}
	return resp.StatusCode, decode(t, resp)
}

// La descripción del perfil (nv-77): se guarda limpia, la ven quienes pueden ver el perfil,
// se borra con "" y un texto rechazado por las reglas de moderación es 422.
func TestUsuariosDescripcion_GuardarVerYBorrar(t *testing.T) {
	env, app := amistadesEnv(t)
	ana := seedAmigo(t, env, "desc_ana")
	beto := seedAmigo(t, env, "desc_beto")

	status, body := patchYo(t, app, ana.token, map[string]any{"descripcion": "  Amo los tacos\nde canasta  y el mezcal  "})
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, "Amo los tacos de canasta y el mezcal", body["descripcion"], "una línea y sin espacios de sobra")

	code, raw := perfil(t, app, beto, ana.id())
	require.Equal(t, http.StatusOK, code, raw)
	assert.Contains(t, raw, `"descripcion":"Amo los tacos de canasta y el mezcal"`, "la ve otra persona que puede ver el perfil")

	status, _ = patchYo(t, app, ana.token, map[string]any{"descripcion": strings.Repeat("a", 161)})
	assert.Equal(t, http.StatusUnprocessableEntity, status, "más de 160 caracteres")

	status, body = patchYo(t, app, ana.token, map[string]any{"descripcion": ""})
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, "", body["descripcion"])
	stored, err := env.Users.FindByID(context.Background(), ana.user.ID)
	require.NoError(t, err)
	assert.Empty(t, stored.Description)
}

func TestUsuariosDescripcion_ModeracionRechaza(t *testing.T) {
	env, app := amistadesEnv(t)
	ana := seedAmigo(t, env, "desc_mod")
	_, err := env.Moderacion.CrearReglas(context.Background(), bson.NewObjectID().Hex(), moderacion.ReglasRequest{Cuarentena: 40, Rechazo: 80, Palabras: []string{"vendo mota"}})
	require.NoError(t, err)

	status, _ := patchYo(t, app, ana.token, map[string]any{"descripcion": "Hola, VENDO MOTA barata"})
	assert.Equal(t, http.StatusUnprocessableEntity, status, "palabra bloqueada, sin importar mayúsculas")
	stored, err := env.Users.FindByID(context.Background(), ana.user.ID)
	require.NoError(t, err)
	assert.Empty(t, stored.Description, "no se guardó nada")

	status, body := patchYo(t, app, ana.token, map[string]any{"descripcion": "Fan del pulque y los museos"})
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, "Fan del pulque y los museos", body["descripcion"])
}

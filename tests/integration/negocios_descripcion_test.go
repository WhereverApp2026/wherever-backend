package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/ia"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios/panel"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/reservastest"
)

func rutaDescripcion(b negocios.Business) string {
	return "/api/v1/negocios/" + b.ID.Hex() + "/ia/descripcion"
}

const descripcionIA = "Restaurante familiar con tacos al pastor, salsas de la casa y cerveza fría para compartir con amigos."

func palabras(p ...string) map[string]any { return map[string]any{"palabras": p} }

// 3 palabras = 200 con la IA; las mismas otra vez salen de la caché de la guardia (sin llamar).
func TestDescripcion_IAYCachePorHuella(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	b, token := dueno(t, env, "duena-desc@wherever.test", reservastest.Config{})
	_, err := env.Mongo.DB().Collection(negocios.Collection).UpdateOne(context.Background(), bson.D{{Key: "_id", Value: b.ID}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "telefono", Value: "7731234567"}}}})
	require.NoError(t, err)
	env.IA.Responder(`{"descripcion":"` + descripcionIA + `"}`)

	for range 2 {
		resp := sendJSON(t, app, http.MethodPost, rutaDescripcion(b), palabras("tacos", "salsas", "cerveza"), token)
		requireStatus(t, resp, http.StatusOK)
		out := decode(t, resp)
		assertSnakeCase(t, out)
		assert.Equal(t, descripcionIA, out["descripcion"])
		assert.Equal(t, panel.FuenteIA, out["fuente"])
	}
	peticiones := env.IA.Peticiones()
	require.Len(t, peticiones, 1, "la misma huella no vuelve a llamar")
	p := peticiones[0]
	assert.Equal(t, "Palabras: tacos, salsas, cerveza", p.TextoUsuario)
	assert.Contains(t, p.Contexto, "restaurante")
	for _, prohibido := range []string{b.Name, b.ID.Hex(), "7731234567", "duena-desc", b.OwnerID.Hex()} {
		assert.NotContains(t, p.Contexto+p.TextoUsuario, prohibido)
	}

	var guardado negocios.Business
	require.NoError(t, env.Mongo.DB().Collection(negocios.Collection).FindOne(context.Background(), bson.D{{Key: "_id", Value: b.ID}}).Decode(&guardado))
	assert.Empty(t, guardado.Description, "no se guarda")
}

func TestDescripcion_PalabrasInvalidas(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	b, token := dueno(t, env, "duena-desc-val@wherever.test", reservastest.Config{})

	requireStatus(t, sendJSON(t, app, http.MethodPost, rutaDescripcion(b), palabras("tacos", "salsa"), token), http.StatusUnprocessableEntity)
	requireStatus(t, sendJSON(t, app, http.MethodPost, rutaDescripcion(b), palabras("tacos", "salsa", "cerveza", "música"), token), http.StatusUnprocessableEntity)
	requireStatus(t, sendJSON(t, app, http.MethodPost, rutaDescripcion(b), palabras("tacos", "https://x.mx", "cerveza"), token), http.StatusUnprocessableEntity)
	req := httptest.NewRequest(http.MethodPost, rutaDescripcion(b), strings.NewReader(`{"palabras":`))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer "+token)
	resp, err := app.Test(req, 20000)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "JSON mal formado")
	assert.Empty(t, env.IA.Peticiones())
}

// Gemini caído = plantilla con la categoría y las 3 palabras.
func TestDescripcion_GeminiCaidoDaPlantilla(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	b, token := dueno(t, env, "duena-desc-caido@wherever.test", reservastest.Config{})
	env.IA.Fallar(ia.ErrProveedor)

	out := decode(t, sendJSON(t, app, http.MethodPost, rutaDescripcion(b), palabras("mole", "terraza", "pet-friendly"), token))
	assert.Equal(t, panel.FuentePlantilla, out["fuente"])
	assert.True(t, strings.HasPrefix(str(t, out["descripcion"]), "Restaurante con mole, terraza y pet-friendly."))
}

// El límite del usuario (IA_LIMITE_HORA) = 429 con Retry-After; un borrador del alta también sirve.
func TestDescripcion_LimiteDelUsuarioYBorrador(t *testing.T) {
	cfg := testutil.Config(config.EnvDev)
	cfg.IA.LimiteHora = 1
	env := authEnvWith(t, cfg)
	env.ClearRateLimits(t)
	app := env.App()
	b, token := dueno(t, env, "duena-desc-lim@wherever.test", reservastest.Config{})
	_, err := env.Mongo.DB().Collection(negocios.Collection).UpdateOne(context.Background(), bson.D{{Key: "_id", Value: b.ID}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "estado", Value: negocios.StatusDraft}}}})
	require.NoError(t, err)
	env.IA.Responder(`{"descripcion":"` + descripcionIA + `"}`)

	out := decode(t, sendJSON(t, app, http.MethodPost, rutaDescripcion(b), palabras("tacos", "salsas", "cerveza"), token))
	assert.Equal(t, panel.FuenteIA, out["fuente"], "el negocio en borrador también es del dueño")

	resp := sendJSON(t, app, http.MethodPost, rutaDescripcion(b), palabras("mole", "terraza", "música"), token)
	requireStatus(t, resp, http.StatusTooManyRequests)
	assert.NotEmpty(t, resp.Header.Get("Retry-After"))
	assert.Len(t, env.IA.Peticiones(), 1)
}

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/embeddings"
	"github.com/WhereverApp2026/wherever-backend/internal/ia"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/semantica"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/iatest"
)

// Búsqueda semántica (nv-51) con MongoDB y Redis reales; Gemini es un servidor local que da
// a cada texto un vector fijo. El negocio esperado guarda el vector exacto de la consulta.
const fraseCafe = "un café tranquilo para leer"

func semanticaEnv(t *testing.T, conVectores bool) (*authtest.Env, *fiber.App, []negocios.Business) {
	t.Helper()
	env := negociosEnv(t)
	docs := insertPlaces(t, env, []placeSpec{
		{slug: "sem-cafe", category: "cafeteria"}, {slug: "sem-bar", category: "bar", rating: 4.9},
		{slug: "sem-rest"}, {slug: "sem-borrador", category: "cafeteria", state: negocios.StatusDraft},
	})
	if conVectores {
		consulta := iatest.VectorDe(ia.TextoConsulta(fraseCafe))
		repo := embeddings.NewRepository(env.Mongo.DB())
		for _, d := range docs {
			v := iatest.VectorDe("otro " + d.Slug)
			if d.Slug == "sem-cafe" || d.Slug == "sem-borrador" {
				v = consulta
			}
			require.NoError(t, repo.Guardar(context.Background(), embeddings.Vector{NegocioID: d.ID, Vector: v, Version: "v", Modelo: "prueba", ActualizadoEn: time.Now()}))
		}
	}
	return env, env.App(), docs
}

func buscarSemantica(t *testing.T, app *fiber.App, q url.Values, estado int) semantica.Respuesta {
	t.Helper()
	resp := get(t, app, "/api/v1/lugares/semantica?"+q.Encode())
	exigirEstado(t, resp, estado)
	var out semantica.Respuesta
	if estado == http.StatusOK {
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
		assert.Equal(t, "no-store", resp.Header.Get(fiber.HeaderCacheControl))
	}
	return out
}

func TestSemantica_FraseDevuelveElEsperadoYSoloAprobados(t *testing.T) {
	env, app, _ := semanticaEnv(t, true)
	out := buscarSemantica(t, app, url.Values{"q": {fraseCafe}}, http.StatusOK)
	assert.Equal(t, semantica.FuenteSemantica, out.Fuente)
	assert.Empty(t, out.Aviso)
	require.Len(t, out.Items, 3, "el borrador nunca sale aunque tenga el mejor vector")
	assert.Equal(t, "sem-cafe", out.Items[0].Slug)
	require.NotNil(t, out.Items[0].Afinidad)
	assert.InDelta(t, 1.0, *out.Items[0].Afinidad, 1e-9)
	assert.Nil(t, out.Items[0].DistanceM, "sin ubicación no hay distancia")
	assert.Equal(t, []string{ia.TextoConsulta(fraseCafe)}, env.Vectores.Textos(), "solo la frase anonimizada viaja")
}

func TestSemantica_Limite(t *testing.T) {
	_, app, _ := semanticaEnv(t, true)
	out := buscarSemantica(t, app, url.Values{"q": {fraseCafe}, "limite": {"2"}}, http.StatusOK)
	assert.Len(t, out.Items, 2)
}

func TestSemantica_CacheDe24HorasPorHuella(t *testing.T) {
	env, app, _ := semanticaEnv(t, true)
	buscarSemantica(t, app, url.Values{"q": {fraseCafe}}, http.StatusOK)
	buscarSemantica(t, app, url.Values{"q": {"  Un CAFE tranquilo para leer "}}, http.StatusOK)
	assert.Equal(t, 1, env.Vectores.Llamadas(), "la segunda sale de la caché")
	llave := "cache:ia:" + semantica.Huella("gemini/gemini-embedding-2", fraseCafe)
	ttl, err := env.Redis.TTL(context.Background(), llave).Result()
	require.NoError(t, err)
	assert.Greater(t, ttl, 23*time.Hour)
	assert.LessOrEqual(t, ttl, semantica.VidaCache)
	keys, err := env.Redis.Keys(context.Background(), "cache:ia:*").Result()
	require.NoError(t, err)
	for _, k := range keys {
		assert.NotContains(t, k, "tranquilo", "la llave no lleva la frase")
	}
}

func TestSemantica_ServicioCaidoRespondeElMotorDeReglas(t *testing.T) {
	for _, estado := range []int{http.StatusInternalServerError, http.StatusTooManyRequests} {
		env, app, _ := semanticaEnv(t, true)
		env.Vectores.Responder(estado)
		out := buscarSemantica(t, app, url.Values{"q": {"cafeteria tranquila"}}, http.StatusOK)
		assert.Equal(t, semantica.FuenteReglas, out.Fuente, estado)
		assert.NotEmpty(t, out.Aviso)
		require.Len(t, out.Items, 3)
		assert.Equal(t, "sem-cafe", out.Items[0].Slug, "la palabra coincide con el giro")
		assert.Nil(t, out.Items[0].Afinidad)
	}
}

func TestSemantica_SinCupoRespondeElMotorDeReglas(t *testing.T) {
	env, app, _ := semanticaEnv(t, true)
	zona, err := time.LoadLocation("America/Los_Angeles")
	require.NoError(t, err)
	llave := "rate:ia_emb_dia:" + time.Now().In(zona).Format("2006-01-02")
	require.NoError(t, env.Redis.Set(context.Background(), llave, 800, time.Hour).Err())
	out := buscarSemantica(t, app, url.Values{"q": {fraseCafe}}, http.StatusOK)
	assert.Equal(t, semantica.FuenteReglas, out.Fuente)
	assert.Zero(t, env.Vectores.Llamadas(), "sin cupo no se llama al proveedor")
}

func TestSemantica_SinVectoresRespondeElMotorDeReglas(t *testing.T) {
	_, app, _ := semanticaEnv(t, false)
	out := buscarSemantica(t, app, url.Values{"q": {fraseCafe}}, http.StatusOK)
	assert.Equal(t, semantica.FuenteReglas, out.Fuente)
	assert.Len(t, out.Items, 3)
}

func TestSemantica_Validacion422(t *testing.T) {
	_, app, _ := semanticaEnv(t, false)
	for _, q := range []url.Values{{}, {"q": {"   "}}, {"q": {"a"}}, {"q": {strings.Repeat("x", 201)}}, {"q": {fraseCafe}, "limite": {"31"}}} {
		buscarSemantica(t, app, q, http.StatusUnprocessableEntity)
	}
}

func TestSemantica_LimiteDePeticiones429(t *testing.T) {
	_, app, _ := semanticaEnv(t, true)
	for range 30 {
		buscarSemantica(t, app, url.Values{"q": {fraseCafe}}, http.StatusOK)
	}
	resp := get(t, app, "/api/v1/lugares/semantica?q=otra+cosa")
	exigirEstado(t, resp, http.StatusTooManyRequests)
}

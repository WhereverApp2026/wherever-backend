package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/busquedas"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/text"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
)

// searchPlaces son cuatro aprobados y uno en revisión que nunca debe salir.
func searchPlaces(t *testing.T, env *authtest.Env) {
	t.Helper()
	docs := []any{}
	for i, p := range []struct {
		name, category, state string
		tags                  []string
		rating                float64
	}{
		{"Café Tacuba", "cafeteria", negocios.StatusApproved, []string{"desayunos"}, 4.8},
		{"Taquería El Pastor", "restaurante", negocios.StatusApproved, []string{"tacos", "pastor"}, 4.2},
		{"Pizzería Nápoles", "restaurante", negocios.StatusApproved, []string{"pizza"}, 4.5},
		{"La Cantina Tacuba", "bar", negocios.StatusApproved, []string{"mezcal"}, 3.9},
		{"Café Oculto", "cafeteria", negocios.StatusReview, nil, 5},
	} {
		b := placeDoc(placeSpec{slug: text.Normalize(p.name), state: p.state, category: p.category, rating: p.rating, dLat: float64(i) * 0.001})
		b.Name, b.NormalizedName, b.Tags = p.name, text.Normalize(p.name), p.tags
		docs = append(docs, b)
	}
	_, err := env.Mongo.DB().Collection(negocios.Collection).InsertMany(context.Background(), docs)
	require.NoError(t, err)
}

func busquedaEnv(t *testing.T) (*authtest.Env, *fiber.App) {
	t.Helper()
	env := negociosEnv(t)
	searchPlaces(t, env)
	return env, env.App()
}

func searchGet(t *testing.T, app *fiber.App, path, q, token string) (int, map[string]any) {
	t.Helper()
	resp := sendJSON(t, app, http.MethodGet, path+"?"+url.Values{"q": {q}}.Encode(), nil, token)
	raw := readAll(t, resp)
	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, nil
	}
	assert.Equal(t, "no-store", resp.Header.Get(fiber.HeaderCacheControl))
	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &body))
	assertSnakeCase(t, body)
	return resp.StatusCode, body
}

func itemNames(t *testing.T, body map[string]any) []string {
	t.Helper()
	out := []string{}
	for _, item := range list(t, body["items"]) {
		out = append(out, str(t, obj(t, item)["nombre"]))
	}
	return out
}

const (
	autocompletarPath = "/api/v1/lugares/autocompletar"
	buscarPath        = "/api/v1/lugares/buscar"
	recientesPath     = "/api/v1/busquedas/recientes"
)

func TestBusqueda_AutocompletarConDosLetrasAcentosYMayusculas(t *testing.T) {
	_, app := busquedaEnv(t)
	// Prefijo de 2 letras: primero los que empiezan así, luego por palabra; nunca el oculto.
	status, body := searchGet(t, app, autocompletarPath, "CA", "")
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, []string{"Café Tacuba", "La Cantina Tacuba"}, itemNames(t, body))
	// Sin acento y en mayúsculas encuentra «Nápoles».
	_, body = searchGet(t, app, autocompletarPath, "NAPO", "")
	assert.Equal(t, []string{"Pizzería Nápoles"}, itemNames(t, body))
}

func TestBusqueda_BuscarConAcentosYMayusculas(t *testing.T) {
	_, app := busquedaEnv(t)
	_, body := searchGet(t, app, buscarPath, "PIZZERIA", "")
	assert.Equal(t, []string{"Pizzería Nápoles"}, itemNames(t, body))
	assert.Equal(t, false, body["aproximada"])
	// Raíz en español: «taco» encuentra la etiqueta «tacos».
	_, body = searchGet(t, app, buscarPath, "taco", "")
	assert.Contains(t, itemNames(t, body), "Taquería El Pastor")
}

func TestBusqueda_ErrorDeUnaLetra(t *testing.T) {
	_, app := busquedaEnv(t)
	_, body := searchGet(t, app, buscarPath, "pizeria", "")
	assert.Equal(t, []string{"Pizzería Nápoles"}, itemNames(t, body))
	assert.Equal(t, true, body["aproximada"])
	_, body = searchGet(t, app, buscarPath, "Tacbua", "")
	assert.Equal(t, []string{"Café Tacuba", "La Cantina Tacuba"}, itemNames(t, body))
}

func TestBusqueda_SinResultados(t *testing.T) {
	_, app := busquedaEnv(t)
	status, body := searchGet(t, app, buscarPath, "sushi", "")
	require.Equal(t, http.StatusOK, status)
	assert.Empty(t, list(t, body["items"]))
	_, body = searchGet(t, app, autocompletarPath, "zz", "")
	assert.Empty(t, list(t, body["items"]))
}

func TestBusqueda_TextoInyectadoYValidacion(t *testing.T) {
	_, app := busquedaEnv(t)
	for _, q := range []string{`.*`, `"-café"`, `{"$ne": null}`, `ca|ta`, `\`} {
		status, _ := searchGet(t, app, autocompletarPath, q, "")
		assert.Contains(t, []int{http.StatusOK, http.StatusUnprocessableEntity}, status, q)
		status, _ = searchGet(t, app, buscarPath, q, "")
		assert.Contains(t, []int{http.StatusOK, http.StatusUnprocessableEntity}, status, q)
	}
	// `.*` no es comodín: no lista todo.
	_, body := searchGet(t, app, autocompletarPath, ".*a", "")
	assert.Empty(t, list(t, body["items"]))
	// Menos de 2 letras = 422.
	for _, path := range []string{autocompletarPath, buscarPath} {
		status, _ := searchGet(t, app, path, "a", "")
		assert.Equal(t, http.StatusUnprocessableEntity, status, path)
	}
}

func TestBusqueda_RecientesConSesion(t *testing.T) {
	env, app := busquedaEnv(t)
	u := env.CreateUser(t, "busqueda@wherever.test")
	token := testutil.ValidTokenFor(t, u.ID.Hex(), "jti-busqueda", middleware.RoleConsumer)
	clock := time.Date(2026, 11, 12, 18, 0, 0, 0, time.UTC)
	env.Busquedas.WithClock(func() time.Time { clock = clock.Add(time.Second); return clock })

	for i := 0; i < busquedas.MaxRecientes+2; i++ {
		searchGet(t, app, buscarPath, "taco "+string(rune('a'+i))+"x", token)
	}
	searchGet(t, app, buscarPath, "Pizzería", token)
	searchGet(t, app, buscarPath, "PIZZERIA", token) // la misma: sube, no duplica
	searchGet(t, app, buscarPath, "anonimo", "")     // sin sesión no se guarda

	_, body := searchGet(t, app, recientesPath, "", token)
	items := list(t, body["items"])
	require.Len(t, items, busquedas.MaxRecientes)
	assert.Equal(t, "PIZZERIA", str(t, obj(t, items[0])["texto"]))

	resp := sendJSON(t, app, http.MethodDelete, recientesPath, nil, token)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	_, body = searchGet(t, app, recientesPath, "", token)
	assert.Empty(t, list(t, body["items"]))
}

func TestBusqueda_RecientesExigenSesion(t *testing.T) {
	_, app := busquedaEnv(t)
	assert.Equal(t, http.StatusUnauthorized, sendJSON(t, app, http.MethodGet, recientesPath, nil, "").StatusCode)
	assert.Equal(t, http.StatusUnauthorized, sendJSON(t, app, http.MethodDelete, recientesPath, nil, "").StatusCode)
}

func TestBusqueda_ArcoExportaYBorraRecientes(t *testing.T) {
	env := negociosEnv(t)
	repo := busquedas.NewRepository(env.Mongo.DB())
	privacy := busquedas.NewPrivacy(repo)
	user := bson.NewObjectID()
	ctx := context.Background()
	require.NoError(t, repo.Guardar(ctx, user, "Tacos al pastor", "tacos al pastor", time.Now().UTC()))

	exported, err := privacy.Export(ctx, user)
	require.NoError(t, err)
	raw, err := json.Marshal(exported)
	require.NoError(t, err)
	assert.Contains(t, string(raw), "Tacos al pastor")

	require.NoError(t, privacy.PurgeAccount(ctx, user))
	left, err := repo.Recientes(ctx, user)
	require.NoError(t, err)
	assert.Empty(t, left)
}

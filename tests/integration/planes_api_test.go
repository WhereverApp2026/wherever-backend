package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
)

type planReply struct {
	status int
	body   string
	etag   string
}

// planCall manda la petición con Bearer e If-Match opcionales.
func planCall(t *testing.T, app *fiber.App, method, path string, body any, token, ifMatch string) planReply {
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
	if ifMatch != "" {
		req.Header.Set(fiber.HeaderIfMatch, ifMatch)
	}
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return planReply{status: resp.StatusCode, body: string(raw), etag: resp.Header.Get(fiber.HeaderETag)}
}

func (r planReply) plan(t *testing.T) planes.Plan {
	t.Helper()
	var p planes.Plan
	require.NoError(t, json.Unmarshal([]byte(r.body), &p), r.body)
	return p
}

type planesAPI struct {
	app   *fiber.App
	token string
	a, b  bson.ObjectID
}

// newPlanesAPI inserta dos negocios aprobados (abiertos de 09:00 a 18:00, a ~1 km) y un
// usuario consumidor activo.
func newPlanesAPI(t *testing.T) planesAPI {
	t.Helper()
	env := negociosEnv(t)
	a := placeDoc(placeSpec{slug: "api-a"})
	b := placeDoc(placeSpec{slug: "api-b", dLat: 0.009})
	_, err := env.Mongo.DB().Collection(negocios.Collection).InsertMany(context.Background(), []any{a, b})
	require.NoError(t, err)
	u := env.CreateUser(t, "planes-api@example.test")
	return planesAPI{app: env.App(), token: testutil.ValidTokenFor(t, u.ID.Hex(), "jti-planes-api", middleware.RoleConsumer), a: a.ID, b: b.ID}
}

// body arma un plan con una parada por negocio, cada dos horas desde las 12:00 de México.
func (api planesAPI) body(day string, businesses ...bson.ObjectID) map[string]any {
	stops := []map[string]any{}
	for i, id := range businesses {
		stops = append(stops, map[string]any{
			"negocio_id": id.Hex(), "llega": day + "T" + []string{"12", "14", "16"}[i] + ":00:00-06:00", "minutos": 60, "precio": 100,
		})
	}
	return map[string]any{"nombre": "Sábado", "fecha": day, "personas": 2, "presupuesto": 800, "movilidad": "a_pie", "paradas": stops}
}

func (api planesAPI) create(t *testing.T, day string, businesses ...bson.ObjectID) planes.Plan {
	t.Helper()
	got := planCall(t, api.app, http.MethodPost, "/api/v1/planes", api.body(day, businesses...), api.token, "")
	require.Equal(t, http.StatusCreated, got.status, got.body)
	assert.Equal(t, `"1"`, got.etag)
	return got.plan(t)
}

func planPath(p planes.Plan, rest string) string { return "/api/v1/planes/" + p.ID.Hex() + rest }

func TestPlanesAPI_CrearLeerYEditar(t *testing.T) {
	api := newPlanesAPI(t)
	p := api.create(t, "2026-11-14", api.a, api.b)
	assert.Equal(t, 400, p.CostoTotal)
	require.Len(t, p.Traslados, 1)

	got := planCall(t, api.app, http.MethodGet, planPath(p, ""), nil, api.token, "")
	require.Equal(t, http.StatusOK, got.status, got.body)
	assert.NotContains(t, got.body, "dueno_id")

	patch := map[string]any{"nombre": "Domingo", "fecha": "2026-11-15", "personas": 3, "presupuesto": 900, "movilidad": "bicicleta"}
	got = planCall(t, api.app, http.MethodPatch, planPath(p, ""), patch, api.token, `"1"`)
	require.Equal(t, http.StatusOK, got.status, got.body)
	edited := got.plan(t)
	assert.Equal(t, "Domingo", edited.Nombre)
	assert.Equal(t, 3, edited.Personas)
	assert.Equal(t, 900, edited.Presupuesto)
	assert.Equal(t, planes.MobilityBike, edited.Movilidad)
	assert.Equal(t, 600, edited.CostoTotal)
	assert.Equal(t, p.Paradas[0].Llega.AddDate(0, 0, 1), edited.Paradas[0].Llega, "la fecha nueva mueve las paradas")
	assert.Equal(t, 2, edited.Version)
	assert.Equal(t, `"2"`, got.etag)
}

func TestPlanesAPI_ParadasAgregarEditarOrdenarQuitar(t *testing.T) {
	api := newPlanesAPI(t)
	p := api.create(t, "2026-11-14", api.a)

	stop := map[string]any{"negocio_id": api.b.Hex(), "llega": "2026-11-14T15:00:00-06:00", "minutos": 30, "precio": 50}
	got := planCall(t, api.app, http.MethodPost, planPath(p, "/paradas"), stop, api.token, `"1"`)
	require.Equal(t, http.StatusCreated, got.status, got.body)
	p = got.plan(t)
	require.Len(t, p.Paradas, 2)
	second := p.Paradas[1]

	got = planCall(t, api.app, http.MethodPatch, planPath(p, "/paradas/"+second.ID.Hex()), map[string]any{"minutos": 90, "nota": "terraza"}, api.token, `"2"`)
	require.Equal(t, http.StatusOK, got.status, got.body)
	p = got.plan(t)
	assert.Equal(t, 90, p.Paradas[1].Minutos)

	order := map[string]any{"orden": []string{second.ID.Hex(), p.Paradas[0].ID.Hex()}}
	got = planCall(t, api.app, http.MethodPut, planPath(p, "/paradas/orden"), order, api.token, `"3"`)
	require.Equal(t, http.StatusOK, got.status, got.body)
	p = got.plan(t)
	assert.Equal(t, second.ID, p.Paradas[0].ID, "la parada movida toma la primera hora")

	got = planCall(t, api.app, http.MethodDelete, planPath(p, "/paradas/"+second.ID.Hex()), nil, api.token, `"4"`)
	require.Equal(t, http.StatusOK, got.status, got.body)
	p = got.plan(t)
	require.Len(t, p.Paradas, 1)
	assert.Empty(t, p.Traslados)

	got = planCall(t, api.app, http.MethodDelete, planPath(p, "/paradas/"+p.Paradas[0].ID.Hex()), nil, api.token, `"5"`)
	assert.Equal(t, http.StatusUnprocessableEntity, got.status, "la última parada no se quita")
	got = planCall(t, api.app, http.MethodDelete, planPath(p, "/paradas/"+bson.NewObjectID().Hex()), nil, api.token, `"5"`)
	assert.Equal(t, http.StatusNotFound, got.status)
}

func TestPlanesAPI_ClonarConFechaNueva(t *testing.T) {
	api := newPlanesAPI(t)
	p := api.create(t, "2026-11-14", api.a, api.b)

	got := planCall(t, api.app, http.MethodPost, planPath(p, "/clonar"), map[string]any{"fecha": "2026-11-21", "nombre": "Otra vez"}, api.token, "")
	require.Equal(t, http.StatusCreated, got.status, got.body)
	clone := got.plan(t)
	assert.NotEqual(t, p.ID, clone.ID)
	assert.Equal(t, "Otra vez", clone.Nombre)
	assert.Equal(t, 1, clone.Version)
	require.Len(t, clone.Paradas, 2)
	assert.NotEqual(t, p.Paradas[0].ID, clone.Paradas[0].ID, "paradas con ids nuevos")
	assert.Equal(t, p.Paradas[0].Llega.AddDate(0, 0, 7), clone.Paradas[0].Llega)
	assert.Equal(t, p.CostoTotal, clone.CostoTotal)

	got = planCall(t, api.app, http.MethodPost, "/api/v1/planes/"+bson.NewObjectID().Hex()+"/clonar", map[string]any{"fecha": "2026-11-21"}, api.token, "")
	assert.Equal(t, http.StatusNotFound, got.status)
}

func TestPlanesAPI_ConcurrenciaIfMatch(t *testing.T) {
	api := newPlanesAPI(t)
	p := api.create(t, "2026-11-14", api.a)
	rename := map[string]any{"nombre": "Nuevo"}

	got := planCall(t, api.app, http.MethodPatch, planPath(p, ""), rename, api.token, "")
	assert.Equal(t, http.StatusBadRequest, got.status, "sin If-Match")
	got = planCall(t, api.app, http.MethodPatch, planPath(p, ""), rename, api.token, `"1"`)
	require.Equal(t, http.StatusOK, got.status, got.body)
	got = planCall(t, api.app, http.MethodPatch, planPath(p, ""), rename, api.token, `"1"`)
	assert.Equal(t, http.StatusConflict, got.status, "versión vieja")
	got = planCall(t, api.app, http.MethodDelete, planPath(p, ""), nil, api.token, `"1"`)
	assert.Equal(t, http.StatusConflict, got.status)

	got = planCall(t, api.app, http.MethodDelete, planPath(p, ""), nil, api.token, `"2"`)
	assert.Equal(t, http.StatusNoContent, got.status, got.body)
	got = planCall(t, api.app, http.MethodGet, planPath(p, ""), nil, api.token, "")
	assert.Equal(t, http.StatusNotFound, got.status, "borrado lógico")
}

func TestPlanesAPI_NegocioCerradoEs422(t *testing.T) {
	api := newPlanesAPI(t)
	body := api.body("2026-11-14", api.a)
	body["paradas"] = []map[string]any{{"negocio_id": api.a.Hex(), "llega": "2026-11-14T20:00:00-06:00", "minutos": 60}}
	got := planCall(t, api.app, http.MethodPost, "/api/v1/planes", body, api.token, "")
	assert.Equal(t, http.StatusUnprocessableEntity, got.status)
	assert.Contains(t, got.body, planes.ReasonClosed)
}

func TestPlanesAPI_PestanasConCursor(t *testing.T) {
	api := newPlanesAPI(t)
	future := []planes.Plan{api.create(t, "2099-01-03", api.a), api.create(t, "2099-01-01", api.a), api.create(t, "2099-01-02", api.a)}
	past := api.create(t, "2020-01-01", api.a)

	page := func(tab, cursor string) (int, pagination.PageResult[planes.Plan]) {
		q := url.Values{"pestana": {tab}, "limite": {"2"}}
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		got := planCall(t, api.app, http.MethodGet, "/api/v1/planes?"+q.Encode(), nil, api.token, "")
		var out pagination.PageResult[planes.Plan]
		if got.status == http.StatusOK {
			require.NoError(t, json.Unmarshal([]byte(got.body), &out))
		}
		return got.status, out
	}
	status, first := page(planes.TabUpcoming, "")
	require.Equal(t, http.StatusOK, status)
	require.Len(t, first.Items, 2)
	require.NotEmpty(t, first.NextCursor)
	assert.Equal(t, future[1].ID, first.Items[0].ID, "próximos en orden ascendente")
	_, second := page(planes.TabUpcoming, first.NextCursor)
	require.Len(t, second.Items, 1)
	assert.Equal(t, future[0].ID, second.Items[0].ID)
	assert.Empty(t, second.NextCursor)

	_, pasts := page(planes.TabPast, "")
	require.Len(t, pasts.Items, 1)
	assert.Equal(t, past.ID, pasts.Items[0].ID)
	_, saved := page(planes.TabSaved, "")
	assert.Equal(t, future[0].ID, saved.Items[0].ID, "guardados de la más reciente a la más vieja")

	status, _ = page(planes.TabPast, first.NextCursor)
	assert.Equal(t, http.StatusUnprocessableEntity, status, "cursor de otra pestaña")
}

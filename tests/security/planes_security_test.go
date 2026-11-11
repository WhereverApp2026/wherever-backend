package security

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
)

// planNote es texto del plan que nunca debe llegar a los logs.
const planNote = "nota-privada-del-plan"

func sendIfMatch(t *testing.T, app *fiber.App, method, path string, body any, token string) reply {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set(fiber.HeaderIfMatch, `"1"`)
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

func businessDoc(slug, status string) bson.D {
	week := bson.A{}
	for d := 0; d < 7; d++ {
		week = append(week, bson.D{{Key: "dia", Value: d}, {Key: "desde", Value: 0}, {Key: "hasta", Value: 1440}})
	}
	return bson.D{
		{Key: "_id", Value: bson.NewObjectID()}, {Key: "slug", Value: slug}, {Key: "qr_checkin", Value: "qr-" + slug},
		{Key: "nombre", Value: slug}, {Key: "estado", Value: status}, {Key: "categoria", Value: "bar"},
		{Key: "ubicacion", Value: bson.D{{Key: "type", Value: "Point"}, {Key: "coordinates", Value: bson.A{-99.1332, 19.4326}}}},
		{Key: "horario_min", Value: week},
	}
}

func stopBody(business bson.ObjectID, hour string) map[string]any {
	return map[string]any{"negocio_id": business.Hex(), "llega": "2026-11-14T" + hour + ":00:00-06:00", "minutos": 30, "precio": 10, "nota": planNote}
}

func planBody(stops ...map[string]any) map[string]any {
	return map[string]any{"nombre": "Privado", "fecha": "2026-11-14", "personas": 1, "movilidad": "a_pie", "paradas": stops}
}

func TestPlanesSecurity_SinTokenEs401(t *testing.T) {
	app := authEnv(t).App()
	id := "/api/v1/planes/" + bson.NewObjectID().Hex()
	for _, r := range [][2]string{
		{http.MethodPost, "/api/v1/planes"}, {http.MethodGet, "/api/v1/planes"}, {http.MethodGet, id},
		{http.MethodPatch, id}, {http.MethodDelete, id}, {http.MethodPost, id + "/clonar"},
		{http.MethodPost, id + "/paradas"}, {http.MethodPut, id + "/paradas/orden"},
		{http.MethodPatch, id + "/paradas/" + bson.NewObjectID().Hex()}, {http.MethodDelete, id + "/paradas/" + bson.NewObjectID().Hex()},
	} {
		got := sendIfMatch(t, app, r[0], r[1], nil, "")
		assert.Equal(t, http.StatusUnauthorized, got.status, r[0]+" "+r[1])
	}
}

func TestPlanesSecurity_SoloElDuenoVeYEditaSuPlan(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	approved := businessDoc("plan-seguro", negocios.StatusApproved)
	_, err := env.Mongo.DB().Collection(negocios.Collection).InsertOne(context.Background(), approved)
	require.NoError(t, err)
	businessID, ok := approved[0].Value.(bson.ObjectID)
	require.True(t, ok)
	owner := env.CreateUser(t, "duena-plan@example.test")
	other := env.CreateUser(t, "otra-persona@example.test")
	ownerToken := testutil.ValidTokenFor(t, owner.ID.Hex(), "jti-plan-duena", middleware.RoleConsumer)
	otherToken := testutil.ValidTokenFor(t, other.ID.Hex(), "jti-plan-otra", middleware.RoleConsumer)

	created := sendIfMatch(t, app, http.MethodPost, "/api/v1/planes", planBody(stopBody(businessID, "12")), ownerToken)
	require.Equal(t, http.StatusCreated, created.status, created.body)
	assert.NotContains(t, created.body, "dueno_id")
	assert.NotContains(t, created.body, owner.ID.Hex())
	var p planes.Plan
	require.NoError(t, json.Unmarshal([]byte(created.body), &p))
	path := "/api/v1/planes/" + p.ID.Hex()
	stopPath := path + "/paradas/" + p.Paradas[0].ID.Hex()

	for _, r := range []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, path, nil},
		{http.MethodPatch, path, map[string]any{"nombre": "Robado"}},
		{http.MethodDelete, path, nil},
		{http.MethodPost, path + "/clonar", map[string]any{"fecha": "2026-11-21"}},
		{http.MethodPost, path + "/paradas", stopBody(businessID, "14")},
		{http.MethodPut, path + "/paradas/orden", map[string]any{"orden": []string{p.Paradas[0].ID.Hex()}}},
		{http.MethodPatch, stopPath, map[string]any{"minutos": 45}},
		{http.MethodDelete, stopPath, nil},
	} {
		got := sendIfMatch(t, app, r.method, r.path, r.body, otherToken)
		assert.Equal(t, http.StatusNotFound, got.status, r.method+" "+r.path)
		assert.NotContains(t, got.body, planNote)
	}
	list := sendIfMatch(t, app, http.MethodGet, "/api/v1/planes?pestana=guardados", nil, otherToken)
	require.Equal(t, http.StatusOK, list.status)
	assert.NotContains(t, list.body, p.ID.Hex(), "la lista de otra persona no lo trae")

	mine := sendIfMatch(t, app, http.MethodGet, path, nil, ownerToken)
	require.Equal(t, http.StatusOK, mine.status)
	assert.Contains(t, mine.body, `"version":1`, "nada lo cambió")
	assert.Contains(t, mine.body, "Privado")
	assert.NotContains(t, env.Logs.String(), planNote, "el contenido del plan no va a los logs")
}

func TestPlanesSecurity_NegocioNoAprobadoYSieteParadas(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	approved := businessDoc("plan-ok", negocios.StatusApproved)
	paused := businessDoc("plan-pausado", negocios.StatusPaused)
	_, err := env.Mongo.DB().Collection(negocios.Collection).InsertMany(context.Background(), []any{approved, paused})
	require.NoError(t, err)
	okID, ok := approved[0].Value.(bson.ObjectID)
	require.True(t, ok)
	pausedID, ok := paused[0].Value.(bson.ObjectID)
	require.True(t, ok)
	u := env.CreateUser(t, "planes-limites@example.test")
	token := testutil.ValidTokenFor(t, u.ID.Hex(), "jti-plan-limites", middleware.RoleConsumer)

	got := sendIfMatch(t, app, http.MethodPost, "/api/v1/planes", planBody(stopBody(pausedID, "12")), token)
	assert.Equal(t, http.StatusUnprocessableEntity, got.status, got.body)
	assert.Contains(t, got.body, planes.ReasonUnavailable)

	seven := []map[string]any{}
	for _, hour := range []string{"08", "09", "10", "11", "12", "13", "14"} {
		seven = append(seven, stopBody(okID, hour))
	}
	got = sendIfMatch(t, app, http.MethodPost, "/api/v1/planes", planBody(seven...), token)
	assert.Equal(t, http.StatusUnprocessableEntity, got.status, got.body)
}

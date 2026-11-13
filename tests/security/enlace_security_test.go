package security

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/grupos"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
)

type testEnvApp struct {
	env *authtest.Env
	app *fiber.App
}

func mustHex(t *testing.T, hex string) bson.ObjectID {
	t.Helper()
	id, err := bson.ObjectIDFromHex(hex)
	require.NoError(t, err)
	return id
}

// sharedPlan guarda un plan del dueño directo en la base y lo comparte por HTTP.
func sharedPlan(t *testing.T) (*testEnvApp, grupos.ShareResponse, string) {
	t.Helper()
	env := authEnv(t)
	env.ClearRateLimits(t)
	owner := env.CreateUser(t, "comparte@wherever.test")
	p := planes.Plan{
		OwnerID: owner.ID, Nombre: "Cumpleaños de Lucía", Fecha: time.Now().AddDate(0, 0, 3), Personas: 3, Movilidad: planes.MobilityWalk,
		Paradas: []planes.Parada{{ID: bson.NewObjectID(), NegocioID: bson.NewObjectID(), Minutos: 60, Nota: "llevar pastel"}},
	}
	require.NoError(t, planes.NewRepository(env.Mongo.DB()).Create(context.Background(), &p))
	token := testutil.ValidTokenFor(t, owner.ID.Hex(), "jti-comparte", middleware.RoleConsumer)
	app := env.App()
	got := send(t, app, http.MethodPost, "/api/v1/planes/"+p.ID.Hex()+"/compartir", nil, token)
	require.Equal(t, http.StatusCreated, got.status, got.body)
	var link grupos.ShareResponse
	require.NoError(t, json.Unmarshal([]byte(got.body), &link))
	return &testEnvApp{env: env, app: app}, link, owner.Email
}

func TestEnlaceSecurity_CodigoSoloComoHashYFueraDeLogs(t *testing.T) {
	te, link, ownerEmail := sharedPlan(t)
	var g grupos.Group
	require.NoError(t, te.env.Mongo.DB().Collection(grupos.Collection).FindOne(context.Background(), bson.D{{Key: "_id", Value: mustHex(t, link.GroupID)}}).Decode(&g))
	assert.NotEmpty(t, g.CodeHash)
	assert.NotEqual(t, link.Code, g.CodeHash)
	raw, err := bson.Marshal(g)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), link.Code, "el código no se guarda en claro")

	view := send(t, te.app, http.MethodPost, "/api/v1/planes/compartidos", map[string]string{"codigo": link.Code}, "")
	require.Equal(t, http.StatusOK, view.status, view.body)
	assert.NotContains(t, view.body, ownerEmail)
	assert.NotContains(t, view.body, g.OrganizerID.Hex())
	assert.NotContains(t, view.body, "llevar pastel", "las notas no son públicas")
	assert.NotContains(t, te.env.Logs.String(), link.Code, "el código no llega a los logs")
	assert.Empty(t, view.headers.Get("Content-Encoding"))
}

func TestEnlaceSecurity_LimitePorIPYCodigoInventado(t *testing.T) {
	te, _, _ := sharedPlan(t)
	for range 30 {
		got := send(t, te.app, http.MethodPost, "/api/v1/planes/compartidos", map[string]string{"codigo": "inventado"}, "")
		require.Equal(t, http.StatusNotFound, got.status)
	}
	got := send(t, te.app, http.MethodPost, "/api/v1/planes/compartidos", map[string]string{"codigo": "inventado"}, "")
	assert.Equal(t, http.StatusTooManyRequests, got.status)
	assert.NotEmpty(t, got.headers.Get("Retry-After"))
}

func TestEnlaceSecurity_CompartirYUnirsePidenSesion(t *testing.T) {
	te, link, _ := sharedPlan(t)
	id := bson.NewObjectID().Hex()
	assert.Equal(t, http.StatusUnauthorized, send(t, te.app, http.MethodPost, "/api/v1/planes/"+id+"/compartir", nil, "").status)
	assert.Equal(t, http.StatusUnauthorized, send(t, te.app, http.MethodDelete, "/api/v1/planes/"+id+"/compartir", nil, "").status)
	assert.Equal(t, http.StatusUnauthorized, send(t, te.app, http.MethodPost, "/api/v1/grupos/unirse", map[string]string{"codigo": link.Code}, "").status)
}

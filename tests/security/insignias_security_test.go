package security

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// Insignias (nv-78): sesión, cuenta activa y las reglas del perfil de persona (bloqueo,
// inactiva, propia o inexistente = el mismo 404).
func TestInsigniasSecurity_SesionCuentaYVisibilidad(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	ana := env.CreateUser(t, "ana-insignias-seg@wherever.test")
	beto := env.CreateUser(t, "beto-insignias-seg@wherever.test")
	anaToken := testutil.ValidTokenFor(t, ana.ID.Hex(), "jti-ana-insignias-seg", middleware.RoleConsumer)
	betoToken := testutil.ValidTokenFor(t, beto.ID.Hex(), "jti-beto-insignias-seg", middleware.RoleConsumer)
	rutas := []string{"/api/v1/usuarios/yo/insignias", "/api/v1/personas/" + beto.ID.Hex() + "/insignias"}

	for _, path := range rutas {
		for name, token := range map[string]string{"sin token": "", "vencido": testutil.ExpiredToken(t), "sin firma": testutil.UnsignedToken(t), "alg none": testutil.AlgNoneToken(t)} {
			assert.Equal(t, http.StatusUnauthorized, send(t, app, http.MethodGet, path, nil, token).status, "%s %s", path, name)
		}
	}
	assert.Equal(t, http.StatusOK, send(t, app, http.MethodGet, rutas[1], nil, anaToken).status)

	for _, id := range []string{ana.ID.Hex(), bson.NewObjectID().Hex()} {
		assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodGet, "/api/v1/personas/"+id+"/insignias", nil, anaToken).status, "propia o inexistente: "+id)
	}
	assert.Equal(t, http.StatusUnprocessableEntity, send(t, app, http.MethodGet, "/api/v1/personas/%7B%22%24ne%22%3Anull%7D/insignias", nil, anaToken).status)

	// Con bloqueo en cualquier sentido, 404.
	_, err := env.Mongo.DB().Collection(amistades.Collection).InsertOne(context.Background(), amistades.Friendship{
		ID: bson.NewObjectID(), From: beto.ID, To: ana.ID, Status: amistades.StatusBlocked, Pair: amistades.PairKey(ana.ID, beto.ID)})
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodGet, rutas[1], nil, anaToken).status, "Beto bloqueó a Ana")
	assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodGet, "/api/v1/personas/"+ana.ID.Hex()+"/insignias", nil, betoToken).status, "y al revés")

	// Cuenta en eliminación pendiente: 403 en las propias.
	_, err = env.Users.Update(context.Background(), ana.ID, bson.D{{Key: "estado", Value: usuarios.StatusPendingDeletion}})
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, send(t, app, http.MethodGet, rutas[0], nil, anaToken).status)
}

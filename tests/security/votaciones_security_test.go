package security

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
	"github.com/WhereverApp2026/wherever-backend/internal/votaciones"
)

// Las cuatro rutas de votaciones (nv-39) piden sesión; las escrituras, cuenta activa; una
// votación de otro grupo responde 404 igual que una inexistente.
func TestVotacionesSecurity_SesionCuentaActivaYAjeno(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	id := bson.NewObjectID().Hex()
	writes := []struct {
		method, path string
		body         any
	}{
		{http.MethodPost, "/api/v1/grupos/" + id + "/votaciones", map[string]any{"parada_id": id, "opciones": []string{id, bson.NewObjectID().Hex()}, "cierra_en": time.Now().Add(time.Hour)}},
		{http.MethodPut, "/api/v1/votaciones/" + id + "/voto", map[string]string{"negocio_id": id}},
		{http.MethodPost, "/api/v1/votaciones/" + id + "/cerrar", nil},
	}
	pending := env.CreateUser(t, "pendiente-votos@wherever.test")
	_, err := env.Users.Update(context.Background(), pending.ID, bson.D{{Key: "estado", Value: usuarios.StatusPendingDeletion}})
	require.NoError(t, err)
	pendingToken := testutil.ValidTokenFor(t, pending.ID.Hex(), "jti-pendiente-votos", middleware.RoleConsumer)
	for _, r := range writes {
		assert.Equal(t, http.StatusUnauthorized, send(t, app, r.method, r.path, r.body, "").status, r.path)
		assert.Equal(t, http.StatusForbidden, send(t, app, r.method, r.path, r.body, pendingToken).status, r.path)
	}
	assert.Equal(t, http.StatusUnauthorized, send(t, app, http.MethodGet, "/api/v1/grupos/"+id+"/votaciones", nil, "").status)

	// Una votación de un grupo ajeno: 404 como si no existiera, también para votar.
	outsider := env.CreateUser(t, "ajeno-votos@wherever.test")
	v := votaciones.Vote{
		ID: bson.NewObjectID(), GroupID: bson.NewObjectID(), PlanID: bson.NewObjectID(), StopID: bson.NewObjectID(),
		Options: []bson.ObjectID{bson.NewObjectID(), bson.NewObjectID()}, Ballots: map[string]bson.ObjectID{},
		ClosesAt: time.Now().Add(time.Hour), Status: votaciones.StatusOpen, CreatedBy: bson.NewObjectID(), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	require.NoError(t, votaciones.NewRepository(env.Mongo.DB()).Create(context.Background(), &v))
	token := testutil.ValidTokenFor(t, outsider.ID.Hex(), "jti-ajeno-votos", middleware.RoleConsumer)
	missing := send(t, app, http.MethodPut, "/api/v1/votaciones/"+bson.NewObjectID().Hex()+"/voto", map[string]string{"negocio_id": v.Options[0].Hex()}, token)
	foreign := send(t, app, http.MethodPut, "/api/v1/votaciones/"+v.ID.Hex()+"/voto", map[string]string{"negocio_id": v.Options[0].Hex()}, token)
	assert.Equal(t, http.StatusNotFound, foreign.status)
	assert.Equal(t, missing.status, foreign.status)
}

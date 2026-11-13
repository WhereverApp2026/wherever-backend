package security

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/grupos"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// Las ocho rutas de grupos (nv-37) piden sesión; las escrituras, además, cuenta activa.
func TestGruposSecurity_RequiereSesionYCuentaActivaParaEscribir(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	id := bson.NewObjectID().Hex()
	users := map[string][]string{"usuarios": {bson.NewObjectID().Hex()}}
	writes := []struct {
		method, path string
		body         any
	}{
		{http.MethodPost, "/api/v1/grupos", map[string]string{"plan_id": id}},
		{http.MethodPost, "/api/v1/grupos/" + id + "/integrantes", users},
		{http.MethodPost, "/api/v1/planes/" + id + "/invitaciones", users},
		{http.MethodPatch, "/api/v1/grupos/" + id + "/integrantes/yo", map[string]string{"estado": "confirmado"}},
		{http.MethodDelete, "/api/v1/grupos/" + id + "/integrantes/yo", nil},
		{http.MethodDelete, "/api/v1/grupos/" + id + "/integrantes/" + id, nil},
	}
	reads := []string{"/api/v1/grupos", "/api/v1/grupos/" + id}

	pending := env.CreateUser(t, "pendiente-grupos@wherever.test")
	_, err := env.Users.Update(context.Background(), pending.ID, bson.D{{Key: "estado", Value: usuarios.StatusPendingDeletion}})
	require.NoError(t, err)
	token := testutil.ValidTokenFor(t, pending.ID.Hex(), "jti-pendiente-grupos", middleware.RoleConsumer)

	for _, r := range writes {
		assert.Equal(t, http.StatusUnauthorized, send(t, app, r.method, r.path, r.body, "").status, r.method+" "+r.path)
		assert.Equal(t, http.StatusForbidden, send(t, app, r.method, r.path, r.body, token).status, r.method+" "+r.path)
	}
	for _, path := range reads {
		assert.Equal(t, http.StatusUnauthorized, send(t, app, http.MethodGet, path, nil, "").status, path)
	}
}

// El grupo de otros responde 404 igual que uno que no existe y no deja ver a nadie.
func TestGruposSecurity_GrupoAjenoNoRevelaNada(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	owner := env.CreateUser(t, "dueno-grupo@wherever.test")
	member := env.CreateUser(t, "integrante-grupo@wherever.test")
	outsider := env.CreateUser(t, "ajeno-grupo@wherever.test")
	now := time.Now().UTC()
	g := grupos.Group{
		ID: bson.NewObjectID(), PlanID: bson.NewObjectID(), OrganizerID: owner.ID, Occupied: 2, CreatedAt: now, UpdatedAt: now,
		Members: []grupos.Member{
			{UserID: owner.ID, Role: grupos.RoleOrganizer, Status: grupos.StatusConfirmed, Since: now},
			{UserID: member.ID, Role: grupos.RoleMember, Status: grupos.StatusInvited, Since: now},
		},
	}
	require.NoError(t, grupos.NewRepository(env.Mongo.DB()).Create(context.Background(), &g))

	token := testutil.ValidTokenFor(t, outsider.ID.Hex(), "jti-ajeno-grupo", middleware.RoleConsumer)
	missing := send(t, app, http.MethodGet, "/api/v1/grupos/"+bson.NewObjectID().Hex(), nil, token)
	foreign := send(t, app, http.MethodGet, "/api/v1/grupos/"+g.ID.Hex(), nil, token)
	assert.Equal(t, http.StatusNotFound, foreign.status)
	assert.Equal(t, missing.status, foreign.status)
	assert.NotContains(t, foreign.body, member.ID.Hex())
	assert.NotContains(t, foreign.body, owner.ID.Hex())

	// Tampoco puede sacar a nadie ni responder por otro.
	assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodDelete, "/api/v1/grupos/"+g.ID.Hex()+"/integrantes/"+member.ID.Hex(), nil, token).status)
	assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodPatch, "/api/v1/grupos/"+g.ID.Hex()+"/integrantes/yo", map[string]string{"estado": "confirmado"}, token).status)
}

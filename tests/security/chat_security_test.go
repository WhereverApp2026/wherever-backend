package security

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/chat"
	"github.com/WhereverApp2026/wherever-backend/internal/grupos"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// Las cuatro rutas del chat (nv-57) piden sesión; las escrituras, además, cuenta activa.
func TestChatSecurity_SesionYCuentaActiva(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	id := bson.NewObjectID().Hex()
	base := "/api/v1/grupos/" + id + "/mensajes"
	writes := []struct {
		method, path string
		body         any
	}{
		{http.MethodPost, base, map[string]string{"texto": "hola", "id_cliente": "3f2b8c1e-5d4a-4b6f-9a7e-1c2d3e4f5a6b"}},
		{http.MethodPut, base + "/" + id + "/reacciones", map[string]string{"emoji": "👍"}},
		{http.MethodDelete, base + "/" + id, nil},
	}
	pending := env.CreateUser(t, "pendiente-chat@wherever.test")
	_, err := env.Users.Update(context.Background(), pending.ID, bson.D{{Key: "estado", Value: usuarios.StatusPendingDeletion}})
	require.NoError(t, err)
	token := testutil.ValidTokenFor(t, pending.ID.Hex(), "jti-pendiente-chat", middleware.RoleConsumer)
	for _, r := range writes {
		assert.Equal(t, http.StatusUnauthorized, send(t, app, r.method, r.path, r.body, "").status, r.method+" "+r.path)
		assert.Equal(t, http.StatusForbidden, send(t, app, r.method, r.path, r.body, token).status, r.method+" "+r.path)
	}
	assert.Equal(t, http.StatusUnauthorized, send(t, app, http.MethodGet, base, nil, "").status)
	expired := testutil.ExpiredToken(t)
	assert.Equal(t, http.StatusUnauthorized, send(t, app, http.MethodGet, base, nil, expired).status)
}

// Un grupo ajeno (o donde solo está invitado) responde 404 igual que uno inexistente y no
// deja ver ningún mensaje.
func TestChatSecurity_GrupoAjenoNoRevelaNada(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	owner := env.CreateUser(t, "dueno-chat@wherever.test")
	invited := env.CreateUser(t, "invitado-chat@wherever.test")
	outsider := env.CreateUser(t, "ajeno-chat@wherever.test")
	now := time.Now().UTC()
	g := grupos.Group{
		ID: bson.NewObjectID(), PlanID: bson.NewObjectID(), OrganizerID: owner.ID, Occupied: 2, CreatedAt: now, UpdatedAt: now,
		Members: []grupos.Member{
			{UserID: owner.ID, Role: grupos.RoleOrganizer, Status: grupos.StatusConfirmed, Since: now},
			{UserID: invited.ID, Role: grupos.RoleMember, Status: grupos.StatusInvited, Since: now},
		},
	}
	require.NoError(t, grupos.NewRepository(env.Mongo.DB()).Create(context.Background(), &g))
	m := chat.Message{ID: bson.NewObjectID(), GroupID: g.ID, AuthorID: &owner.ID, Kind: chat.KindText, Text: "secreto", Reactions: []chat.Reaction{}, CreatedAt: now}
	require.NoError(t, chat.NewRepository(env.Mongo.DB()).Insert(context.Background(), &m))

	for i, who := range []usuarios.User{outsider, invited} {
		token := testutil.ValidTokenFor(t, who.ID.Hex(), "jti-ajeno-chat-"+string(rune('a'+i)), middleware.RoleConsumer)
		missing := send(t, app, http.MethodGet, "/api/v1/grupos/"+bson.NewObjectID().Hex()+"/mensajes", nil, token)
		foreign := send(t, app, http.MethodGet, "/api/v1/grupos/"+g.ID.Hex()+"/mensajes", nil, token)
		assert.Equal(t, http.StatusNotFound, foreign.status)
		assert.Equal(t, missing.status, foreign.status)
		assert.NotContains(t, foreign.body, "secreto")
		path := "/api/v1/grupos/" + g.ID.Hex() + "/mensajes/" + m.ID.Hex()
		assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodPut, path+"/reacciones", map[string]string{"emoji": "👍"}, token).status)
		assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodDelete, path, nil, token).status)
	}
}

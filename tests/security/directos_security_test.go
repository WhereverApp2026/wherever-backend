package security

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/chat"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

const dmUUID = "3f2b8c1e-5d4a-4b6f-9a7e-1c2d3e4f5a6b"

// Las seis rutas de mensajes directos (nv-87) piden sesión y cuenta activa.
func TestDirectosSecurity_SesionYCuentaActiva(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	id := bson.NewObjectID().Hex()
	base := "/api/v1/conversaciones"
	routes := []struct {
		method, path string
		body         any
	}{
		{http.MethodPost, base, map[string]string{"usuario_id": id}},
		{http.MethodGet, base, nil},
		{http.MethodGet, base + "/" + id + "/mensajes", nil},
		{http.MethodPost, base + "/" + id + "/mensajes", map[string]string{"texto": "hola", "id_cliente": dmUUID}},
		{http.MethodDelete, base + "/" + id + "/mensajes/" + id, nil},
		{http.MethodPut, base + "/" + id + "/mensajes/" + id + "/reacciones", map[string]string{"emoji": "👍"}},
		{http.MethodPost, base + "/" + id + "/leido", nil},
	}
	pending := env.CreateUser(t, "pendiente-dm@wherever.test")
	_, err := env.Users.Update(context.Background(), pending.ID, bson.D{{Key: "estado", Value: usuarios.StatusPendingDeletion}})
	require.NoError(t, err)
	token := testutil.ValidTokenFor(t, pending.ID.Hex(), "jti-pendiente-dm", middleware.RoleConsumer)
	expired := testutil.ExpiredToken(t)
	for _, r := range routes {
		assert.Equal(t, http.StatusUnauthorized, send(t, app, r.method, r.path, r.body, "").status, r.method+" "+r.path)
		assert.Equal(t, http.StatusUnauthorized, send(t, app, r.method, r.path, r.body, expired).status, r.method+" "+r.path)
		assert.Equal(t, http.StatusForbidden, send(t, app, r.method, r.path, r.body, token).status, r.method+" "+r.path)
	}
}

type dmPeople struct {
	a, b, outsider usuarios.User
	tokens         map[bson.ObjectID]string
	conv           chat.Conversation
	msg            chat.Message
}

// dmSeed siembra dos amigos con una conversación y un mensaje secreto, y una tercera persona.
func dmSeed(t *testing.T, env *authtest.Env, tag string) dmPeople {
	t.Helper()
	p := dmPeople{
		a:        env.CreateUser(t, "a-"+tag+"@wherever.test"),
		b:        env.CreateUser(t, "b-"+tag+"@wherever.test"),
		outsider: env.CreateUser(t, "c-"+tag+"@wherever.test"),
		tokens:   map[bson.ObjectID]string{},
	}
	for i, u := range []usuarios.User{p.a, p.b, p.outsider} {
		p.tokens[u.ID] = testutil.ValidTokenFor(t, u.ID.Hex(), "jti-dm-"+tag+string(rune('a'+i)), middleware.RoleConsumer)
	}
	db := env.Mongo.DB()
	friends, convs, msgs := amistades.NewRepository(db), chat.NewConversationRepository(db), chat.NewRepository(db)
	now := time.Now().UTC()
	f := amistades.Friendship{ID: bson.NewObjectID(), From: p.a.ID, To: p.b.ID, Status: amistades.StatusAccepted, Pair: amistades.PairKey(p.a.ID, p.b.ID), CreatedAt: now, UpdatedAt: now}
	require.NoError(t, friends.Insert(context.Background(), &f))
	conv, _, err := convs.Open(context.Background(), p.a.ID, p.b.ID, now)
	require.NoError(t, err)
	p.conv = conv
	p.msg = chat.Message{ID: bson.NewObjectID(), ConversationID: conv.ID, AuthorID: &p.a.ID, Kind: chat.KindText, Text: "secreto-directo", Reactions: []chat.Reaction{}, CreatedAt: now}
	require.NoError(t, msgs.Insert(context.Background(), &p.msg))
	require.NoError(t, convs.SetLast(context.Background(), conv.ID, chat.DirectLast{MessageID: p.msg.ID, Text: p.msg.Text, AuthorID: &p.a.ID, At: now}))
	return p
}

// Quien no participa recibe 404, igual que una conversación inexistente, y no ve nada; tampoco
// puede abrir una conversación con quien no es su amigo.
func TestDirectosSecurity_NoParticipanteYSinAmistad(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	p := dmSeed(t, env, "ajeno")
	token := p.tokens[p.outsider.ID]
	base := "/api/v1/conversaciones/" + p.conv.ID.Hex()
	missing := send(t, app, http.MethodGet, "/api/v1/conversaciones/"+bson.NewObjectID().Hex()+"/mensajes", nil, token)
	foreign := send(t, app, http.MethodGet, base+"/mensajes", nil, token)
	assert.Equal(t, http.StatusNotFound, foreign.status)
	assert.Equal(t, missing.status, foreign.status)
	assert.NotContains(t, foreign.body, "secreto-directo")
	assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodPost, base+"/mensajes", map[string]string{"texto": "hola", "id_cliente": dmUUID}, token).status)
	assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodPut, base+"/mensajes/"+p.msg.ID.Hex()+"/reacciones", map[string]string{"emoji": "👍"}, token).status)
	assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodDelete, base+"/mensajes/"+p.msg.ID.Hex(), nil, token).status)
	assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodPost, base+"/leido", nil, token).status)
	list := send(t, app, http.MethodGet, "/api/v1/conversaciones", nil, token)
	assert.Equal(t, http.StatusOK, list.status)
	assert.NotContains(t, list.body, "secreto-directo")

	// Sin amistad: 404, sin decir si la persona existe.
	assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodPost, "/api/v1/conversaciones", map[string]string{"usuario_id": p.a.ID.Hex()}, token).status)
	// Borrar el mensaje de la otra persona: 404.
	assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodDelete, base+"/mensajes/"+p.msg.ID.Hex(), nil, p.tokens[p.b.ID]).status)
}

// Un adjunto que no puedo ver es 422 sin filtrar datos, y el texto nunca llega a los logs.
func TestDirectosSecurity_AdjuntoAjenoYLogsSinTexto(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	p := dmSeed(t, env, "adjunto")
	path := "/api/v1/conversaciones/" + p.conv.ID.Hex() + "/mensajes"
	for _, kind := range []string{"plan", "momento", "lugar"} {
		r := send(t, app, http.MethodPost, path, map[string]any{"texto": "mira", "id_cliente": dmUUID, "adjunto": map[string]string{"tipo": kind, "id": bson.NewObjectID().Hex()}}, p.tokens[p.a.ID])
		assert.Equal(t, http.StatusUnprocessableEntity, r.status, kind+": "+r.body)
		assert.Contains(t, r.body, "adjunto")
	}
	ok := send(t, app, http.MethodPost, path, map[string]any{"texto": "texto-privado-del-chat", "id_cliente": "6a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d"}, p.tokens[p.b.ID])
	require.Equal(t, http.StatusCreated, ok.status, ok.body)
	assert.NotContains(t, env.Logs.String(), "texto-privado-del-chat")
	assert.NotContains(t, env.Logs.String(), "secreto-directo")
}

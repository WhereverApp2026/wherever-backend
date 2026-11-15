package contract

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// TestContract_PersonasPerfil fija las formas de nv-70: perfil de amigo (degradado sin grafo),
// 404, silencio y contactos.
func TestContract_PersonasPerfil(t *testing.T) {
	env := authtest.New(t, mongoClient, redisClient, testutil.Config(config.EnvProd))
	app := env.App()
	me := env.CreateUser(t, "yo-perfil-contrato@wherever.test")
	friend := env.CreateUser(t, "amiga-perfil-contrato@wherever.test")
	_, err := env.Users.Update(context.Background(), friend.ID, bson.D{
		{Key: "telefono_hash", Value: usuarios.HashPhone(env.Cfg.PhoneHMACKey, "+525587654321")}, {Key: "privacidad.buscable_por_telefono", Value: true},
	})
	require.NoError(t, err)
	now := time.Now().UTC()
	f := amistades.Friendship{ID: bson.NewObjectID(), From: me.ID, To: friend.ID, Status: amistades.StatusAccepted, Pair: amistades.PairKey(me.ID, friend.ID), CreatedAt: now, UpdatedAt: now}
	require.NoError(t, amistades.NewRepository(env.Mongo.DB()).Insert(context.Background(), &f))
	token := testutil.ValidTokenFor(t, me.ID.Hex(), "jti-yo-perfil-contrato", middleware.RoleConsumer)

	status, body := call(t, app, http.MethodGet, "/api/v1/personas/"+friend.ID.Hex(), nil, token)
	check(t, "personas/perfil_amigo_200", status, body)
	status, body = call(t, app, http.MethodGet, "/api/v1/personas/"+bson.NewObjectID().Hex(), nil, token)
	check(t, "personas/perfil_404", status, body)
	status, body = call(t, app, http.MethodPut, "/api/v1/amistades/"+f.ID.Hex()+"/silencio", map[string]bool{"silenciado": true}, token)
	check(t, "personas/silencio_200", status, body)
	status, body = call(t, app, http.MethodPost, "/api/v1/personas/contactos", map[string][]string{"telefonos": {"5587654321"}}, token)
	check(t, "personas/contactos_200", status, body)
}

package security

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// Las tres rutas de nv-70 piden sesión y cuenta activa.
func TestPersonasPrivacidad_SesionYCuentaActiva(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	id := bson.NewObjectID().Hex()
	routes := []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, "/api/v1/personas/" + id, nil},
		{http.MethodPut, "/api/v1/amistades/" + id + "/silencio", map[string]bool{"silenciado": true}},
		{http.MethodPost, "/api/v1/personas/contactos", map[string][]string{"telefonos": {"5512345678"}}},
	}
	pending := env.CreateUser(t, "pendiente-personas@wherever.test")
	_, err := env.Users.Update(context.Background(), pending.ID, bson.D{{Key: "estado", Value: usuarios.StatusPendingDeletion}})
	require.NoError(t, err)
	token := testutil.ValidTokenFor(t, pending.ID.Hex(), "jti-pendiente-personas", middleware.RoleConsumer)
	for _, r := range routes {
		assert.Equal(t, http.StatusUnauthorized, send(t, app, r.method, r.path, r.body, "").status, r.method+" "+r.path)
		assert.Equal(t, http.StatusForbidden, send(t, app, r.method, r.path, r.body, token).status, r.method+" "+r.path)
	}
}

// Ni el perfil ni los contactos revelan correo, teléfono, hash o código de amigo; el silencio
// del otro nunca sale; los números no llegan a los logs.
func TestPersonasPrivacidad_NadaSensibleSale(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	me := env.CreateUser(t, "yo-privacidad70@wherever.test")
	friend := env.CreateUser(t, "amiga-privacidad70@wherever.test")
	stranger := env.CreateUser(t, "ajena-privacidad70@wherever.test")
	hash := usuarios.HashPhone(env.Cfg.PhoneHMACKey, "+525598765432")
	_, err := env.Users.Update(context.Background(), friend.ID, bson.D{{Key: "telefono_hash", Value: hash}, {Key: "privacidad.buscable_por_telefono", Value: true}})
	require.NoError(t, err)
	now := time.Now().UTC()
	f := amistades.Friendship{ID: bson.NewObjectID(), From: me.ID, To: friend.ID, Status: amistades.StatusAccepted, Pair: amistades.PairKey(me.ID, friend.ID),
		MutedBy: []bson.ObjectID{friend.ID}, CreatedAt: now, UpdatedAt: now}
	require.NoError(t, amistades.NewRepository(env.Mongo.DB()).Insert(context.Background(), &f))
	token := testutil.ValidTokenFor(t, me.ID.Hex(), "jti-yo-privacidad70", middleware.RoleConsumer)

	basic := send(t, app, http.MethodGet, "/api/v1/personas/"+stranger.ID.Hex(), nil, token)
	require.Equal(t, http.StatusOK, basic.status, basic.body)
	for _, hidden := range []string{"amigos_en_comun", "grupos_en_comun"} {
		assert.NotContains(t, basic.body, hidden)
	}
	full := send(t, app, http.MethodGet, "/api/v1/personas/"+friend.ID.Hex(), nil, token)
	require.Equal(t, http.StatusOK, full.status, full.body)
	assert.Contains(t, full.body, `"silenciado":false`, "la marca de la amiga sobre mí no sale")
	found := send(t, app, http.MethodPost, "/api/v1/personas/contactos", map[string][]string{"telefonos": {"55 9876 5432"}}, token)
	require.Equal(t, http.StatusOK, found.status, found.body)
	assert.Contains(t, found.body, friend.ID.Hex())
	for _, body := range []string{basic.body, full.body, found.body} {
		for _, leak := range []string{"@wherever.test", "telefono", hash, "codigo_amigo", "5598765432"} {
			assert.NotContains(t, body, leak)
		}
	}
	logs := env.Logs.String()
	for _, leak := range []string{"5598765432", "9876 5432", hash} {
		assert.False(t, strings.Contains(logs, leak), "el número no llega a los logs")
	}
}

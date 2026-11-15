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

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// Las vistas piden sesión y cuenta activa; nadie ve las de otro; ningún log junta persona y
// lugar de una vista.
func TestVistasSecurity_SesionPropiedadYLogs(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	place := negocios.Business{ID: bson.NewObjectID(), Slug: "vista-sec", QRCheckin: "qr-vista-sec", Name: "Vista seguridad", Category: "restaurante",
		State: negocios.StatusApproved, Location: negocios.Location{Type: "Point", Coordinates: []float64{-99.13, 19.43}}, CreatedAt: time.Now()}
	_, err := env.Mongo.DB().Collection(negocios.Collection).InsertOne(context.Background(), place)
	require.NoError(t, err)
	routes := [][2]string{{http.MethodPost, "/api/v1/lugares/" + place.ID.Hex() + "/vistas"}, {http.MethodGet, "/api/v1/lugares/recientes"}, {http.MethodDelete, "/api/v1/lugares/recientes"}}
	pending := env.CreateUser(t, "pendiente-vistas@wherever.test")
	_, err = env.Users.Update(context.Background(), pending.ID, bson.D{{Key: "estado", Value: usuarios.StatusPendingDeletion}})
	require.NoError(t, err)
	pendingToken := testutil.ValidTokenFor(t, pending.ID.Hex(), "jti-pendiente-vistas", middleware.RoleConsumer)
	for _, r := range routes {
		assert.Equal(t, http.StatusUnauthorized, send(t, app, r[0], r[1], nil, "").status, r[0]+" "+r[1])
		assert.Equal(t, http.StatusForbidden, send(t, app, r[0], r[1], nil, pendingToken).status, r[0]+" "+r[1])
	}

	me := env.CreateUser(t, "vistas-sec-yo@wherever.test")
	other := env.CreateUser(t, "vistas-sec-otra@wherever.test")
	myToken := testutil.ValidTokenFor(t, me.ID.Hex(), "jti-vistas-sec-yo", middleware.RoleConsumer)
	otherToken := testutil.ValidTokenFor(t, other.ID.Hex(), "jti-vistas-sec-otra", middleware.RoleConsumer)
	require.Equal(t, http.StatusNoContent, send(t, app, http.MethodPost, routes[0][1], map[string]string{"origen": "mapa"}, myToken).status)
	theirs := send(t, app, http.MethodGet, "/api/v1/lugares/recientes", nil, otherToken)
	require.Equal(t, http.StatusOK, theirs.status)
	assert.NotContains(t, theirs.body, place.ID.Hex())
	mine := send(t, app, http.MethodGet, "/api/v1/lugares/recientes", nil, myToken)
	assert.Contains(t, mine.body, place.ID.Hex())
	assert.NotContains(t, mine.body, me.ID.Hex())

	for _, line := range strings.Split(env.Logs.String(), "\n") {
		assert.False(t, strings.Contains(line, me.ID.Hex()) && strings.Contains(line, place.ID.Hex()), "un log no junta persona y lugar: %s", line)
	}
}

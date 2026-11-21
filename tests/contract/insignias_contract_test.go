package contract

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/insignias"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
)

// TestContract_Insignias fija las formas de nv-78: las propias (ganada y bloqueada) y las
// ganadas de otra persona.
func TestContract_Insignias(t *testing.T) {
	env := authtest.New(t, mongoClient, redisClient, testutil.Config(config.EnvProd))
	app := env.App()
	ana := env.CreateUser(t, "ana-insignias-contrato@wherever.test")
	beto := env.CreateUser(t, "beto-insignias-contrato@wherever.test")
	_, err := env.Mongo.DB().Collection(insignias.ColPlanes).InsertOne(context.Background(), bson.D{{Key: "_id", Value: bson.NewObjectID()}, {Key: "dueno_id", Value: ana.ID}, {Key: "estado", Value: "activo"}})
	require.NoError(t, err)
	anaToken := testutil.ValidTokenFor(t, ana.ID.Hex(), "jti-ana-insignias-contrato", middleware.RoleConsumer)
	betoToken := testutil.ValidTokenFor(t, beto.ID.Hex(), "jti-beto-insignias-contrato", middleware.RoleConsumer)

	status, body := call(t, app, http.MethodGet, "/api/v1/usuarios/yo/insignias", nil, anaToken)
	check(t, "insignias/mias_200", status, body)
	status, body = call(t, app, http.MethodGet, "/api/v1/personas/"+ana.ID.Hex()+"/insignias", nil, betoToken)
	check(t, "insignias/persona_200", status, body)
}

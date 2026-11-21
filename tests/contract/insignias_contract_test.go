package contract

import (
	"context"
	"fmt"
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

// TestContract_Insignias fija las formas de nv-78 y nv-79: las propias (ganada y bloqueada) con
// nivel y racha, y las ganadas de otra persona. Ana visitó 5 cafeterías: la primera del
// catálogo (`catador_cafe`) sale ganada.
func TestContract_Insignias(t *testing.T) {
	env := authtest.New(t, mongoClient, redisClient, testutil.Config(config.EnvProd))
	app := env.App()
	ana := env.CreateUser(t, "ana-insignias-contrato@wherever.test")
	beto := env.CreateUser(t, "beto-insignias-contrato@wherever.test")
	db := env.Mongo.DB()
	for i := range 5 {
		negocio := bson.NewObjectID()
		_, err := db.Collection(insignias.ColNegocios).InsertOne(context.Background(), bson.D{{Key: "_id", Value: negocio}, {Key: "slug", Value: "contrato-insignias-" + negocio.Hex()}, {Key: "qr_checkin", Value: "qr-" + negocio.Hex()}, {Key: "categoria", Value: insignias.CategoriaCafeteria}})
		require.NoError(t, err)
		_, err = db.Collection(insignias.ColCheckins).InsertOne(context.Background(), bson.D{{Key: "_id", Value: bson.NewObjectID()}, {Key: "usuario_id", Value: ana.ID},
			{Key: "negocio_id", Value: negocio}, {Key: "valido", Value: true}, {Key: "dia", Value: fmt.Sprintf("2026-11-0%d", i+1)}})
		require.NoError(t, err)
	}
	anaToken := testutil.ValidTokenFor(t, ana.ID.Hex(), "jti-ana-insignias-contrato", middleware.RoleConsumer)
	betoToken := testutil.ValidTokenFor(t, beto.ID.Hex(), "jti-beto-insignias-contrato", middleware.RoleConsumer)

	status, body := call(t, app, http.MethodGet, "/api/v1/usuarios/yo/insignias", nil, anaToken)
	check(t, "insignias/mias_200", status, body)
	status, body = call(t, app, http.MethodGet, "/api/v1/personas/"+ana.ID.Hex()+"/insignias", nil, betoToken)
	check(t, "insignias/persona_200", status, body)
}

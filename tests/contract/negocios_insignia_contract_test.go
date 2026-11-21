package contract

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
)

// TestContract_InsigniaSello fija la forma del 200 JSON de nv-76.
func TestContract_InsigniaSello(t *testing.T) {
	env := authtest.New(t, mongoClient, redisClient, testutil.Config(config.EnvProd))
	app := env.App()
	duena := env.CreateUser(t, "duena-insignia-contrato@wherever.test")
	b := env.AltaAprobada(t, duena, "Contrato Insignia")
	oid, err := bson.ObjectIDFromHex(b.ID)
	require.NoError(t, err)
	_, err = env.Mongo.DB().Collection(negocios.Collection).UpdateByID(context.Background(), oid, bson.D{{Key: "$set", Value: bson.D{
		{Key: "sello_excelencia", Value: true},
		{Key: "sello", Value: negocios.Sello{Otorgado: true, OtorgadoPor: bson.NewObjectID(), OtorgadoEn: time.Now().UTC(), Motivo: "Cumple la regla del Sello"}},
	}}})
	require.NoError(t, err)
	token := testutil.ValidTokenFor(t, duena.ID.Hex(), "jti-duena-insignia-contrato", middleware.RoleMerchant)

	status, body := call(t, app, http.MethodGet, "/api/v1/negocios/"+b.ID+"/sello/insignia", nil, token)
	check(t, "negocios/insignia_200", status, body)
}

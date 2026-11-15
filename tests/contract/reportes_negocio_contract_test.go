package contract

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
)

// TestContract_ReportesNegocio fija las formas de nv-67: 201, la cola y el PATCH.
func TestContract_ReportesNegocio(t *testing.T) {
	env := authtest.New(t, mongoClient, redisClient, testutil.Config(config.EnvProd))
	app := env.App()
	place := negocios.Business{ID: bson.NewObjectID(), Slug: "rep-contrato", QRCheckin: "qr-rep-contrato", Name: "Reporte contrato", Category: "restaurante",
		State: negocios.StatusApproved, Location: negocios.Location{Type: "Point", Coordinates: []float64{-99.13, 19.43}}, CreatedAt: time.Now()}
	_, err := env.Mongo.DB().Collection(negocios.Collection).InsertOne(context.Background(), place)
	require.NoError(t, err)
	user := env.CreateUser(t, "reporta-contrato@wherever.test")
	token := testutil.ValidTokenFor(t, user.ID.Hex(), "jti-reporta-contrato", middleware.RoleConsumer)
	admin := env.CreateAdmin(t, "admin-reportes-contrato@wherever.test")
	adminToken := testutil.ValidTokenFor(t, admin.ID.Hex(), "jti-admin-reportes-contrato", middleware.RoleAdmin)

	status, body := call(t, app, http.MethodPost, "/api/v1/lugares/"+place.ID.Hex()+"/reportes", map[string]string{"motivo": "horario", "detalle": "Cierran antes"}, token)
	check(t, "reportes_negocio/crear_201", status, body)
	id, _ := body.(map[string]any)["id"].(string)
	require.NotEmpty(t, id)
	status, body = call(t, app, http.MethodGet, "/api/v1/admin/reportes-negocio", nil, adminToken)
	check(t, "reportes_negocio/cola_200", status, body)
	status, body = callHeaders(t, app, http.MethodPatch, "/api/v1/admin/reportes-negocio/"+id, map[string]string{"estado": "descartado"}, adminToken, map[string]string{fiber.HeaderIfMatch: `"1"`})
	check(t, "reportes_negocio/resolver_200", status, body)
}

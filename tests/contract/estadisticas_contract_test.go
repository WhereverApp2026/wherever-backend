package contract

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/estadisticas"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/estadisticastest"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/reservastest"
)

// TestContract_Estadisticas fija el 200 de las estadísticas de un negocio premium con datos en
// los dos periodos (nv-74) y las cabeceras del reporte en PDF.
func TestContract_Estadisticas(t *testing.T) {
	env := authtest.New(t, mongoClient, redisClient, testutil.Config(config.EnvProd))
	app := env.App()
	ctx := context.Background()
	duena := env.CreateUser(t, "duena-stats-contrato@wherever.test")
	b := reservastest.Negocio(t, env.Mongo.DB(), duena.ID, reservastest.Config{})
	_, err := env.Mongo.DB().Collection(negocios.Collection).UpdateOne(ctx, bson.D{{Key: "_id", Value: b.ID}}, bson.D{{Key: "$set", Value: bson.D{{Key: "plan", Value: negocios.PlanPremium}}}})
	require.NoError(t, err)
	ayer := time.Now().AddDate(0, 0, -1)
	cal := 4.5
	for i := range 14 {
		estadisticastest.Sembrar(t, env.Mongo.DB(), b.ID, estadisticastest.Dia(ayer.AddDate(0, 0, -i)), estadisticas.Diaria{
			Categoria: b.Category, VistasFicha: 10 + i, Reservas: 2, Llegadas: 1, CuponesCanjeados: 1, FavoritosNuevos: 1,
			Origen: estadisticas.Origen{Busqueda: 5, Mapa: 5}, ResenasDia: 2, CalificacionDia: &cal,
			AspectosSuma: estadisticas.Aspectos{Sabor: 9, Atencion: 8, Ambiente: 8, Espera: 7, PrecioCalidad: 9},
		})
	}
	_, err = env.Mongo.DB().Collection(estadisticas.CategoriaCollection).InsertOne(ctx, estadisticas.Categoria{ID: bson.NewObjectID(), Categoria: b.Category,
		Mes: estadisticastest.Ayer()[:7], Aspectos: estadisticas.Aspectos{Sabor: 4, Atencion: 4, Ambiente: 4, Espera: 4, PrecioCalidad: 4}, Calificacion: 4, Resenas: 50, Negocios: 6})
	require.NoError(t, err)

	token := testutil.ValidTokenFor(t, duena.ID.Hex(), "jti-duena-stats-contrato", middleware.RoleConsumer, middleware.RoleMerchant)
	ruta := "/api/v1/negocios/" + b.ID.Hex()
	status, body := call(t, app, http.MethodGet, ruta+"/estadisticas?desde="+estadisticastest.Dia(ayer.AddDate(0, 0, -6))+"&hasta="+estadisticastest.Ayer(), nil, token)
	check(t, "estadisticas/estadisticas_200", status, body)

	mes := estadisticastest.Dia(time.Now())[:7] // el negocio se dio de alta este mes
	req := httptest.NewRequest(http.MethodGet, ruta+"/reportes/mensual?mes="+mes, nil)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer "+token)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/pdf", resp.Header.Get(fiber.HeaderContentType))
	assert.Equal(t, `attachment; filename="wherever-`+mes+`.pdf"`, resp.Header.Get(fiber.HeaderContentDisposition))
	assert.Equal(t, "no-store", resp.Header.Get(fiber.HeaderCacheControl))
}

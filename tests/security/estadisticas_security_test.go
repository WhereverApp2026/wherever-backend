package security

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/estadisticas"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/reservas"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/estadisticastest"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/reservastest"
)

// Las estadísticas y el reporte (nv-74) solo los ve el dueño comerciante: sin token 401,
// consumidor 403 y dueño ajeno 404. Ni la respuesta ni el PDF traen datos de clientes, y
// ninguna colección de estadísticas guarda `usuario_id`.
func TestEstadisticasSecurity_SoloDuenoYSinDatosDeClientes(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	ctx := context.Background()
	duena := env.CreateUser(t, "duena-sec-stats@wherever.test")
	b := reservastest.Negocio(t, env.Mongo.DB(), duena.ID, reservastest.Config{})
	_, err := env.Mongo.DB().Collection(negocios.Collection).UpdateOne(ctx, bson.D{{Key: "_id", Value: b.ID}}, bson.D{{Key: "$set", Value: bson.D{{Key: "plan", Value: negocios.PlanPremium}}}})
	require.NoError(t, err)
	cliente := env.CreateUser(t, "cliente-sec-stats@wherever.test")
	now := time.Now().UTC()
	r := reservas.Reserva{ID: bson.NewObjectID(), Codigo: "SECSTATS", UsuarioID: cliente.ID, NegocioID: b.ID, FechaHora: now.Add(48 * time.Hour), Personas: 2,
		Nota: "alergia al cacahuate", Estado: reservas.EstadoConfirmada, Historial: []reservas.Cambio{}, Version: 1, CreadoEn: now, ActualizadoEn: now}
	require.NoError(t, reservas.NewRepository(env.Mongo.DB()).Insert(ctx, &r))
	_, err = env.Estadisticas.Recalcular(ctx)
	require.NoError(t, err)

	hoy := estadisticastest.Dia(now)
	rutas := []string{
		"/api/v1/negocios/" + b.ID.Hex() + "/estadisticas?desde=" + hoy + "&hasta=" + hoy,
		"/api/v1/negocios/" + b.ID.Hex() + "/reportes/mensual?mes=" + hoy[:7],
	}
	tokenDuena := testutil.ValidTokenFor(t, duena.ID.Hex(), "jti-duena-sec-stats", middleware.RoleConsumer, middleware.RoleMerchant)
	tokenCliente := testutil.ValidTokenFor(t, cliente.ID.Hex(), "jti-cliente-sec-stats", middleware.RoleConsumer)
	ajeno := env.CreateUser(t, "ajeno-sec-stats@wherever.test")
	tokenAjeno := testutil.ValidTokenFor(t, ajeno.ID.Hex(), "jti-ajeno-sec-stats", middleware.RoleConsumer, middleware.RoleMerchant)
	for _, ruta := range rutas {
		assert.Equal(t, http.StatusUnauthorized, send(t, app, http.MethodGet, ruta, nil, "").status, ruta)
		assert.Equal(t, http.StatusForbidden, send(t, app, http.MethodGet, ruta, nil, tokenCliente).status, ruta)
		assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodGet, ruta, nil, tokenAjeno).status, ruta)
		got := send(t, app, http.MethodGet, ruta, nil, tokenDuena)
		require.Equal(t, http.StatusOK, got.status, got.body)
		for _, dato := range []string{cliente.ID.Hex(), "cliente-sec-stats", "cacahuate", "SECSTATS"} {
			assert.NotContains(t, got.body, dato, ruta)
		}
		assert.NotContains(t, estadisticastest.Texto([]byte(got.body)), "cliente-sec-stats")
	}
	assert.Contains(t, send(t, app, http.MethodGet, rutas[0], nil, tokenDuena).body, `"reservas":1`)

	for _, col := range []string{estadisticas.Collection, estadisticas.CategoriaCollection} {
		n, err := env.Mongo.DB().Collection(col).CountDocuments(ctx, bson.D{{Key: "usuario_id", Value: bson.D{{Key: "$exists", Value: true}}}})
		require.NoError(t, err)
		assert.Zero(t, n, col)
	}
	assert.NotContains(t, env.Logs.String(), "cacahuate", "los logs no llevan datos de clientes")
}

package contract

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/pagos"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/pagostest"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/reservastest"
)

// TestContract_PagosCobros fija el 200 de la lista de cobros del negocio (nv-93) con un pago
// liquidado, con tarjeta y con un reembolso parcial, y la siguiente página.
func TestContract_PagosCobros(t *testing.T) {
	env := authtest.New(t, mongoClient, redisClient, testutil.Config(config.EnvProd))
	duena := env.CreateUser(t, "duena-cobros-contrato@wherever.test")
	b := pagostest.Negocio(t, env.Mongo.DB(), duena.ID, 100, reservastest.Config{})
	repo := pagos.NewRepository(env.Mongo.DB())
	now := time.Now().UTC().Truncate(time.Millisecond)
	semana := pagos.SemanaInicio(now, time.UTC)
	for i := range 2 {
		creado := now.Add(-time.Duration(i) * time.Hour)
		p := pagos.Pago{
			ID: bson.NewObjectID(), ReservaID: bson.NewObjectID(), UsuarioID: bson.NewObjectID(), NegocioID: b.ID, Concepto: pagos.ConceptoAnticipo,
			PaymentIntentID: "pi_contrato_" + bson.NewObjectID().Hex(), MontoCentavos: 15000, ComisionCentavos: 1500, Moneda: pagos.Moneda,
			Estado: pagos.EstadoReembolsadoParcial, Tarjeta: &pagos.Tarjeta{Marca: "visa", Ultimos4: "4242"}, IdempotencyKey: "contrato:" + bson.NewObjectID().Hex(),
			FechaReserva: creado.Add(24 * time.Hour), LiquidacionSemana: &semana, EventosProcesados: []string{},
			Reembolsos:     []pagos.Reembolso{{RefundID: "re_contrato", MontoCentavos: 5000, Fecha: creado}},
			ConservarHasta: pagos.ConservarHasta(creado), CreadoEn: creado, ActualizadoEn: creado,
		}
		require.NoError(t, repo.Insert(context.Background(), &p))
	}

	token := testutil.ValidTokenFor(t, duena.ID.Hex(), "jti-duena-cobros-contrato", middleware.RoleConsumer, middleware.RoleMerchant)
	status, body := call(t, env.App(), http.MethodGet, "/api/v1/negocios/"+b.ID.Hex()+"/cobros?limite=1", nil, token)
	check(t, "pagos/cobros_200", status, body)
}

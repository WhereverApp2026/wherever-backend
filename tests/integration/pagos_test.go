package integration

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/pagos"
	"github.com/WhereverApp2026/wherever-backend/internal/reservas"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/pagostest"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/reservastest"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/stripetest"
)

func pagar(t *testing.T, env *authtest.Env, token, reserva, llave string, body any) *http.Response {
	t.Helper()
	return sendWithHeaders(t, env.App(), http.MethodPost, "/api/v1/reservas/"+reserva+"/pago", body, token, map[string]string{"Idempotency-Key": llave})
}

func pagoDe(t *testing.T, env *authtest.Env, id string) pagos.Pago {
	t.Helper()
	oid, err := bson.ObjectIDFromHex(id)
	require.NoError(t, err)
	var p pagos.Pago
	require.NoError(t, env.Mongo.DB().Collection(pagos.Collection).FindOne(context.Background(), bson.D{{Key: "_id", Value: oid}}).Decode(&p))
	return p
}

func reservaDe(t *testing.T, env *authtest.Env, id string) reservas.Reserva {
	t.Helper()
	oid, err := bson.ObjectIDFromHex(id)
	require.NoError(t, err)
	r, err := reservas.NewRepository(env.Mongo.DB()).FindByID(context.Background(), oid)
	require.NoError(t, err)
	return r
}

// cobrado aparta, crea el cobro y devuelve la reserva y el pago.
func cobrado(t *testing.T, env *authtest.Env, b negocios.Business, email string, hora time.Time) (reservas.ReservaResponse, string, string) {
	t.Helper()
	u, token := cliente(t, env, email)
	r := pagostest.Apartar(t, env, u.ID, b, hora, 2)
	resp := pagar(t, env, token, r.ID, llave(int(hora.Unix()%100000)), nil)
	requireStatus(t, resp, http.StatusOK)
	return r, str(t, decode(t, resp)["pago_id"]), token
}

func aprobar(t *testing.T, env *authtest.Env, pagoID, evento string) int {
	t.Helper()
	p := pagoDe(t, env, pagoID)
	env.Stripe.SetStatus(p.PaymentIntentID, "succeeded")
	payload, firma := pagostest.Evento(evento, pagos.EventoAprobado, stripetest.Succeeded(p.PaymentIntentID))
	return pagostest.Webhook(t, env.App(), payload, firma)
}

func TestPagos_CobroWebhookYReembolso(t *testing.T) {
	env := authEnv(t)
	duena := env.CreateUser(t, "duena-pago@wherever.test")
	b := pagostest.Negocio(t, env.Mongo.DB(), duena.ID, 100, reservastest.Config{})
	u, token := cliente(t, env, "cliente-pago@wherever.test")
	r := pagostest.Apartar(t, env, u.ID, b, reservastest.Manana(14), 2)

	// Un cuerpo con datos de tarjeta no entra (400) y no crea nada.
	resp := pagar(t, env, token, r.ID, llave(601), map[string]any{"numero": "4242424242424242", "cvc": "123"})
	requireStatus(t, resp, http.StatusBadRequest)
	resp = sendWithHeaders(t, env.App(), http.MethodPost, "/api/v1/reservas/"+r.ID+"/pago", nil, token, nil)
	requireStatus(t, resp, http.StatusBadRequest)

	resp = pagar(t, env, token, r.ID, llave(602), nil)
	requireStatus(t, resp, http.StatusOK)
	assert.Equal(t, "no-store", resp.Header.Get("Cache-Control"))
	body := decode(t, resp)
	assertSnakeCase(t, body)
	pagoID := str(t, body["pago_id"])
	assert.Equal(t, float64(10000), body["monto_centavos"])
	assert.Equal(t, "mxn", body["moneda"])
	assert.Contains(t, body["client_secret"], "_secret_")

	// La misma llave y otra llave reutilizan el mismo PaymentIntent: una sola creación en Stripe.
	for _, k := range []string{llave(602), llave(603)} {
		again := pagar(t, env, token, r.ID, k, nil)
		requireStatus(t, again, http.StatusOK)
		assert.Equal(t, pagoID, decode(t, again)["pago_id"])
	}
	creados := env.Stripe.Calls(http.MethodPost, "/v1/payment_intents")
	require.Len(t, creados, 1)
	assert.Equal(t, "reserva:"+r.ID+":intento:1", creados[0].IdempotencyKey)
	assert.Equal(t, "10000", creados[0].Form.Get("amount"), "el monto sale de la reserva")

	// Webhook aprobado: pago con marca y últimos 4, reserva confirmada.
	require.Equal(t, http.StatusOK, aprobar(t, env, pagoID, "evt_aprobado_1"))
	p := pagoDe(t, env, pagoID)
	assert.Equal(t, pagos.EstadoAprobado, p.Estado)
	require.NotNil(t, p.Tarjeta)
	assert.Equal(t, pagos.Tarjeta{Marca: "visa", Ultimos4: "4242"}, *p.Tarjeta)
	assert.Equal(t, int64(1000), p.ComisionCentavos, "comisión inicial de 10 %")
	assert.True(t, p.ConservarHasta.After(time.Now().AddDate(4, 11, 0)))
	assert.Equal(t, reservas.EstadoConfirmada, reservaDe(t, env, r.ID).Estado)

	// El documento solo guarda la marca y los últimos 4.
	var raw bson.M
	require.NoError(t, env.Mongo.DB().Collection(pagos.Collection).FindOne(context.Background(), bson.D{{Key: "_id", Value: p.ID}}).Decode(&raw))
	assert.Len(t, raw["tarjeta"], 2)
	assert.NotContains(t, raw, "client_secret")

	// El mismo evento otra vez: 200 y nada cambia.
	lecturas := len(env.Stripe.Calls(http.MethodGet, "/v1/payment_intents/"))
	require.Equal(t, http.StatusOK, aprobar(t, env, pagoID, "evt_aprobado_1"))
	assert.Len(t, env.Stripe.Calls(http.MethodGet, "/v1/payment_intents/"), lecturas)

	// Estado al volver de 3D Secure.
	resp = sendJSON(t, env.App(), http.MethodGet, "/api/v1/pagos/"+pagoID+"/estado", nil, token)
	requireStatus(t, resp, http.StatusOK)
	assert.Equal(t, map[string]any{"estado": pagos.EstadoAprobado, "reserva_estado": reservas.EstadoConfirmada}, decode(t, resp))

	// Ya pagada: 409.
	requireStatus(t, pagar(t, env, token, r.ID, llave(604), nil), http.StatusConflict)

	// Cancelar con política flexible a más de 2 h: reembolso del 100 % pedido a Stripe.
	resp = sendJSON(t, env.App(), http.MethodPost, "/api/v1/reservas/"+r.ID+"/cancelar", map[string]any{}, token)
	requireStatus(t, resp, http.StatusOK)
	refunds := env.Stripe.Calls(http.MethodPost, "/v1/refunds")
	require.Len(t, refunds, 1)
	assert.Equal(t, "reembolso:"+r.ID, refunds[0].IdempotencyKey)
	assert.Equal(t, "10000", refunds[0].Form.Get("amount"))
	p = pagoDe(t, env, pagoID)
	require.Len(t, p.Reembolsos, 1)

	// charge.refunded concilia y deja el pago reembolsado.
	payload, firma := pagostest.Evento("evt_reembolso_1", pagos.EventoReembolsado,
		stripetest.Refunded(p.PaymentIntentID, 10000, stripetest.Refund{ID: p.Reembolsos[0].RefundID, Amount: 10000}))
	require.Equal(t, http.StatusOK, pagostest.Webhook(t, env.App(), payload, firma))
	assert.Equal(t, pagos.EstadoReembolsado, pagoDe(t, env, pagoID).Estado)

	// Un tipo que no se usa: 200 sin hacer nada.
	payload, firma = pagostest.Evento("evt_otro", "customer.created", map[string]any{"id": "cus_1"})
	assert.Equal(t, http.StatusOK, pagostest.Webhook(t, env.App(), payload, firma))
}

func TestPagos_RechazadoYReintento(t *testing.T) {
	env := authEnv(t)
	duena := env.CreateUser(t, "duena-rechazo@wherever.test")
	b := pagostest.Negocio(t, env.Mongo.DB(), duena.ID, 50, reservastest.Config{})
	r, pagoID, token := cobrado(t, env, b, "cliente-rechazo@wherever.test", reservastest.Manana(15))
	p := pagoDe(t, env, pagoID)

	payload, firma := pagostest.Evento("evt_rechazo_1", pagos.EventoRechazado, stripetest.Failed(p.PaymentIntentID, "Tu tarjeta fue rechazada."))
	require.Equal(t, http.StatusOK, pagostest.Webhook(t, env.App(), payload, firma))
	p = pagoDe(t, env, pagoID)
	assert.Equal(t, pagos.EstadoRechazado, p.Estado)
	assert.Equal(t, "Tu tarjeta fue rechazada.", p.MotivoRechazo)
	assert.Equal(t, reservas.EstadoBloqueada, reservaDe(t, env, r.ID).Estado, "sigue apartada para reintentar")

	// El reintento usa el mismo PaymentIntent.
	resp := pagar(t, env, token, r.ID, llave(611), nil)
	requireStatus(t, resp, http.StatusOK)
	assert.Equal(t, pagoID, decode(t, resp)["pago_id"])
	require.Equal(t, http.StatusOK, aprobar(t, env, pagoID, "evt_rechazo_2"))
	assert.Equal(t, reservas.EstadoConfirmada, reservaDe(t, env, r.ID).Estado)
}

func TestPagos_ApartadoVencidoYPagoTardio(t *testing.T) {
	env := authEnv(t)
	col := env.Mongo.DB().Collection(reservas.Collection)
	expirar := func(id string) {
		oid, err := bson.ObjectIDFromHex(id)
		require.NoError(t, err)
		_, err = col.UpdateOne(context.Background(), bson.D{{Key: "_id", Value: oid}},
			bson.D{{Key: "$set", Value: bson.D{{Key: "estado", Value: reservas.EstadoExpirada}, {Key: "bloqueo_expira_en", Value: time.Now().Add(-time.Minute)}}}})
		require.NoError(t, err)
	}
	duena := env.CreateUser(t, "duena-tarde@wherever.test")
	b := pagostest.Negocio(t, env.Mongo.DB(), duena.ID, 100, reservastest.Config{Capacidad: 2})

	// Apartado vencido: 410.
	u, token := cliente(t, env, "vencido-tarde@wherever.test")
	vencida := pagostest.Apartar(t, env, u.ID, b, reservastest.Manana(10), 2)
	expirar(vencida.ID)
	requireStatus(t, pagar(t, env, token, vencida.ID, llave(621), nil), http.StatusGone)

	// Pago tardío con el cupo libre: se confirma.
	r, pagoID, _ := cobrado(t, env, b, "con-cupo-tarde@wherever.test", reservastest.Manana(11))
	expirar(r.ID)
	require.Equal(t, http.StatusOK, aprobar(t, env, pagoID, "evt_tarde_1"))
	assert.Equal(t, reservas.EstadoConfirmada, reservaDe(t, env, r.ID).Estado)

	// Pago tardío sin cupo: reserva cancelada con cupo_perdido y reembolso completo.
	r2, pago2, _ := cobrado(t, env, b, "sin-cupo-tarde@wherever.test", reservastest.Manana(12))
	expirar(r2.ID)
	otro, _ := cliente(t, env, "gana-cupo@wherever.test")
	pagostest.Apartar(t, env, otro.ID, b, reservastest.Manana(12), 2)
	antes := len(env.Stripe.Calls(http.MethodPost, "/v1/refunds"))
	require.Equal(t, http.StatusOK, aprobar(t, env, pago2, "evt_tarde_2"))
	perdida := reservaDe(t, env, r2.ID)
	assert.Equal(t, reservas.EstadoCancelada, perdida.Estado)
	refunds := env.Stripe.Calls(http.MethodPost, "/v1/refunds")
	require.Len(t, refunds, antes+1)
	assert.Equal(t, "10000", refunds[len(refunds)-1].Form.Get("amount"))
}

func TestPagos_Webhook_ConcurrenteUnaVez(t *testing.T) {
	env := authEnv(t)
	duena := env.CreateUser(t, "duena-concurrente@wherever.test")
	b := pagostest.Negocio(t, env.Mongo.DB(), duena.ID, 80, reservastest.Config{})
	r, pagoID, _ := cobrado(t, env, b, "cliente-concurrente@wherever.test", reservastest.Manana(16))
	p := pagoDe(t, env, pagoID)
	env.Stripe.SetStatus(p.PaymentIntentID, "succeeded")
	payload, firma := pagostest.Evento("evt_concurrente", pagos.EventoAprobado, stripetest.Succeeded(p.PaymentIntentID))
	app := env.App()

	var wg sync.WaitGroup
	codes := make(chan int, 10)
	for range 10 {
		wg.Go(func() { codes <- pagostest.Webhook(t, app, payload, firma) })
	}
	wg.Wait()
	close(codes)
	for c := range codes {
		assert.Equal(t, http.StatusOK, c)
	}
	confirmaciones := 0
	for _, c := range reservaDe(t, env, r.ID).Historial {
		if c.Estado == reservas.EstadoConfirmada {
			confirmaciones++
		}
	}
	assert.Equal(t, 1, confirmaciones)
	assert.Equal(t, []string{"evt_concurrente"}, pagoDe(t, env, pagoID).EventosProcesados)
}

func TestPagos_ConnectSaldoYCorteSemanal(t *testing.T) {
	env := authEnv(t)
	duena := env.CreateUser(t, "duena-connect@wherever.test")
	token := merchantToken(t, duena.ID, "jti-duena-connect")
	b := reservastest.Negocio(t, env.Mongo.DB(), duena.ID, reservastest.Config{})

	// Conectar crea la cuenta Express una vez y da el enlace de registro.
	for range 2 {
		resp := sendJSON(t, env.App(), http.MethodPost, "/api/v1/negocios/"+b.ID.Hex()+"/cobros/conectar", nil, token)
		requireStatus(t, resp, http.StatusOK)
		assert.Contains(t, decode(t, resp)["url"], "https://connect.stripe.com/")
	}
	cuentas := env.Stripe.Calls(http.MethodPost, "/v1/accounts")
	require.Len(t, cuentas, 1)
	assert.Equal(t, "express", cuentas[0].Form.Get("type"))
	assert.Equal(t, "MX", cuentas[0].Form.Get("country"))
	got, err := negocios.NewRepository(env.Mongo.DB()).FindByID(context.Background(), b.ID)
	require.NoError(t, err)
	require.NotNil(t, got.Cobros)
	assert.Equal(t, negocios.CobrosPendiente, got.Cobros.Estado)
	assert.Equal(t, pagos.ComisionPctInicial, got.Cobros.ComisionPct)

	// account.updated la activa.
	payload, firma := pagostest.Evento("evt_cuenta_1", pagos.EventoCuenta, stripetest.AccountUpdated(got.Cobros.StripeAccountID, true))
	require.Equal(t, http.StatusOK, pagostest.Webhook(t, env.App(), payload, firma))
	got, err = negocios.NewRepository(env.Mongo.DB()).FindByID(context.Background(), b.ID)
	require.NoError(t, err)
	assert.Equal(t, negocios.CobrosActivo, got.Cobros.Estado)

	// La CLABE se registra como cuenta externa (sin guardarla completa).
	externa, err := env.Pagos.RegistrarCuenta(context.Background(), got, "002010077777777771", "Fonda SA")
	require.NoError(t, err)
	assert.NotEmpty(t, externa)

	// Dos pagos aprobados de la semana pasada (uno con la mitad reembolsada) y el corte.
	loc := negocios.MexicoCity()
	semana := pagos.SemanaInicio(time.Now(), loc).In(loc).AddDate(0, 0, -7).UTC()
	repo := pagos.NewRepository(env.Mongo.DB())
	now := time.Now().UTC()
	for i, reembolso := range []int64{0, 5000} {
		p := pagos.Pago{
			ID: bson.NewObjectID(), ReservaID: bson.NewObjectID(), UsuarioID: bson.NewObjectID(), NegocioID: b.ID, Concepto: pagos.ConceptoAnticipo,
			PaymentIntentID: "pi_corte_" + bson.NewObjectID().Hex(), MontoCentavos: 10000, ComisionCentavos: 1000, Moneda: pagos.Moneda,
			Estado: pagos.EstadoAprobado, IdempotencyKey: "corte:" + bson.NewObjectID().Hex(), FechaReserva: semana.Add(time.Duration(i+1) * 24 * time.Hour),
			Reembolsos: []pagos.Reembolso{}, EventosProcesados: []string{}, ConservarHasta: pagos.ConservarHasta(now), CreadoEn: now, ActualizadoEn: now,
		}
		if reembolso > 0 {
			p.Estado = pagos.EstadoReembolsadoParcial
			p.Reembolsos = []pagos.Reembolso{{RefundID: "re_corte", MontoCentavos: reembolso, Fecha: now}}
		}
		require.NoError(t, repo.Insert(context.Background(), &p))
	}

	resp := sendJSON(t, env.App(), http.MethodGet, "/api/v1/negocios/"+b.ID.Hex()+"/saldo", nil, token)
	requireStatus(t, resp, http.StatusOK)
	saldo := decode(t, resp)
	assert.Equal(t, float64(9000+4500), saldo["en_espera"])

	for range 2 {
		_, err := env.Pagos.Cortar(context.Background(), semana)
		require.NoError(t, err)
	}
	transfers := env.Stripe.Calls(http.MethodPost, "/v1/transfers")
	require.Len(t, transfers, 1, "el corte repetido no transfiere dos veces")
	assert.Equal(t, "13500", transfers[0].Form.Get("amount"))
	assert.Equal(t, "liquidacion:"+b.ID.Hex()+":"+semana.In(loc).Format("2006-01-02"), transfers[0].IdempotencyKey)

	resp = sendJSON(t, env.App(), http.MethodGet, "/api/v1/negocios/"+b.ID.Hex()+"/liquidaciones", nil, token)
	requireStatus(t, resp, http.StatusOK)
	items := list(t, decode(t, resp)["items"])
	require.Len(t, items, 1)
	l := obj(t, items[0])
	assertSnakeCase(t, l)
	assert.Equal(t, pagos.LiquidacionTransferida, l["estado"])
	assert.Equal(t, float64(20000), l["total_anticipos"])
	assert.Equal(t, float64(5000), l["reembolsos"])
	assert.Equal(t, float64(1500), l["comision"])
	assert.Equal(t, float64(13500), l["neto"])
}

func TestPagos_CobroDeLaReservaParaElNegocio(t *testing.T) {
	env := authEnv(t)
	duena := env.CreateUser(t, "duena-cobro@wherever.test")
	b := pagostest.Negocio(t, env.Mongo.DB(), duena.ID, 100, reservastest.Config{})
	r, pagoID, _ := cobrado(t, env, b, "cliente-cobro@wherever.test", reservastest.Manana(17))
	require.Equal(t, http.StatusOK, aprobar(t, env, pagoID, "evt_cobro_1"))

	token := merchantToken(t, duena.ID, "jti-duena-cobro")
	resp := sendJSON(t, env.App(), http.MethodGet, "/api/v1/negocios/"+b.ID.Hex()+"/reservas/"+r.ID+"/cobro", nil, token)
	requireStatus(t, resp, http.StatusOK)
	body := decode(t, resp)
	assert.Equal(t, float64(10000), body["anticipo"])
	assert.Equal(t, float64(1000), body["comision"])
	assert.Equal(t, float64(9000), body["neto"])
}

// Sin llave de Stripe la API arranca y las rutas de pago responden 503.
func TestPagos_SinLlaveEs503(t *testing.T) {
	env := authEnv(t)
	svc := pagos.NewService(pagos.NewRepository(env.Mongo.DB()), nil, pagos.NewRedis(env.Redis), nil, negocios.NewRepository(env.Mongo.DB()), nil, pagos.Config{}, nil)
	_, err := svc.Cobrar(context.Background(), authtest.ClaimsDe(bson.NewObjectID().Hex(), middleware.RoleConsumer), bson.NewObjectID().Hex(), llave(631))
	assertKind(t, err, apperr.KindUnavailable)
	assertKind(t, svc.Webhook(context.Background(), []byte("{}"), "t=1,v1=00"), apperr.KindUnavailable)
	ok, err := svc.Habilitado(context.Background(), negocios.Business{Cobros: &negocios.Cobros{Estado: negocios.CobrosActivo, StripeAccountID: "acct_1"}})
	require.NoError(t, err)
	assert.False(t, ok, "sin llave no se cobra el anticipo")
}

func assertKind(t *testing.T, err error, kind apperr.Kind) {
	t.Helper()
	var ae *apperr.Error
	require.True(t, errors.As(err, &ae), "%v", err)
	assert.Equal(t, kind, ae.Kind)
}

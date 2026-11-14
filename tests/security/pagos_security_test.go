package security

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/pagos"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/pagostest"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/reservastest"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/stripetest"
)

func rutasPagos(negocio, id string) []rutaReserva {
	return []rutaReserva{
		{http.MethodPost, "/api/v1/reservas/" + id + "/pago", nil, false},
		{http.MethodGet, "/api/v1/pagos/" + id + "/estado", nil, false},
		{http.MethodGet, "/api/v1/negocios/" + negocio + "/reservas/" + id + "/cobro", nil, true},
		{http.MethodPost, "/api/v1/negocios/" + negocio + "/cobros/conectar", nil, true},
		{http.MethodGet, "/api/v1/negocios/" + negocio + "/saldo", nil, true},
		{http.MethodGet, "/api/v1/negocios/" + negocio + "/liquidaciones", nil, true},
	}
}

// Firma ausente, mala, de otro secreto o vieja (> 5 min) = 400 sin detalle y sin aplicar nada.
func TestPagosSecurity_WebhookFirma(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	payload := stripetest.Event("evt_firma", pagos.EventoCuenta, stripetest.AccountUpdated("acct_x", true))

	for name, firma := range map[string]string{
		"sin firma":     "",
		"mala":          "t=1700000000,v1=deadbeef",
		"otro secreto":  stripetest.SignedEvent(payload, "whsec_otro", time.Time{}),
		"vieja":         stripetest.SignedEvent(payload, stripetest.WebhookSecret, time.Now().Add(-6*time.Minute)),
		"sin timestamp": "v1=" + strings.Repeat("0", 64),
	} {
		assert.Equal(t, http.StatusBadRequest, pagostest.Webhook(t, app, payload, firma), name)
	}
	// Con la firma buena entra; un cuerpo alterado después de firmar no.
	firma := stripetest.SignedEvent(payload, stripetest.WebhookSecret, time.Time{})
	assert.Equal(t, http.StatusBadRequest, pagostest.Webhook(t, app, append([]byte(" "), payload...), firma))
	assert.Equal(t, http.StatusOK, pagostest.Webhook(t, app, payload, firma), "sin X-App-Version ni token")
}

// Las rutas con token piden sesión y el rol; la reserva y el negocio ajenos son 404.
func TestPagosSecurity_SesionRolYAjeno(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	duena := env.CreateUser(t, "duena-sec-pago@wherever.test")
	b := pagostest.Negocio(t, env.Mongo.DB(), duena.ID, 100, reservastest.Config{})
	cliente := env.CreateUser(t, "cliente-sec-pago@wherever.test")
	r := pagostest.Apartar(t, env, cliente.ID, b, reservastest.Manana(14), 2)

	consumidor := testutil.ValidTokenFor(t, cliente.ID.Hex(), "jti-cliente-sec-pago", middleware.RoleConsumer)
	for _, ruta := range rutasPagos(b.ID.Hex(), r.ID) {
		assert.Equal(t, http.StatusUnauthorized, send(t, app, ruta.method, ruta.path, ruta.body, "").status, ruta.method+" "+ruta.path)
		if ruta.negocio {
			assert.Equal(t, http.StatusForbidden, send(t, app, ruta.method, ruta.path, ruta.body, consumidor).status, "rol: "+ruta.method+" "+ruta.path)
		}
	}
	soloDueno := testutil.ValidTokenFor(t, duena.ID.Hex(), "jti-duena-sec-pago", middleware.RoleMerchant)
	assert.Equal(t, http.StatusForbidden, sendIdem(t, app, "/api/v1/reservas/"+r.ID+"/pago", soloDueno), "el comerciante no paga")

	ajeno := env.CreateUser(t, "ajeno-sec-pago@wherever.test")
	token := testutil.ValidTokenFor(t, ajeno.ID.Hex(), "jti-ajeno-sec-pago", middleware.RoleConsumer, middleware.RoleMerchant)
	assert.Equal(t, http.StatusNotFound, sendIdem(t, app, "/api/v1/reservas/"+r.ID+"/pago", token))
	for _, ruta := range rutasPagos(b.ID.Hex(), r.ID)[1:] {
		got := send(t, app, ruta.method, ruta.path, ruta.body, token)
		assert.Equal(t, http.StatusNotFound, got.status, ruta.method+" "+ruta.path)
		assert.NotContains(t, got.body, "client_secret")
	}
	assert.Empty(t, env.Stripe.Calls(http.MethodPost, "/v1/payment_intents"), "nadie ajeno llegó a Stripe")
}

// El client_secret solo sale en POST /reservas/{id}/pago: nunca en los logs ni en otra respuesta.
func TestPagosSecurity_ClientSecretSoloEnSuRuta(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	duena := env.CreateUser(t, "duena-secreto@wherever.test")
	b := pagostest.Negocio(t, env.Mongo.DB(), duena.ID, 100, reservastest.Config{})
	cliente := env.CreateUser(t, "cliente-secreto@wherever.test")
	r := pagostest.Apartar(t, env, cliente.ID, b, reservastest.Manana(15), 2)
	token := testutil.ValidTokenFor(t, cliente.ID.Hex(), "jti-cliente-secreto", middleware.RoleConsumer)

	got := sendIdemBody(t, app, "/api/v1/reservas/"+r.ID+"/pago", token)
	require.Equal(t, http.StatusOK, got.status)
	assert.Contains(t, got.body, "_secret_")
	assert.NotContains(t, env.Logs.String(), "_secret_")
	assert.NotContains(t, send(t, app, http.MethodGet, "/api/v1/reservas/"+r.ID, nil, token).body, "_secret_")
}

// Redis caído en el webhook = 503 para que Stripe reintente; no se aplica nada.
func TestPagosSecurity_WebhookRedisCaidoEs503(t *testing.T) {
	env := authEnv(t)
	dead := goredis.NewClient(&goredis.Options{Addr: "127.0.0.1:1", DialTimeout: 200 * time.Millisecond, MaxRetries: -1})
	t.Cleanup(func() { _ = dead.Close() })
	svc := pagos.NewService(pagos.NewRepository(env.Mongo.DB()), pagos.NewClient(stripetest.SecretKey, env.Stripe.URL), pagos.NewRedis(dead), nil,
		negocios.NewRepository(env.Mongo.DB()), nil, pagos.Config{WebhookSecret: stripetest.WebhookSecret}, nil)
	payload, firma := pagostest.Evento("evt_redis", pagos.EventoCuenta, stripetest.AccountUpdated("acct_redis", true))
	err := svc.Webhook(context.Background(), payload, firma)
	var ae *apperr.Error
	require.True(t, errors.As(err, &ae), "%v", err)
	assert.Equal(t, apperr.KindUnavailable, ae.Kind)
	n, err := env.Mongo.DB().Collection(negocios.Collection).CountDocuments(context.Background(), bson.D{{Key: "cobros.stripe_account_id", Value: "acct_redis"}})
	require.NoError(t, err)
	assert.Zero(t, n)
}

func sendIdem(t *testing.T, app *fiber.App, path, token string) int {
	t.Helper()
	return sendIdemBody(t, app, path, token).status
}

// sendIdemBody manda POST sin cuerpo con una Idempotency-Key nueva.
func sendIdemBody(t *testing.T, app *fiber.App, path, token string) reply {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, nil)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer "+token)
	id := bson.NewObjectID().Hex()
	req.Header.Set("Idempotency-Key", id[:8]+"-1b7d-4c2a-9e3f-"+id[12:])
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return reply{status: resp.StatusCode, body: string(raw), headers: resp.Header}
}

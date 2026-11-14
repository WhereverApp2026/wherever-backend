// Package pagostest arma lo que piden las pruebas de pagos (nv-60) sobre la base real: un negocio
// con anticipo y cuenta de Stripe activa, una reserva apartada y la petición firmada del webhook.
package pagostest

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/reservas"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/reservastest"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/stripetest"
)

// Cuenta es la cuenta de Connect de prueba de los negocios con cobros activos.
const Cuenta = "acct_doble_activa"

// Negocio inserta un negocio con anticipo fijo de `pesos` y `cobros` activo.
func Negocio(t testing.TB, db *mongo.Database, owner bson.ObjectID, pesos int, c reservastest.Config) negocios.Business {
	t.Helper()
	c.Anticipo = negocios.Anticipo{Tipo: "fijo", Valor: pesos}
	b := reservastest.Negocio(t, db, owner, c)
	b.Cobros = &negocios.Cobros{StripeAccountID: Cuenta, Estado: negocios.CobrosActivo, ComisionPct: 10}
	_, err := db.Collection(negocios.Collection).UpdateOne(context.Background(), bson.D{{Key: "_id", Value: b.ID}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "cobros", Value: b.Cobros}}}})
	require.NoError(t, err)
	return b
}

// Apartar crea la reserva `bloqueada` del cliente (correo verificado) a esa hora.
func Apartar(t testing.TB, env *authtest.Env, cliente bson.ObjectID, b negocios.Business, hora time.Time, personas int) reservas.ReservaResponse {
	t.Helper()
	require.NoError(t, env.Users.MarkEmailVerified(context.Background(), cliente))
	r, _, err := env.Reservas.Crear(context.Background(), authtest.ClaimsDe(cliente.Hex(), middleware.RoleConsumer),
		bson.NewObjectID().Hex()[:8]+"-1b7d-4c2a-9e3f-"+bson.NewObjectID().Hex()[12:24], reservas.CrearRequest{NegocioID: b.ID.Hex(), FechaHora: hora.Format(time.RFC3339), Personas: personas})
	require.NoError(t, err)
	require.Equal(t, reservas.EstadoBloqueada, r.Estado)
	return r
}

// Webhook manda a la app el evento firmado en `at` (cero = ahora) y devuelve el estado HTTP.
func Webhook(t testing.TB, app *fiber.App, payload []byte, firma string) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, middleware.StripeWebhookPath, bytes.NewReader(payload))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	if firma != "" {
		req.Header.Set("Stripe-Signature", firma)
	}
	resp, err := app.Test(req, int((20 * time.Second).Milliseconds()))
	require.NoError(t, err)
	_ = resp.Body.Close()
	return resp.StatusCode
}

// Evento firma el evento con el secreto del doble, ahora.
func Evento(id, tipo string, object any) ([]byte, string) {
	payload := stripetest.Event(id, tipo, object)
	return payload, stripetest.SignedEvent(payload, stripetest.WebhookSecret, time.Time{})
}

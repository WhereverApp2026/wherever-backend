package security

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/notificaciones"
	"github.com/WhereverApp2026/wherever-backend/internal/reservas"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/reservastest"
)

// El texto del mensaje (nv-73) solo llega al cliente de la reserva: nunca a los logs ni al
// aviso; el consumidor no puede escribir y el detalle de otra persona sigue siendo 404.
func TestReservasMensajesSecurity_TextoSoloAlCliente(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	duena := env.CreateUser(t, "duena-sec-msj@wherever.test")
	b := reservastest.Negocio(t, env.Mongo.DB(), duena.ID, reservastest.Config{})
	tokenNegocio := testutil.ValidTokenFor(t, duena.ID.Hex(), "jti-duena-sec-msj", middleware.RoleConsumer, middleware.RoleMerchant)
	cliente := env.CreateUser(t, "cliente-sec-msj@wherever.test")
	require.NoError(t, env.Users.MarkEmailVerified(context.Background(), cliente.ID))
	r, _, err := env.Reservas.Crear(context.Background(), authtest.ClaimsDe(cliente.ID.Hex(), middleware.RoleConsumer),
		"7c1d3e2f-4a5b-4c6d-9e7f-8091a2b3c4d5", reservas.CrearRequest{NegocioID: b.ID.Hex(), FechaHora: reservastest.Manana(14).Format(time.RFC3339), Personas: 2})
	require.NoError(t, err)
	ruta := "/api/v1/negocios/" + b.ID.Hex() + "/reservas/" + r.ID + "/mensajes"
	const secreto = "Clave del wifi: sol-y-luna-7731"

	// Un consumidor (aunque sea el cliente) no escribe: 403.
	tokenCliente := testutil.ValidTokenFor(t, cliente.ID.Hex(), "jti-cliente-sec-msj", middleware.RoleConsumer)
	assert.Equal(t, http.StatusForbidden, send(t, app, http.MethodPost, ruta, map[string]any{"texto": secreto}, tokenCliente).status)

	got := send(t, app, http.MethodPost, ruta, map[string]any{"texto": secreto}, tokenNegocio)
	require.Equal(t, http.StatusCreated, got.status, got.body)
	assert.NotContains(t, env.Logs.String(), secreto, "el texto no llega a los logs")
	assert.NotContains(t, env.Logs.String(), "sol-y-luna")

	var aviso notificaciones.Notification
	require.NoError(t, env.Mongo.DB().Collection(notificaciones.Collection).FindOne(context.Background(),
		bson.D{{Key: "usuario_id", Value: cliente.ID}, {Key: "tipo", Value: notificaciones.KindReservaMensaje}}).Decode(&aviso))
	assert.ElementsMatch(t, []string{"reserva_id", "negocio_id", "mensaje_id"}, mapKeys(aviso.Data), "el aviso solo lleva ids")
	for _, v := range aviso.Data {
		assert.NotContains(t, v, "sol-y-luna")
	}
	n, err := env.Mongo.DB().Collection(notificaciones.Collection).CountDocuments(context.Background(), bson.D{{Key: "usuario_id", Value: bson.D{{Key: "$ne", Value: cliente.ID}}}})
	require.NoError(t, err)
	assert.Zero(t, n, "nadie más recibe el aviso")

	// Otra persona no ve el detalle (ni los mensajes): 404.
	ajeno := env.CreateUser(t, "ajeno-sec-msj@wherever.test")
	tokenAjeno := testutil.ValidTokenFor(t, ajeno.ID.Hex(), "jti-ajeno-sec-msj", middleware.RoleConsumer)
	detalle := send(t, app, http.MethodGet, "/api/v1/reservas/"+r.ID, nil, tokenAjeno)
	assert.Equal(t, http.StatusNotFound, detalle.status)
	assert.NotContains(t, detalle.body, "sol-y-luna")
}

func mapKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

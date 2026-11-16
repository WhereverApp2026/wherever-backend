package contract

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/reservas"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/reservastest"
)

// TestContract_ReservasMensajes fija el 201 del mensaje del negocio (nv-73) y el detalle de la
// reserva que ve el cliente con `mensajes`.
func TestContract_ReservasMensajes(t *testing.T) {
	env := authtest.New(t, mongoClient, redisClient, testutil.Config(config.EnvProd))
	app := env.App()
	duena := env.CreateUser(t, "duena-mensajes-contrato@wherever.test")
	b := reservastest.Negocio(t, env.Mongo.DB(), duena.ID, reservastest.Config{})
	cliente := env.CreateUser(t, "cliente-mensajes-contrato@wherever.test")
	require.NoError(t, env.Users.MarkEmailVerified(context.Background(), cliente.ID))
	r, _, err := env.Reservas.Crear(context.Background(), authtest.ClaimsDe(cliente.ID.Hex(), middleware.RoleConsumer),
		"8d2e4f3a-5b6c-4d7e-8f90-a1b2c3d4e5f6", reservas.CrearRequest{NegocioID: b.ID.Hex(), FechaHora: reservastest.Manana(14).Format(time.RFC3339), Personas: 2})
	require.NoError(t, err)

	tokenNegocio := testutil.ValidTokenFor(t, duena.ID.Hex(), "jti-duena-mensajes-contrato", middleware.RoleConsumer, middleware.RoleMerchant)
	status, body := call(t, app, http.MethodPost, "/api/v1/negocios/"+b.ID.Hex()+"/reservas/"+r.ID+"/mensajes", map[string]string{"texto": "Tu mesa está lista"}, tokenNegocio)
	check(t, "reservas/mensaje_201", status, body)

	tokenCliente := testutil.ValidTokenFor(t, cliente.ID.Hex(), "jti-cliente-mensajes-contrato", middleware.RoleConsumer)
	status, body = call(t, app, http.MethodGet, "/api/v1/reservas/"+r.ID, nil, tokenCliente)
	check(t, "reservas/detalle_con_mensajes_200", status, body)
}

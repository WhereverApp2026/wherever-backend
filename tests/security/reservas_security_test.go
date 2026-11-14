package security

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/reservas"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/reservastest"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

type rutaReserva struct {
	method, path string
	body         any
	negocio      bool
}

func rutasReservas(negocio, id string) []rutaReserva {
	return []rutaReserva{
		{http.MethodPost, "/api/v1/reservas", map[string]any{"personas": 1}, false},
		{http.MethodGet, "/api/v1/reservas", nil, false},
		{http.MethodGet, "/api/v1/reservas/" + id, nil, false},
		{http.MethodPatch, "/api/v1/reservas/" + id, map[string]any{"personas": 2}, false},
		{http.MethodGet, "/api/v1/reservas/" + id + "/reembolso", nil, false},
		{http.MethodPost, "/api/v1/reservas/" + id + "/cancelar", nil, false},
		{http.MethodGet, "/api/v1/negocios/" + negocio + "/reservas", nil, true},
		{http.MethodPatch, "/api/v1/negocios/" + negocio + "/reservas/" + id, map[string]any{"estado": "asistio"}, true},
		{http.MethodPost, "/api/v1/negocios/" + negocio + "/reservas/escanear", map[string]any{"codigo": "ABC234"}, true},
		{http.MethodPost, "/api/v1/negocios/" + negocio + "/bloqueos", map[string]any{"desde": "2026-12-01T10:00:00Z", "hasta": "2026-12-01T12:00:00Z"}, true},
		{http.MethodDelete, "/api/v1/negocios/" + negocio + "/bloqueos/" + id, nil, true},
	}
}

// Las once rutas con token (todas menos disponibilidad) piden sesión, cuenta activa y el rol.
func TestReservasSecurity_SesionRolYCuentaActiva(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	rutas := rutasReservas(bson.NewObjectID().Hex(), bson.NewObjectID().Hex())

	pending := env.CreateUser(t, "pendiente-res@wherever.test")
	_, err := env.Users.Update(context.Background(), pending.ID, bson.D{{Key: "estado", Value: usuarios.StatusPendingDeletion}})
	require.NoError(t, err)
	pendiente := testutil.ValidTokenFor(t, pending.ID.Hex(), "jti-pendiente-res", middleware.RoleConsumer, middleware.RoleMerchant)
	consumidor := env.CreateUser(t, "consumidor-res@wherever.test")
	soloConsumidor := testutil.ValidTokenFor(t, consumidor.ID.Hex(), "jti-consumidor-res", middleware.RoleConsumer)

	for _, r := range rutas {
		assert.Equal(t, http.StatusUnauthorized, send(t, app, r.method, r.path, r.body, "").status, r.method+" "+r.path)
		assert.Equal(t, http.StatusForbidden, send(t, app, r.method, r.path, r.body, pendiente).status, r.method+" "+r.path)
		if r.negocio {
			assert.Equal(t, http.StatusForbidden, send(t, app, r.method, r.path, r.body, soloConsumidor).status, "rol: "+r.method+" "+r.path)
		}
	}
	// La disponibilidad es pública.
	assert.NotEqual(t, http.StatusUnauthorized, send(t, app, http.MethodGet, "/api/v1/lugares/"+bson.NewObjectID().Hex()+"/disponibilidad?fecha=2026-12-01&personas=2", nil, "").status)
}

// El negocio ajeno y la reserva ajena responden 404 igual que si no existieran.
func TestReservasSecurity_AjenoEs404(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	duena := env.CreateUser(t, "duena-sec-res@wherever.test")
	b := reservastest.Negocio(t, env.Mongo.DB(), duena.ID, reservastest.Config{})
	cliente := env.CreateUser(t, "cliente-sec-res@wherever.test")
	require.NoError(t, env.Users.MarkEmailVerified(context.Background(), cliente.ID))
	r, _, err := env.Reservas.Crear(context.Background(), authtest.ClaimsDe(cliente.ID.Hex(), middleware.RoleConsumer),
		"6b0c2d1e-3f4a-4b5c-8d6e-7f8091a2b3c4", reservas.CrearRequest{NegocioID: b.ID.Hex(), FechaHora: reservastest.Manana(14).Format(time.RFC3339), Personas: 2})
	require.NoError(t, err)

	ajeno := env.CreateUser(t, "ajeno-sec-res@wherever.test")
	token := testutil.ValidTokenFor(t, ajeno.ID.Hex(), "jti-ajeno-res", middleware.RoleConsumer, middleware.RoleMerchant)
	for _, ruta := range rutasReservas(b.ID.Hex(), r.ID)[1:] {
		got := send(t, app, ruta.method, ruta.path, ruta.body, token)
		if ruta.path == "/api/v1/reservas" {
			continue
		}
		if ruta.method == http.MethodPatch && !ruta.negocio {
			assert.Contains(t, []int{http.StatusBadRequest, http.StatusNotFound}, got.status, "sin If-Match responde 400 antes de buscar")
			continue
		}
		assert.Equal(t, http.StatusNotFound, got.status, ruta.method+" "+ruta.path)
		assert.NotContains(t, got.body, cliente.ID.Hex())
	}
}

// Sin Redis no hay candado ni idempotencia: POST /reservas responde 503 y no guarda nada.
func TestReservasSecurity_RedisCaidoEs503(t *testing.T) {
	env := authEnv(t)
	duena := env.CreateUser(t, "redis-duena-res@wherever.test")
	b := reservastest.Negocio(t, env.Mongo.DB(), duena.ID, reservastest.Config{})
	cliente := env.CreateUser(t, "redis-cliente-res@wherever.test")
	require.NoError(t, env.Users.MarkEmailVerified(context.Background(), cliente.ID))

	dead := goredis.NewClient(&goredis.Options{Addr: "127.0.0.1:1", DialTimeout: 200 * time.Millisecond, MaxRetries: -1})
	t.Cleanup(func() { _ = dead.Close() })
	svc := reservas.NewService(reservas.NewRepository(env.Mongo.DB()), negocios.NewRepository(env.Mongo.DB()), env.Users, reservas.NewRedis(dead), nil, nil)
	_, _, err := svc.Crear(context.Background(), authtest.ClaimsDe(cliente.ID.Hex(), middleware.RoleConsumer), "6b0c2d1e-3f4a-4b5c-8d6e-7f8091a2b3c5",
		reservas.CrearRequest{NegocioID: b.ID.Hex(), FechaHora: reservastest.Manana(14).Format(time.RFC3339), Personas: 2})
	var ae *apperr.Error
	require.True(t, errors.As(err, &ae), "%v", err)
	assert.Equal(t, apperr.KindUnavailable, ae.Kind)
	n, err := env.Mongo.DB().Collection(reservas.Collection).CountDocuments(context.Background(), bson.D{})
	require.NoError(t, err)
	assert.Zero(t, n)
}

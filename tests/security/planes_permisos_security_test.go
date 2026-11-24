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
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/reservas"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/reservastest"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// Reservar todo y cambiar el permiso de un integrante (nv-89) piden sesión y cuenta activa;
// reservar todo además el rol de consumidor.
func TestPlanesPermisosSecurity_SesionYCuentaActiva(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	id := bson.NewObjectID().Hex()
	rutas := []struct {
		method, path string
		body         any
	}{
		{http.MethodPost, "/api/v1/planes/" + id + "/reservas", nil},
		{http.MethodPatch, "/api/v1/grupos/" + id + "/integrantes/" + id, map[string]string{"permiso_plan": "editar"}},
	}
	pending := env.CreateUser(t, "pendiente-nv89@wherever.test")
	_, err := env.Users.Update(context.Background(), pending.ID, bson.D{{Key: "estado", Value: usuarios.StatusPendingDeletion}})
	require.NoError(t, err)
	token := testutil.ValidTokenFor(t, pending.ID.Hex(), "jti-pendiente-nv89", middleware.RoleConsumer)
	for _, r := range rutas {
		assert.Equal(t, http.StatusUnauthorized, send(t, app, r.method, r.path, r.body, "").status, r.method+" "+r.path)
		assert.Equal(t, http.StatusForbidden, send(t, app, r.method, r.path, r.body, token).status, r.method+" "+r.path)
	}
}

// Un plan ajeno es 404 al reservar todo, sin crear nada ni decir de quién es.
func TestPlanesPermisosSecurity_PlanAjenoEs404(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	duena := env.CreateUser(t, "duena-nv89@wherever.test")
	b := reservastest.Negocio(t, env.Mongo.DB(), duena.ID, reservastest.Config{})
	owner := env.CreateUser(t, "owner-nv89@wherever.test")
	p := planes.Plan{
		OwnerID: owner.ID, Nombre: "Ajeno", Fecha: reservastest.Manana(0), Personas: 2, Movilidad: planes.MobilityWalk, Origen: planes.OriginManual,
		Paradas: []planes.Parada{{ID: bson.NewObjectID(), NegocioID: b.ID, Llega: reservastest.Manana(13), Minutos: 60}},
	}
	require.NoError(t, planes.NewRepository(env.Mongo.DB()).Create(context.Background(), &p))
	ajeno := env.CreateUser(t, "ajeno-nv89@wherever.test")
	require.NoError(t, env.Users.MarkEmailVerified(context.Background(), ajeno.ID))
	token := testutil.ValidTokenFor(t, ajeno.ID.Hex(), "jti-ajeno-nv89", middleware.RoleConsumer)

	got := sendIdemBody(t, app, "/api/v1/planes/"+p.ID.Hex()+"/reservas", token)
	assert.Equal(t, http.StatusNotFound, got.status, got.body)
	assert.NotContains(t, got.body, owner.ID.Hex())
	n, err := env.Mongo.DB().Collection(reservas.Collection).CountDocuments(context.Background(), bson.D{{Key: "plan_id", Value: p.ID}})
	require.NoError(t, err)
	assert.Zero(t, n)
}

// Sin Redis no hay candado ni idempotencia: reservar todo responde 503 y no guarda nada.
func TestPlanesPermisosSecurity_RedisCaidoEs503(t *testing.T) {
	env := authEnv(t)
	duena := env.CreateUser(t, "redis-duena-nv89@wherever.test")
	b := reservastest.Negocio(t, env.Mongo.DB(), duena.ID, reservastest.Config{})
	owner := env.CreateUser(t, "redis-owner-nv89@wherever.test")
	require.NoError(t, env.Users.MarkEmailVerified(context.Background(), owner.ID))
	p := planes.Plan{
		OwnerID: owner.ID, Nombre: "Sin Redis", Fecha: reservastest.Manana(0), Personas: 2, Movilidad: planes.MobilityWalk, Origen: planes.OriginManual,
		Paradas: []planes.Parada{{ID: bson.NewObjectID(), NegocioID: b.ID, Llega: reservastest.Manana(15), Minutos: 60}},
	}
	require.NoError(t, planes.NewRepository(env.Mongo.DB()).Create(context.Background(), &p))

	dead := goredis.NewClient(&goredis.Options{Addr: "127.0.0.1:1", DialTimeout: 200 * time.Millisecond, MaxRetries: -1})
	t.Cleanup(func() { _ = dead.Close() })
	svc := reservas.NewService(reservas.NewRepository(env.Mongo.DB()), negocios.NewRepository(env.Mongo.DB()), env.Users, reservas.NewRedis(dead), nil, nil).
		WithPlanes(env.Planes)
	_, err := svc.ReservarPlan(context.Background(), authtest.ClaimsDe(owner.ID.Hex(), middleware.RoleConsumer), p.ID.Hex(),
		"6b0c2d1e-3f4a-4b5c-8d6e-7f8091a2b390", reservas.PlanRequest{}, nil)
	var ae *apperr.Error
	require.True(t, errors.As(err, &ae), "%v", err)
	assert.Equal(t, apperr.KindUnavailable, ae.Kind)
	n, err := env.Mongo.DB().Collection(reservas.Collection).CountDocuments(context.Background(), bson.D{{Key: "plan_id", Value: p.ID}})
	require.NoError(t, err)
	assert.Zero(t, n)
}

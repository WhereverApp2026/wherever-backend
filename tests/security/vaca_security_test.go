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
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/WhereverApp2026/wherever-backend/internal/grupos"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
	"github.com/WhereverApp2026/wherever-backend/internal/vaca"
)

// Las siete rutas de la Vaca (nv-58) piden sesión y cuenta activa.
func TestVacaSecurity_RequiereSesionYCuentaActiva(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	id := bson.NewObjectID().Hex()
	routes := []struct {
		method, path string
		body         any
	}{
		{http.MethodPost, "/api/v1/grupos/" + id + "/gastos", map[string]any{"monto_centavos": 100}},
		{http.MethodGet, "/api/v1/grupos/" + id + "/gastos", nil},
		{http.MethodGet, "/api/v1/grupos/" + id + "/vaca", nil},
		{http.MethodPatch, "/api/v1/gastos/" + id, map[string]any{"concepto": "otro"}},
		{http.MethodDelete, "/api/v1/gastos/" + id, nil},
		{http.MethodPatch, "/api/v1/vaca/deudas/" + id, map[string]string{"estado": "pagada"}},
		{http.MethodGet, "/api/v1/vaca/mis-deudas", nil},
	}
	pending := env.CreateUser(t, "pendiente-vaca@wherever.test")
	_, err := env.Users.Update(context.Background(), pending.ID, bson.D{{Key: "estado", Value: usuarios.StatusPendingDeletion}})
	require.NoError(t, err)
	token := testutil.ValidTokenFor(t, pending.ID.Hex(), "jti-pendiente-vaca", middleware.RoleConsumer)

	for _, r := range routes {
		assert.Equal(t, http.StatusUnauthorized, send(t, app, r.method, r.path, r.body, "").status, r.method+" "+r.path)
		assert.Equal(t, http.StatusForbidden, send(t, app, r.method, r.path, r.body, token).status, r.method+" "+r.path)
	}
}

// La Vaca de un grupo ajeno responde 404 igual que un grupo que no existe.
func TestVacaSecurity_GrupoAjenoEs404(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	owner := env.CreateUser(t, "dueno-vaca@wherever.test")
	outsider := env.CreateUser(t, "ajeno-vaca@wherever.test")
	g := vacaGroup(t, env.Mongo.DB(), owner.ID)
	token := testutil.ValidTokenFor(t, outsider.ID.Hex(), "jti-ajeno-vaca", middleware.RoleConsumer)

	missing := send(t, app, http.MethodGet, "/api/v1/grupos/"+bson.NewObjectID().Hex()+"/vaca", nil, token)
	foreign := send(t, app, http.MethodGet, "/api/v1/grupos/"+g.ID.Hex()+"/vaca", nil, token)
	assert.Equal(t, http.StatusNotFound, foreign.status)
	assert.Equal(t, missing.status, foreign.status)
	assert.NotContains(t, foreign.body, owner.ID.Hex())
}

// Sin Redis el candado del recálculo no se toma y la escritura responde 503 (fail-closed).
func TestVacaSecurity_RedisCaidoEnEscrituraEs503(t *testing.T) {
	env := authEnv(t)
	owner := env.CreateUser(t, "redis-vaca@wherever.test")
	friend := env.CreateUser(t, "redis-vaca-2@wherever.test")
	g := vacaGroup(t, env.Mongo.DB(), owner.ID, friend.ID)

	dead := goredis.NewClient(&goredis.Options{Addr: "127.0.0.1:1", DialTimeout: 200 * time.Millisecond, MaxRetries: -1})
	t.Cleanup(func() { _ = dead.Close() })
	svc := vaca.NewService(vaca.NewRepository(env.Mongo.DB()), grupos.NewRepository(env.Mongo.DB()), env.Users, vaca.NewRedisLocker(dead), nil)
	_, _, err := svc.Create(context.Background(), owner.ID.Hex(), g.ID.Hex(), vaca.CreateRequest{
		Cents: 1000, Concept: vaca.ConceptFood, PaidBy: owner.ID.Hex(), Division: vaca.DivisionEqual,
		ClientID: "6b0c2d1e-3f4a-4b5c-8d6e-7f8091a2b3c4",
	})
	var ae *apperr.Error
	require.True(t, errors.As(err, &ae), "%v", err)
	assert.Equal(t, apperr.KindUnavailable, ae.Kind)
	n, err := env.Mongo.DB().Collection(vaca.ExpensesCollection).CountDocuments(context.Background(), bson.D{{Key: "grupo_id", Value: g.ID}})
	require.NoError(t, err)
	assert.Zero(t, n, "sin candado no se guarda el gasto")
}

// vacaGroup siembra un grupo con el organizador y los demás como integrantes confirmados.
func vacaGroup(t *testing.T, db *mongo.Database, org bson.ObjectID, others ...bson.ObjectID) grupos.Group {
	t.Helper()
	now := time.Now().UTC()
	g := grupos.Group{
		ID: bson.NewObjectID(), PlanID: bson.NewObjectID(), OrganizerID: org, Occupied: 1 + len(others), CreatedAt: now, UpdatedAt: now,
		Members: []grupos.Member{{UserID: org, Role: grupos.RoleOrganizer, Status: grupos.StatusConfirmed, Since: now}},
	}
	for _, id := range others {
		g.Members = append(g.Members, grupos.Member{UserID: id, Role: grupos.RoleMember, Status: grupos.StatusConfirmed, Since: now})
	}
	require.NoError(t, grupos.NewRepository(db).Create(context.Background(), &g))
	return g
}

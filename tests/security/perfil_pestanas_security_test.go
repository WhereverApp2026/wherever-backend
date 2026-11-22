package security

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

var pestanasPerfil = []string{"momentos", "planes", "resenas"}

func pestanaPath(id, pestana string) string { return "/api/v1/personas/" + id + "/" + pestana }

// Las tres pestañas (nv-81) piden sesión y cuenta activa; propia, inexistente, inactiva o con
// bloqueo en cualquier sentido dan el mismo 404; un id mal formado, 422.
func TestPerfilPestanasSecurity_SesionY404Uniforme(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	ctx := context.Background()
	db := env.Mongo.DB()
	ana := env.CreateUser(t, "ana-pestanas81@wherever.test")
	bloqueada := env.CreateUser(t, "bloq-pestanas81@wherever.test")
	inactiva := env.CreateUser(t, "inact-pestanas81@wherever.test")
	_, err := env.Users.Update(ctx, inactiva.ID, bson.D{{Key: "estado", Value: usuarios.StatusPendingDeletion}})
	require.NoError(t, err)
	now := time.Now().UTC()
	f := amistades.Friendship{ID: bson.NewObjectID(), From: bloqueada.ID, To: ana.ID, Status: amistades.StatusBlocked, Pair: amistades.PairKey(ana.ID, bloqueada.ID), CreatedAt: now, UpdatedAt: now}
	require.NoError(t, amistades.NewRepository(db).Insert(ctx, &f))
	tok := testutil.ValidTokenFor(t, ana.ID.Hex(), "jti-ana-pestanas81", middleware.RoleConsumer)
	pendiente := testutil.ValidTokenFor(t, inactiva.ID.Hex(), "jti-inact-pestanas81", middleware.RoleConsumer)

	for _, p := range pestanasPerfil {
		otra := pestanaPath(bloqueada.ID.Hex(), p)
		assert.Equal(t, http.StatusUnauthorized, send(t, app, http.MethodGet, otra, nil, "").status, p)
		assert.Equal(t, http.StatusForbidden, send(t, app, http.MethodGet, otra, nil, pendiente).status, p+": cuenta en eliminación")
		for _, id := range []string{ana.ID.Hex(), bloqueada.ID.Hex(), inactiva.ID.Hex(), bson.NewObjectID().Hex()} {
			r := send(t, app, http.MethodGet, pestanaPath(id, p), nil, tok)
			assert.Equal(t, http.StatusNotFound, r.status, p+" "+id)
			assert.Contains(t, r.body, `"mensaje":"persona no encontrada"`, "el mismo 404 no deja saber quién existe")
		}
		assert.Equal(t, http.StatusUnprocessableEntity, send(t, app, http.MethodGet, pestanaPath("no-es-id", p), nil, tok).status, p)
	}
}

// Un plan `privado` nunca sale en la pestaña, aun entre amigos; uno público sale sin dueño,
// integrantes, grupo, notas, correo ni ubicación.
func TestPerfilPestanasSecurity_PlanesSoloPublicosYSinDatosPrivados(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	ctx := context.Background()
	db := env.Mongo.DB()
	ana := env.CreateUser(t, "ana-planes81@wherever.test")
	beto := env.CreateUser(t, "beto-planes81@wherever.test")
	now := time.Now().UTC()
	f := amistades.Friendship{ID: bson.NewObjectID(), From: ana.ID, To: beto.ID, Status: amistades.StatusAccepted, Pair: amistades.PairKey(ana.ID, beto.ID), CreatedAt: now, UpdatedAt: now}
	require.NoError(t, amistades.NewRepository(db).Insert(ctx, &f))
	publico, privado := bson.NewObjectID(), bson.NewObjectID()
	for _, p := range []struct {
		id  bson.ObjectID
		vis string
	}{{publico, planes.VisibilityPublic}, {privado, planes.VisibilityPrivate}} {
		_, err := db.Collection(planes.Collection).InsertOne(ctx, bson.D{
			{Key: "_id", Value: p.id}, {Key: "dueno_id", Value: beto.ID}, {Key: "nombre", Value: "Plan " + p.vis}, {Key: "fecha", Value: now},
			{Key: "estado", Value: planes.StatusActive}, {Key: "visibilidad", Value: p.vis}, {Key: "paradas", Value: bson.A{}},
			{Key: "zona", Value: bson.D{{Key: "type", Value: "Point"}, {Key: "coordinates", Value: bson.A{-99.13, 19.43}}}},
		})
		require.NoError(t, err)
	}
	tok := testutil.ValidTokenFor(t, ana.ID.Hex(), "jti-ana-planes81", middleware.RoleConsumer)
	r := send(t, app, http.MethodGet, pestanaPath(beto.ID.Hex(), "planes"), nil, tok)
	require.Equal(t, http.StatusOK, r.status, r.body)
	assert.Contains(t, r.body, publico.Hex())
	assert.NotContains(t, r.body, privado.Hex(), "un plan privado nunca sale")
	for _, secreto := range []string{beto.ID.Hex(), "dueno", "integrantes", "grupo", "nota", "correo", "zona", "coordinates", "beto-planes81"} {
		assert.NotContains(t, r.body, secreto)
	}
}

// Las tres pestañas cuentan en la política del perfil (120 por hora por usuario, nv-70).
func TestPerfilPestanasSecurity_Limite429(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	ana := env.CreateUser(t, "ana-lim81@wherever.test")
	beto := env.CreateUser(t, "beto-lim81@wherever.test")
	tok := testutil.ValidTokenFor(t, ana.ID.Hex(), "jti-ana-lim81", middleware.RoleConsumer)
	for i := range 120 {
		require.Equal(t, http.StatusOK, send(t, app, http.MethodGet, pestanaPath(beto.ID.Hex(), pestanasPerfil[i%3]), nil, tok).status, i)
	}
	assert.Equal(t, http.StatusTooManyRequests, send(t, app, http.MethodGet, "/api/v1/personas/"+beto.ID.Hex(), nil, tok).status,
		"el perfil comparte la cuota con sus pestañas")
	env.ClearRateLimits(t)
}

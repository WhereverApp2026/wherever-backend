package security

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/recomendaciones"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

func TestRecomendadorSecurity_RequiereSesion(t *testing.T) {
	app := authEnv(t).App()
	id := bson.NewObjectID().Hex()
	for _, r := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/recomendaciones?lat=19.43&lng=-99.13"},
		{http.MethodPost, "/api/v1/recomendaciones/" + id + "/no-me-interesa"},
		{http.MethodDelete, "/api/v1/recomendaciones/" + id + "/no-me-interesa"},
	} {
		assert.Equal(t, http.StatusUnauthorized, send(t, app, r.method, r.path, nil, "").status, r.path)
	}
}

func TestRecomendadorSecurity_NadieTocaLosDescartesDeOtro(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	ctx := context.Background()
	b := negocios.Business{
		ID: bson.NewObjectID(), Slug: "seguridad-descarte", QRCheckin: "qr-seguridad-descarte", Name: "Seguridad",
		Category: "restaurante", State: negocios.StatusApproved,
		Location: negocios.Location{Type: "Point", Coordinates: []float64{-99.13, 19.43}},
	}
	_, err := env.Mongo.DB().Collection(negocios.Collection).InsertOne(ctx, b)
	require.NoError(t, err)

	alice := env.CreateUser(t, "alice-descartes@wherever.test")
	bob := env.CreateUser(t, "bob-descartes@wherever.test")
	aliceToken := testutil.ValidTokenFor(t, alice.ID.Hex(), "jti-alice-descartes", middleware.RoleConsumer)
	bobToken := testutil.ValidTokenFor(t, bob.ID.Hex(), "jti-bob-descartes", middleware.RoleConsumer)
	path := "/api/v1/recomendaciones/" + b.ID.Hex() + "/no-me-interesa"

	require.Equal(t, http.StatusNoContent, send(t, app, http.MethodPost, path, nil, aliceToken).status)
	assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodDelete, path, nil, bobToken).status, "Bob no deshace el descarte de Alice")

	repo := recomendaciones.NewRepository(env.Mongo.DB())
	aliceExcluded, err := repo.Excluded(ctx, alice.ID)
	require.NoError(t, err)
	assert.Equal(t, []bson.ObjectID{b.ID}, aliceExcluded, "el descarte de Alice sigue")
	bobExcluded, err := repo.Excluded(ctx, bob.ID)
	require.NoError(t, err)
	assert.Empty(t, bobExcluded, "Bob no ve ni hereda descartes de Alice")
	assert.Equal(t, http.StatusNoContent, send(t, app, http.MethodDelete, path, nil, aliceToken).status)
}

func TestRecomendadorSecurity_CuentaEnEliminacionEs403(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	u := env.CreateUser(t, "eliminacion-reco@wherever.test")
	_, err := env.Users.Update(context.Background(), u.ID, bson.D{{Key: "estado", Value: usuarios.StatusPendingDeletion}})
	require.NoError(t, err)
	token := testutil.ValidTokenFor(t, u.ID.Hex(), "jti-eliminacion-reco", middleware.RoleConsumer)
	assert.Equal(t, http.StatusForbidden, send(t, app, http.MethodGet, "/api/v1/recomendaciones?lat=19.43&lng=-99.13", nil, token).status)
}

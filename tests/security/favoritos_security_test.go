package security

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// Las cinco rutas de favoritos (nv-54) piden sesión (401), rol consumidor (403 con
// comerciante) y cuenta activa (403 en eliminación pendiente).
func TestFavoritosSecurity_SesionRolYCuentaActiva(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	id := bson.NewObjectID().Hex()
	routes := []struct{ method, path string }{
		{http.MethodPut, "/api/v1/favoritos/negocio/" + id},
		{http.MethodDelete, "/api/v1/favoritos/negocio/" + id},
		{http.MethodGet, "/api/v1/favoritos?tipo=negocio"},
		{http.MethodGet, "/api/v1/favoritos/ids?tipo=plan"},
		{http.MethodGet, "/api/v1/favoritos/buscar?q=cafe"},
	}
	merchant := env.CreateUser(t, "comerciante-fav@wherever.test")
	merchantToken := testutil.ValidTokenFor(t, merchant.ID.Hex(), "jti-comerciante-fav", middleware.RoleMerchant)
	pending := env.CreateUser(t, "pendiente-fav@wherever.test")
	_, err := env.Users.Update(context.Background(), pending.ID, bson.D{{Key: "estado", Value: usuarios.StatusPendingDeletion}})
	require.NoError(t, err)
	pendingToken := testutil.ValidTokenFor(t, pending.ID.Hex(), "jti-pendiente-fav", middleware.RoleConsumer)
	for _, r := range routes {
		assert.Equal(t, http.StatusUnauthorized, send(t, app, r.method, r.path, nil, "").status, r.path)
		assert.Equal(t, http.StatusForbidden, send(t, app, r.method, r.path, nil, merchantToken).status, r.path)
		assert.Equal(t, http.StatusForbidden, send(t, app, r.method, r.path, nil, pendingToken).status, r.path)
	}
}

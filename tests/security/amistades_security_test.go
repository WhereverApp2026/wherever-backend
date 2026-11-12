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

// Las cinco rutas de amistades (nv-36) piden sesión y cuenta activa.
func TestAmistadesSecurity_RequiereSesionYCuentaActiva(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	id := bson.NewObjectID().Hex()
	routes := []struct {
		method, path string
		body         any
	}{
		{http.MethodPost, "/api/v1/amistades", map[string]string{"usuario_id": id}},
		{http.MethodGet, "/api/v1/amistades", nil},
		{http.MethodPatch, "/api/v1/amistades/" + id, map[string]string{"accion": "aceptar"}},
		{http.MethodDelete, "/api/v1/amistades/" + id, nil},
		{http.MethodPost, "/api/v1/amistades/" + id + "/bloquear", nil},
	}

	pending := env.CreateUser(t, "pendiente-amistades@wherever.test")
	_, err := env.Users.Update(context.Background(), pending.ID, bson.D{{Key: "estado", Value: usuarios.StatusPendingDeletion}})
	require.NoError(t, err)
	token := testutil.ValidTokenFor(t, pending.ID.Hex(), "jti-pendiente-amistades", middleware.RoleConsumer)

	for _, r := range routes {
		assert.Equal(t, http.StatusUnauthorized, send(t, app, r.method, r.path, r.body, "").status, r.method+" "+r.path)
		assert.Equal(t, http.StatusForbidden, send(t, app, r.method, r.path, r.body, token).status, r.method+" "+r.path)
	}
}

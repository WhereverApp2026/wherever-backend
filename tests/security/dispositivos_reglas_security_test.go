package security

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// Las rutas por id de nv-71 piden sesión; las reglas, además, rol de administrador.
func TestDispositivosReglasSecurity_SesionRolYCuentaActiva(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	id := bson.NewObjectID().Hex()
	routes := []struct {
		method, path string
		body         any
	}{
		{http.MethodPut, "/api/v1/dispositivos/" + id, map[string]string{"app_version": "1.0.0"}},
		{http.MethodDelete, "/api/v1/dispositivos/" + id, nil},
		{http.MethodPatch, "/api/v1/admin/moderacion/reglas/spam", map[string]any{"activa": true}},
		{http.MethodDelete, "/api/v1/admin/moderacion/reglas/spam", nil},
	}
	for _, r := range routes {
		assert.Equal(t, http.StatusUnauthorized, sendIfMatch(t, app, r.method, r.path, r.body, "").status, r.method+" "+r.path)
	}
	user := env.CreateUser(t, "rol-reglas@wherever.test")
	for _, roles := range [][]string{{middleware.RoleConsumer}, {middleware.RoleConsumer, middleware.RoleMerchant}} {
		token := testutil.ValidTokenFor(t, user.ID.Hex(), "jti-rol-reglas-"+strings.Join(roles, "-"), roles...)
		for _, r := range routes[2:] {
			assert.Equal(t, http.StatusForbidden, sendIfMatch(t, app, r.method, r.path, r.body, token).status, r.method+" "+r.path)
		}
	}
	pending := env.CreateUser(t, "pendiente-dispositivos@wherever.test")
	_, err := env.Users.Update(context.Background(), pending.ID, bson.D{{Key: "estado", Value: usuarios.StatusPendingDeletion}})
	require.NoError(t, err)
	token := testutil.ValidTokenFor(t, pending.ID.Hex(), "jti-pendiente-dispositivos", middleware.RoleConsumer)
	for _, r := range routes[:2] {
		assert.Equal(t, http.StatusForbidden, send(t, app, r.method, r.path, r.body, token).status, r.method+" "+r.path)
	}
}

// El token de FCM no aparece en la ruta, la respuesta ni los logs.
func TestDispositivosReglasSecurity_TokenNuncaSale(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	user := env.CreateUser(t, "token-dispositivo@wherever.test")
	token := testutil.ValidTokenFor(t, user.ID.Hex(), "jti-token-dispositivo", middleware.RoleConsumer)
	fcm := strings.Repeat("f", 150) + "secreto"
	reg := send(t, app, http.MethodPut, "/api/v1/dispositivos", map[string]string{"token": fcm}, token)
	require.Equal(t, http.StatusNoContent, reg.status, reg.body)
	loc := reg.headers.Get("Location")
	assert.NotContains(t, loc, fcm)
	got := send(t, app, http.MethodPut, loc, map[string]any{"canales_activos": []string{"grupos"}}, token)
	require.Equal(t, http.StatusOK, got.status, got.body)
	for _, leak := range []string{fcm, "token_hash", `"token"`} {
		assert.NotContains(t, got.body, leak)
	}
	assert.NotContains(t, env.Logs.String(), fcm)
}

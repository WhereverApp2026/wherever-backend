package security

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
)

// Las siete rutas de nv-40 piden sesión; el token del teléfono no llega a los logs.
func TestNotificacionesSecurity_SesionYTokenFueraDeLogs(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	id := bson.NewObjectID().Hex()
	device := strings.Repeat("f", 150)
	routes := []struct {
		method, path string
		body         any
	}{
		{http.MethodPut, "/api/v1/dispositivos", map[string]string{"token": device}},
		{http.MethodDelete, "/api/v1/dispositivos", map[string]string{"token": device}},
		{http.MethodGet, "/api/v1/notificaciones", nil},
		{http.MethodGet, "/api/v1/notificaciones/no-leidas", nil},
		{http.MethodPost, "/api/v1/notificaciones/leer-todas", nil},
		{http.MethodPatch, "/api/v1/notificaciones/" + id, map[string]bool{"leida": true}},
		{http.MethodDelete, "/api/v1/notificaciones/" + id, nil},
	}
	for _, r := range routes {
		assert.Equal(t, http.StatusUnauthorized, send(t, app, r.method, r.path, r.body, "").status, r.method+" "+r.path)
	}

	u := env.CreateUser(t, "telefono-seguro@wherever.test")
	token := testutil.ValidTokenFor(t, u.ID.Hex(), "jti-telefono", middleware.RoleConsumer)
	assert.Equal(t, http.StatusNoContent, send(t, app, http.MethodPut, "/api/v1/dispositivos", map[string]string{"token": device}, token).status)
	assert.NotContains(t, env.Logs.String(), device)
}

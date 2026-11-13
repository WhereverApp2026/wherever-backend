package security

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
)

// Las seis rutas de moderación (nv-43) son solo de administrador: 401 sin token y 403 a un
// consumidor, antes de leer el cuerpo o la versión.
func TestModeracionSecurity_SoloAdministrador(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	consumer := env.CreateUser(t, "consumidor-moderacion@wherever.test")
	consumerToken := testutil.ValidTokenFor(t, consumer.ID.Hex(), "jti-consumidor-moderacion", middleware.RoleConsumer)
	id := bson.NewObjectID().Hex()
	rutas := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/admin/moderacion"},
		{http.MethodGet, "/api/v1/admin/moderacion/" + id},
		{http.MethodPatch, "/api/v1/admin/moderacion/" + id},
		{http.MethodGet, "/api/v1/admin/moderacion/reglas"},
		{http.MethodPost, "/api/v1/admin/moderacion/reglas"},
		{http.MethodPatch, "/api/v1/admin/moderacion/reglas"},
	}
	body := map[string]any{"decision": "aprobado", "cuarentena": 40, "rechazo": 80}
	for _, r := range rutas {
		assert.Equal(t, http.StatusUnauthorized, send(t, app, r.method, r.path, body, "").status, "%s %s sin token", r.method, r.path)
		assert.Equal(t, http.StatusForbidden, send(t, app, r.method, r.path, body, consumerToken).status, "%s %s consumidor", r.method, r.path)
	}
}

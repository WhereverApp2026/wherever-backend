package contract

import (
	"net/http"
	"testing"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
)

// nv-64: reporte de errores, lista del administrador y perfil por código de amigo.
func TestContract_ErroresYCodigo(t *testing.T) {
	env := authtest.New(t, mongoClient, redisClient, testutil.Config(config.EnvProd))
	env.ClearRateLimits(t)
	app := env.App()
	me := env.CreateUser(t, "contrato-errores@wherever.test")
	other := env.CreateUser(t, "contrato-codigo@wherever.test")
	admin := env.CreateAdmin(t, "admin-contrato-errores@wherever.test")
	token := testutil.ValidTokenFor(t, me.ID.Hex(), "jti-contrato-errores", middleware.RoleConsumer)
	adminToken := testutil.ValidTokenFor(t, admin.ID.Hex(), "jti-contrato-admin-errores", middleware.RoleAdmin)

	status, body := call(t, app, http.MethodPost, "/api/v1/errores", map[string]string{"modulo": "pagos", "mensaje": "falló", "version": "1.4.0"}, token)
	check(t, "errores/reportar_202", status, body)
	status, body = call(t, app, http.MethodGet, "/api/v1/admin/errores?limite=1", nil, adminToken)
	check(t, "errores/admin_200", status, body)

	status, body = call(t, app, http.MethodGet, "/api/v1/personas/codigo/"+other.FriendCode, nil, token)
	check(t, "personas/codigo_200", status, body)
	status, body = call(t, app, http.MethodGet, "/api/v1/personas/codigo/ZZZZZZZZ", nil, token)
	check(t, "personas/codigo_404", status, body)
}

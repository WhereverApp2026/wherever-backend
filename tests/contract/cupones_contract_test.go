package contract

import (
	"net/http"
	"testing"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
)

// TestContract_CuponesValidar fija la forma del rojo al validar (nv-61): 409 con
// `{valido: false, motivo}`, no el error uniforme.
func TestContract_CuponesValidar(t *testing.T) {
	env := authtest.New(t, mongoClient, redisClient, testutil.Config(config.EnvProd))
	app := env.App()
	duena := env.CreateUser(t, "duena-cupones-contrato@wherever.test")
	b := env.AltaAprobada(t, duena, "Contrato Cupones")
	token := testutil.ValidTokenFor(t, duena.ID.Hex(), "jti-cupones-contrato", middleware.RoleConsumer, middleware.RoleMerchant)
	status, body := call(t, app, http.MethodPost, "/api/v1/negocios/"+b.ID+"/cupones/validar", map[string]string{"codigo": "ZZZZZZ"}, token)
	check(t, "cupones/validar_409", status, body)
}

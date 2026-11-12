package contract

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
)

func TestContract_Configuracion(t *testing.T) {
	env := authtest.New(t, mongoClient, redisClient, testutil.Config(config.EnvProd))
	status, body := call(t, env.App(), http.MethodGet, "/api/v1/configuracion", nil, "")
	check(t, "configuracion/configuracion_200", status, body)
	assert.Equal(t, []string{"mantenimiento", "version_actual_app", "version_aviso_privacidad", "version_minima_app", "version_terminos"}, keys(obj(t, body)))
}

func TestContract_ConfiguracionMantenimiento(t *testing.T) {
	cfg := testutil.Config(config.EnvProd)
	cfg.VersionApp.Mantenimiento = true
	env := authtest.New(t, mongoClient, redisClient, cfg)
	status, body := call(t, env.App(), http.MethodGet, "/api/v1/configuracion", nil, "")
	check(t, "configuracion/configuracion_mantenimiento_200", status, body)
	assert.Equal(t, true, obj(t, body)["mantenimiento"])
}

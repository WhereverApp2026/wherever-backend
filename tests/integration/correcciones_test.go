package integration

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
)

var gustosConAlergias = map[string]any{
	"categorias": []string{"cafe", "tacos", "museos"}, "presupuesto_min": 100, "presupuesto_max": 300, "alergias": "nueces",
}

// US-010: sin consentimiento expreso las alergias no se guardan.
func TestConsentimiento_SinElLasAlergiasNoSeGuardan(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	access := registerHTTP(t, app, lu)

	resp := sendJSON(t, app, http.MethodPut, "/api/v1/usuarios/yo/gustos", gustosConAlergias, access)
	raw := readAll(t, resp)
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode, raw)
	assert.Contains(t, raw, `"campo":"alergias"`)

	perfil := decode(t, sendJSON(t, app, http.MethodGet, "/api/v1/usuarios/yo", nil, access))
	assert.Nil(t, perfil["gustos"], "no se guarda nada")
}

func TestConsentimiento_SinAlergiasNoHaceFalta(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	access := registerHTTP(t, app, lu)

	for _, alergias := range []string{"", "   "} {
		resp := sendJSON(t, app, http.MethodPut, "/api/v1/usuarios/yo/gustos", map[string]any{
			"categorias": []string{"cafe", "tacos", "museos"}, "presupuesto_min": 100, "presupuesto_max": 300, "alergias": alergias,
		}, access)
		assert.Equal(t, http.StatusOK, resp.StatusCode, "alergias %q", alergias)
	}
}

func TestConsentimiento_ConElSeGuardanYAlRetirarloSeBorran(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	access := registerHTTP(t, app, lu)
	privacidad := "/api/v1/usuarios/yo/privacidad"

	require.Equal(t, http.StatusOK, sendJSON(t, app, http.MethodPatch, privacidad, map[string]any{"datos_sensibles": true}, access).StatusCode)
	resp := sendJSON(t, app, http.MethodPut, "/api/v1/usuarios/yo/gustos", gustosConAlergias, access)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "nueces", decode(t, resp)["alergias"])

	require.Equal(t, http.StatusOK, sendJSON(t, app, http.MethodPatch, privacidad, map[string]any{"datos_sensibles": false}, access).StatusCode)

	perfil := decode(t, sendJSON(t, app, http.MethodGet, "/api/v1/usuarios/yo", nil, access))
	gustos := obj(t, perfil["gustos"])
	assert.Empty(t, gustos["alergias"])
	assert.NotEmpty(t, gustos["categorias"], "lo demás se conserva")
	u, err := env.Users.FindByEmail(t.Context(), lu)
	require.NoError(t, err)
	assert.Empty(t, u.Tastes.Allergies, "también se borra de la base")
}

// Retirarlo sin haber guardado gustos no debe fallar: `gustos` está vacío.
func TestConsentimiento_RetirarloSinGustos(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	access := registerHTTP(t, app, lu)

	resp := sendJSON(t, app, http.MethodPatch, "/api/v1/usuarios/yo/privacidad", map[string]any{"datos_sensibles": false}, access)
	assert.Equal(t, http.StatusOK, resp.StatusCode, readAll(t, resp))
}

func TestInternos_OcultosConHideInternals(t *testing.T) {
	cfg := testutil.Config(config.EnvStaging)
	cfg.HideInternals = true
	app := newAppWith(cfg)

	resp := get(t, app, "/api/v1/probe/error")
	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "sin sondas")
	assert.NotContains(t, decode(t, resp), "detalle")
	assert.Equal(t, http.StatusNotFound, get(t, app, "/docs").StatusCode)
	assert.Equal(t, http.StatusOK, get(t, app, "/health").StatusCode)
}

// La causa técnica no sale en un servidor público, aunque se registre en el log.
func TestInternos_ErrorDeBaseSinDetalle(t *testing.T) {
	cfg := testutil.Config(config.EnvStaging)
	cfg.HideInternals = true
	app := authtest.Build(t, mongoAt(t, testutil.DeadAddr), redisClient, cfg).App()

	resp := sendJSON(t, app, http.MethodGet, "/api/v1/ciudades/autocompletar?q=guada", nil, "")
	raw := readAll(t, resp)
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode, raw)
	assert.NotContains(t, raw, "detalle")
	assert.NotContains(t, raw, testutil.DeadAddr)
}

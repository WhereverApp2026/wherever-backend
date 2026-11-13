package integration

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
)

// Lo que el plan gratuito de Upstash no cobra (nv-11).
var unbilledCommands = map[string]bool{
	"auth": true, "hello": true, "reset": true, "ping": true, "select": true,
	"quit": true, "info": true, "config": true, "command": true,
}

// billedCommands suma las llamadas de INFO commandstats sin los comandos que no se cobran.
func billedCommands(t *testing.T) int {
	t.Helper()
	raw, err := redisClient.Info(context.Background(), "commandstats").Result()
	require.NoError(t, err)
	total := 0
	for line := range strings.Lines(raw) {
		name, stats, found := strings.Cut(strings.TrimPrefix(strings.TrimSpace(line), "cmdstat_"), ":")
		if !found {
			continue
		}
		base, _, _ := strings.Cut(name, "|")
		if unbilledCommands[base] {
			continue
		}
		calls, _, _ := strings.Cut(strings.TrimPrefix(stats, "calls="), ",")
		count, err := strconv.Atoi(calls)
		require.NoError(t, err, line)
		total += count
	}
	return total
}

// commandsOf cuenta lo que gasta una operación.
func commandsOf(t *testing.T, operation func()) int {
	t.Helper()
	require.NoError(t, redisClient.ConfigResetStat(context.Background()).Err())
	operation()
	return billedCommands(t)
}

// Si sube alguna de estas cifras, hay que rehacer la estimación mensual de specs/nv-11.
func TestPresupuestoRedis_ComandosPorOperacion(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	access := registerHTTP(t, app, "presupuesto@example.com")

	assert.Equal(t, 0, commandsOf(t, func() {
		assert.Equal(t, http.StatusOK, get(t, app, "/health").StatusCode)
	}), "/health no gasta: lo consulta el balanceador")

	assert.Equal(t, 3, commandsOf(t, func() {
		get(t, app, "/ready")
	}), "/ready: solo el limitador; PING no se cobra")

	assert.Equal(t, 4, commandsOf(t, func() {
		resp := sendJSON(t, app, http.MethodGet, "/api/v1/usuarios/yo", nil, access)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	}), "lectura autenticada: limitador + revocación")

	assert.Equal(t, 7, commandsOf(t, func() {
		resp := sendJSON(t, app, http.MethodPatch, "/api/v1/usuarios/yo/privacidad",
			map[string]any{"datos_sensibles": false}, access)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	}), "escritura autenticada: limitador global + escrituras + revocación")
}

func TestPresupuestoRedis_HealthNoTieneLimite(t *testing.T) {
	cfg := testutil.Config(config.EnvDev)
	cfg.RateLimitGlobal = 1
	app := authEnvWith(t, cfg).App()

	for range 3 {
		assert.Equal(t, http.StatusOK, get(t, app, "/health").StatusCode)
	}
	assert.NotEqual(t, http.StatusTooManyRequests, get(t, app, "/ready").StatusCode, "/health no consumió la ventana")
	assert.Equal(t, http.StatusTooManyRequests, get(t, app, "/ready").StatusCode, "el resto sí sigue limitado")
}

// La caché de nv-18 suma, sobre el limitador, un GET al acertar y GET + SET al fallar.
func TestPresupuestoRedis_CacheDeFicha(t *testing.T) {
	env, app := lugaresEnv(t)
	placed := insertPlaces(t, env, []placeSpec{{slug: "presupuesto"}})
	ficha := "/api/v1/lugares/" + placed[0].ID.Hex()

	base := commandsOf(t, func() { get(t, app, "/ready") })
	fallo := commandsOf(t, func() {
		assert.Equal(t, http.StatusOK, get(t, app, ficha).StatusCode)
	})
	acierto := commandsOf(t, func() {
		assert.Equal(t, http.StatusOK, get(t, app, ficha).StatusCode)
	})
	assert.Equal(t, 2, fallo-base, "fallo: GET + SET")
	assert.Equal(t, 1, acierto-base, "acierto: GET")
}

// nv-64: las dos políticas nuevas (`personas_codigo`, `errores_reporte`) cuestan lo mismo que
// la ventana por usuario de `personas_buscar` (nv-35): una ruta igual más esa ventana.
func TestPresupuestoRedis_VentanasPorUsuarioNV64(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	access := registerHTTP(t, app, "presupuesto64@example.com")

	read := commandsOf(t, func() {
		assert.Equal(t, http.StatusOK, sendJSON(t, app, http.MethodGet, "/api/v1/usuarios/yo", nil, access).StatusCode)
	})
	write := commandsOf(t, func() {
		resp := sendJSON(t, app, http.MethodPatch, "/api/v1/usuarios/yo/privacidad", map[string]any{"datos_sensibles": false}, access)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
	window := commandsOf(t, func() {
		assert.Equal(t, http.StatusOK, sendJSON(t, app, http.MethodGet, "/api/v1/personas/buscar?q=nadie", nil, access).StatusCode)
	}) - read
	require.Positive(t, window)
	t.Logf("ventana por usuario: %d comandos", window)

	assert.Equal(t, read+window, commandsOf(t, func() {
		assert.Equal(t, http.StatusNotFound, sendJSON(t, app, http.MethodGet, "/api/v1/personas/codigo/ZZZZZZZZ", nil, access).StatusCode)
	}), "GET /personas/codigo: lectura autenticada + ventana")
	assert.Equal(t, write+window, commandsOf(t, func() {
		resp := sendJSON(t, app, http.MethodPost, "/api/v1/errores", map[string]string{"modulo": "pagos", "mensaje": "falló", "version": "1.4.0"}, access)
		assert.Equal(t, http.StatusAccepted, resp.StatusCode)
	}), "POST /errores: escritura autenticada + ventana")
}

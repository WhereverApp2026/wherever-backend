package integration

import (
	"compress/gzip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
)

func TestCompresion_PerfilComprimidoYRegistroSinComprimir(t *testing.T) {
	app := authEnv(t).App()
	access := registerHTTP(t, app, "compresion@example.com")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/usuarios/yo", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	req.Header.Set("Accept-Encoding", "gzip")
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "gzip", resp.Header.Get("Content-Encoding"))
	reader, err := gzip.NewReader(resp.Body)
	require.NoError(t, err)
	var perfil map[string]any
	require.NoError(t, json.NewDecoder(reader).Decode(&perfil), "el cuerpo se descomprime a JSON válido")
	assert.Equal(t, "compresion@example.com", perfil["correo"])
}

func TestSondaLenta_RespetaElPlazoYSeOculta(t *testing.T) {
	cfg := testutil.Config(config.EnvDev)
	cfg.RequestTimeout = 50 * time.Millisecond
	start := time.Now()
	resp := get(t, newAppWith(cfg), "/api/v1/probe/lenta")
	assert.Equal(t, http.StatusRequestTimeout, resp.StatusCode, "corta con el plazo de la petición")
	assert.Less(t, time.Since(start), 5*time.Second)

	hidden := testutil.Config(config.EnvStaging)
	hidden.HideInternals = true
	assert.Equal(t, http.StatusNotFound, get(t, newAppWith(hidden), "/api/v1/probe/lenta").StatusCode)
}

package configuracion_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/configuracion"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/versionado"
)

func versiones(t *testing.T, mantenimiento bool) config.VersionApp {
	t.Helper()
	minima, err := versionado.Parse("1.4.0")
	require.NoError(t, err)
	actual, err := versionado.Parse("1.6.2")
	require.NoError(t, err)
	return config.VersionApp{Minima: minima, Actual: actual, Terminos: "2026-10", Aviso: "2026-11", Mantenimiento: mantenimiento}
}

func app(t *testing.T, v config.VersionApp) *fiber.App {
	t.Helper()
	svc := configuracion.NewService(v)
	a := fiber.New(fiber.Config{DisableStartupMessage: true})
	configuracion.NewHandler(svc).Routes(a.Group("/api/v1"), nil)
	return a
}

func get(t *testing.T, a *fiber.App, ifNoneMatch string) (*http.Response, []byte) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/configuracion", nil)
	if ifNoneMatch != "" {
		req.Header.Set(fiber.HeaderIfNoneMatch, ifNoneMatch)
	}
	resp, err := a.Test(req, -1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp, raw
}

func TestConfiguracionHTTP_CincoCampos(t *testing.T) {
	resp, raw := get(t, app(t, versiones(t, false)), "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "no-cache", resp.Header.Get(fiber.HeaderCacheControl))
	var body map[string]any
	require.NoError(t, json.Unmarshal(raw, &body))
	assert.Equal(t, map[string]any{
		"version_minima_app": "1.4.0", "version_actual_app": "1.6.2",
		"version_terminos": "2026-10", "version_aviso_privacidad": "2026-11", "mantenimiento": false,
	}, body)
}

func TestConfiguracionHTTP_ETagEstableYCambia(t *testing.T) {
	a := app(t, versiones(t, false))
	r1, _ := get(t, a, "")
	r2, _ := get(t, a, "")
	assert.Regexp(t, `^W/"[0-9a-f]{32}"$`, r1.Header.Get(fiber.HeaderETag))
	assert.Equal(t, r1.Header.Get(fiber.HeaderETag), r2.Header.Get(fiber.HeaderETag))
	r3, _ := get(t, app(t, versiones(t, true)), "")
	assert.NotEqual(t, r1.Header.Get(fiber.HeaderETag), r3.Header.Get(fiber.HeaderETag))
}

func TestConfiguracionHTTP_304ConIfNoneMatch(t *testing.T) {
	a := app(t, versiones(t, false))
	r1, _ := get(t, a, "")
	resp, raw := get(t, a, r1.Header.Get(fiber.HeaderETag))
	assert.Equal(t, http.StatusNotModified, resp.StatusCode)
	assert.Empty(t, raw)
	assert.Equal(t, r1.Header.Get(fiber.HeaderETag), resp.Header.Get(fiber.HeaderETag))
}

func TestConfiguracionHTTP_Mantenimiento(t *testing.T) {
	resp, raw := get(t, app(t, versiones(t, true)), "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var body map[string]any
	require.NoError(t, json.Unmarshal(raw, &body))
	assert.Equal(t, true, body["mantenimiento"])
}

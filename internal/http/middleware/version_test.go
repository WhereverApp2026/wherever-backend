package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/versionado"
)

func versionApp(t *testing.T, mantenimiento bool) *fiber.App {
	t.Helper()
	minima, err := versionado.Parse("1.4.0")
	if err != nil {
		t.Fatal(err)
	}
	app := newTestApp(false, discardLogger(), middleware.Version(versionado.NuevaPolitica(minima, mantenimiento)))
	ok := func(c *fiber.Ctx) error { return c.SendStatus(http.StatusOK) }
	app.Get("/api/v1/planes", ok)
	app.Post("/api/v1/auth/login", ok)
	app.Get("/api/v1/configuracion", ok)
	return app
}

func withVersion(t *testing.T, app *fiber.App, method, path, version string) *http.Response {
	req := httptest.NewRequest(method, path, nil)
	if version != "" {
		req.Header.Set(middleware.HeaderAppVersion, version)
	}
	return do(t, app, req)
}

func TestVersion_ViejaDa426(t *testing.T) {
	resp := withVersion(t, versionApp(t, false), http.MethodGet, "/api/v1/planes", "1.3.9")
	assert.Equal(t, http.StatusUpgradeRequired, resp.StatusCode)
	body := decode(t, resp)
	assert.Equal(t, "actualizacion_requerida", body["codigo"])
	assert.Equal(t, "actualiza la app a la versión 1.4.0 o superior", body["mensaje"])
	assert.Equal(t, []any{}, body["campos"])
	assert.NotEmpty(t, body["traza_id"])
}

func TestVersion_IgualONuevaSigue(t *testing.T) {
	app := versionApp(t, false)
	for _, v := range []string{"1.4.0", "v1.10.0"} {
		assert.Equal(t, http.StatusOK, withVersion(t, app, http.MethodGet, "/api/v1/planes", v).StatusCode, v)
	}
}

func TestVersion_AusenteSigue(t *testing.T) {
	assert.Equal(t, http.StatusOK, withVersion(t, versionApp(t, false), http.MethodGet, "/api/v1/planes", "").StatusCode)
}

func TestVersion_IlegibleSigue(t *testing.T) {
	assert.Equal(t, http.StatusOK, withVersion(t, versionApp(t, false), http.MethodGet, "/api/v1/planes", "1.2.3-beta").StatusCode)
}

func TestVersion_AuthExenta(t *testing.T) {
	assert.Equal(t, http.StatusOK, withVersion(t, versionApp(t, false), http.MethodPost, "/api/v1/auth/login", "0.1.0").StatusCode)
}

func TestVersion_MantenimientoDa503(t *testing.T) {
	resp := withVersion(t, versionApp(t, true), http.MethodGet, "/api/v1/planes", "2.0.0")
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	assert.Equal(t, "60", resp.Header.Get(fiber.HeaderRetryAfter))
	body := decode(t, resp)
	assert.Equal(t, "no_disponible", body["codigo"])
	assert.Equal(t, "en mantenimiento", body["mensaje"])
}

func TestVersion_ConfiguracionEnMantenimientoSigue(t *testing.T) {
	assert.Equal(t, http.StatusOK, withVersion(t, versionApp(t, true), http.MethodGet, "/api/v1/configuracion", "0.1.0").StatusCode)
}

func TestVersion_FueraDeV1ConservaEl404(t *testing.T) {
	app := versionApp(t, true)
	assert.Equal(t, http.StatusNotFound, withVersion(t, app, http.MethodGet, "/planes", "0.1.0").StatusCode)
	assert.Equal(t, http.StatusNotFound, withVersion(t, versionApp(t, false), http.MethodGet, "/planes", "0.1.0").StatusCode)
}

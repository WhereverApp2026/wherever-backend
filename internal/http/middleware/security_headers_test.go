package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

func headersApp(env string, handler fiber.Handler) *fiber.App {
	app := fiber.New(fiber.Config{DisableStartupMessage: true, ErrorHandler: middleware.NewErrorHandler(true, discardLogger())})
	app.Use(middleware.SecurityHeaders(env))
	app.Get("/*", handler)
	return app
}

func okHandler(c *fiber.Ctx) error { return c.SendString("ok") }

func TestSecurityHeaders_SetsBaseHeaders(t *testing.T) {
	resp := get(t, headersApp("dev", okHandler), "/cualquiera")

	assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", resp.Header.Get("X-Frame-Options"))
	assert.Equal(t, "strict-origin-when-cross-origin", resp.Header.Get("Referrer-Policy"))
	assert.Equal(t, "0", resp.Header.Get("X-XSS-Protection"))
	assert.Empty(t, resp.Header.Get("Strict-Transport-Security"), "sin HSTS en dev")
}

func TestSecurityHeaders_StrictCSP(t *testing.T) {
	for _, env := range []string{"dev", "staging", "prod"} {
		resp := get(t, headersApp(env, okHandler), "/api/v1/ping")
		assert.Equal(t, "default-src 'none'; frame-ancestors 'none'", resp.Header.Get("Content-Security-Policy"), env)
	}
}

func TestSecurityHeaders_DocsExemptInDev(t *testing.T) {
	resp := get(t, headersApp("dev", okHandler), "/docs")

	csp := resp.Header.Get("Content-Security-Policy")
	assert.Contains(t, csp, "script-src https://unpkg.com")
	assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"), "el resto de encabezados sigue")
}

func TestSecurityHeaders_OpenAPIExemptInDevOnly(t *testing.T) {
	dev := get(t, headersApp("dev", okHandler), "/openapi.json")
	prod := get(t, headersApp("prod", okHandler), "/openapi.json")

	assert.Contains(t, dev.Header.Get("Content-Security-Policy"), "unpkg.com")
	assert.Equal(t, "default-src 'none'; frame-ancestors 'none'", prod.Header.Get("Content-Security-Policy"))
}

func TestSecurityHeaders_SetBeforeNext(t *testing.T) {
	var seen string
	app := headersApp("prod", func(c *fiber.Ctx) error {
		seen = string(c.Response().Header.Peek("X-Frame-Options"))
		return apperr.Forbidden("no")
	})

	resp := get(t, app, "/x")

	assert.Equal(t, "DENY", seen, "el handler ya ve los encabezados")
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	assert.Equal(t, "DENY", resp.Header.Get("X-Frame-Options"), "también en la respuesta de error")
	assert.Equal(t, "max-age=31536000; includeSubDomains", resp.Header.Get("Strict-Transport-Security"))
}

func TestRequestID_UniquePerRequest(t *testing.T) {
	app := newTestApp(true, discardLogger())
	app.Get("/x", okHandler)

	first := get(t, app, "/x").Header.Get(middleware.HeaderRequestID)
	second := get(t, app, "/x").Header.Get(middleware.HeaderRequestID)

	require.Len(t, first, 32)
	assert.NotEqual(t, first, second)
}

func TestRequestID_KeepsValidIncoming(t *testing.T) {
	app := newTestApp(true, discardLogger())
	app.Get("/x", okHandler)
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set(middleware.HeaderRequestID, "cliente-abc-123")

	resp := do(t, app, req)

	assert.Equal(t, "cliente-abc-123", resp.Header.Get(middleware.HeaderRequestID))
}

func TestRequestID_ReplacesInvalidIncoming(t *testing.T) {
	app := newTestApp(true, discardLogger())
	app.Get("/x", okHandler)
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set(middleware.HeaderRequestID, "<script>alert(1)</script>")

	resp := do(t, app, req)

	got := resp.Header.Get(middleware.HeaderRequestID)
	assert.NotContains(t, got, "<")
	assert.Len(t, got, 32)
}

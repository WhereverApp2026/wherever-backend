package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
)

const corsOrigin = "https://app.wherever.test"

func corsApp() *fiber.App {
	app := newTestApp(false, discardLogger(), middleware.CORS(corsOrigin))
	app.Get("/api/v1/configuracion", func(c *fiber.Ctx) error { return c.SendStatus(http.StatusOK) })
	return app
}

func TestCORS_PreflightPermiteVersionYCondicional(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/configuracion", nil)
	req.Header.Set(fiber.HeaderOrigin, corsOrigin)
	req.Header.Set(fiber.HeaderAccessControlRequestMethod, http.MethodGet)
	resp := do(t, corsApp(), req)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	allowed := resp.Header.Get(fiber.HeaderAccessControlAllowHeaders)
	assert.Contains(t, allowed, middleware.HeaderAppVersion)
	assert.Contains(t, allowed, fiber.HeaderIfNoneMatch)
}

func TestCORS_ExponeETag(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/configuracion", nil)
	req.Header.Set(fiber.HeaderOrigin, corsOrigin)
	resp := do(t, corsApp(), req)
	assert.Contains(t, resp.Header.Get(fiber.HeaderAccessControlExposeHeaders), fiber.HeaderETag)
}

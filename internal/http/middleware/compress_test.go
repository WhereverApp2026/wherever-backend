package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
)

// fasthttp no comprime cuerpos de menos de 200 bytes.
var longBody = strings.Repeat("wherever ", 100)

func compressApp() *fiber.App {
	app := fiber.New()
	app.Use(middleware.Compress())
	reply := func(c *fiber.Ctx) error { return c.SendString(longBody) }
	app.Get("/api/v1/ciudades", reply)
	app.Post("/api/v1/auth/login", reply)
	app.Get("/api/v1/auth/sesiones", reply)
	app.Post("/api/v1/usuarios", reply)
	app.Get("/api/v1/usuarios", reply)
	return app
}

func encodingOf(t *testing.T, method, path, accept string) string {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	if accept != "" {
		req.Header.Set(fiber.HeaderAcceptEncoding, accept)
	}
	resp, err := compressApp().Test(req, -1)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	return resp.Header.Get(fiber.HeaderContentEncoding)
}

func TestCompress_RespetaAcceptEncoding(t *testing.T) {
	assert.Equal(t, "gzip", encodingOf(t, http.MethodGet, "/api/v1/ciudades", "gzip"))
	assert.Empty(t, encodingOf(t, http.MethodGet, "/api/v1/ciudades", ""), "sin la cabecera no se comprime")
}

func TestCompress_LoQueLlevaTokensNoSeComprime(t *testing.T) {
	assert.Empty(t, encodingOf(t, http.MethodPost, "/api/v1/auth/login", "gzip"))
	assert.Empty(t, encodingOf(t, http.MethodGet, "/api/v1/auth/sesiones", "gzip"))
	assert.Empty(t, encodingOf(t, http.MethodPost, "/api/v1/usuarios", "gzip"), "el registro devuelve tokens")
	assert.Equal(t, "gzip", encodingOf(t, http.MethodGet, "/api/v1/usuarios", "gzip"), "solo se excluye el POST")
}

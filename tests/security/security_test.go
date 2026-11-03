// Package security prueba la app real en proceso contra ataques base: encabezados, JWT y
// auth (nv-04). Las de este archivo usan bases caídas: un token inválido se rechaza antes de
// consultar Redis. Las de auth_security_test.go usan MongoDB y Redis reales.
package security

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	httpapi "github.com/WhereverApp2026/wherever-backend/internal/http"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
)

func newApp(t *testing.T, env string) *fiber.App {
	log, _ := testutil.Logger()
	deps := httpapi.Deps{
		Logger:  log,
		Version: "v0.0.0-test",
		Redis:   testutil.DeadRedis(t),
		Mongo:   testutil.DownPinger{},
		Neo4j:   testutil.DownPinger{},
	}
	return httpapi.NewApp(testutil.Config(env), deps)
}

func get(t *testing.T, app *fiber.App, path string, token string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set(fiber.HeaderAuthorization, "Bearer "+token)
	}
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func assertSecurityHeaders(t *testing.T, resp *http.Response) {
	t.Helper()
	assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", resp.Header.Get("X-Frame-Options"))
	assert.Equal(t, "strict-origin-when-cross-origin", resp.Header.Get("Referrer-Policy"))
	assert.Equal(t, "0", resp.Header.Get("X-XSS-Protection"))
	assert.Equal(t, "default-src 'none'; frame-ancestors 'none'", resp.Header.Get("Content-Security-Policy"))
}

func TestHeaders_AnonymousHealthHasSecurityHeaders(t *testing.T) {
	resp := get(t, newApp(t, "dev"), "/health", "")

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assertSecurityHeaders(t, resp)
}

func TestHeaders_ForbiddenResponseHasSecurityHeaders(t *testing.T) {
	resp := get(t, newApp(t, "staging"), "/api/v1/probe/forbidden", "")

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	assertSecurityHeaders(t, resp)
}

func TestHeaders_HSTS365DaysInProd(t *testing.T) {
	resp := get(t, newApp(t, "prod"), "/health", "")

	assert.Equal(t, "max-age=31536000; includeSubDomains", resp.Header.Get("Strict-Transport-Security"))
}

func TestJWT_AlgNoneIs401(t *testing.T) {
	resp := get(t, newApp(t, "prod"), "/api/v1/ping", testutil.AlgNoneToken(t))

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestJWT_UnsignedIs401(t *testing.T) {
	resp := get(t, newApp(t, "prod"), "/api/v1/ping", testutil.UnsignedToken(t))

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestJWT_ExpiredIs401(t *testing.T) {
	resp := get(t, newApp(t, "prod"), "/api/v1/ping", testutil.ExpiredToken(t))

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestJWT_MalformedOrTamperedIs401(t *testing.T) {
	app := newApp(t, "prod")
	for name, token := range map[string]string{
		"basura":         "no.es.un.jwt",
		"sin Bearer":     "",
		"firma alterada": testutil.ValidToken(t, "jti-x") + "x",
	} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/ping", nil)
			if token == "" {
				req.Header.Set(fiber.HeaderAuthorization, "Token abc")
			} else {
				req.Header.Set(fiber.HeaderAuthorization, "Bearer "+token)
			}
			resp, err := app.Test(req, -1)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		})
	}
}

func TestCORS_OnlyConfiguredOrigin(t *testing.T) {
	cfg := testutil.Config("prod")
	cfg.CORSAllowedOrigin = "https://app.wherever.test"
	log, _ := testutil.Logger()
	app := httpapi.NewApp(cfg, httpapi.Deps{Logger: log, Redis: testutil.DeadRedis(t), Mongo: testutil.DownPinger{}, Neo4j: testutil.DownPinger{}})

	for origin, want := range map[string]string{
		"https://app.wherever.test": "https://app.wherever.test",
		"https://evil.test":         "",
	} {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		req.Header.Set(fiber.HeaderOrigin, origin)
		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		_ = resp.Body.Close()
		assert.Equal(t, want, resp.Header.Get(fiber.HeaderAccessControlAllowOrigin), origin)
	}
}

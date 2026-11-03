package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/auth"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
)

var device = auth.Device{Model: "Pixel 8", System: "Android 15", AppVersion: "1.0.0"}

var deviceJSON = map[string]string{"modelo": "Pixel 8", "sistema": "Android 15", "app_version": "1.0.0"}

func authEnv(t *testing.T) *authtest.Env {
	t.Helper()
	return authEnvWith(t, testutil.Config(config.EnvDev))
}

func authEnvWith(t *testing.T, cfg config.Config) *authtest.Env {
	t.Helper()
	return authtest.New(t, mongoClient, redisClient, cfg)
}

// sendJSON manda un cuerpo JSON (o nada si body es nil) con Bearer opcional.
func sendJSON(t *testing.T, app *fiber.App, method, path string, body any, token string) *http.Response {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	if token != "" {
		req.Header.Set(fiber.HeaderAuthorization, "Bearer "+token)
	}
	resp, err := app.Test(req, int((20 * time.Second).Milliseconds()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func loginBody(email, password string) map[string]any {
	return map[string]any{"correo": email, "password": password, "dispositivo": deviceJSON}
}

// loginHTTP inicia sesión por HTTP y devuelve los tokens.
func loginHTTP(t *testing.T, app *fiber.App, email string) (access, refresh string) {
	t.Helper()
	resp := sendJSON(t, app, http.MethodPost, "/api/v1/auth/login", loginBody(email, authtest.Password), "")
	raw := readAll(t, resp)
	require.Equal(t, http.StatusOK, resp.StatusCode, raw)
	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &body))
	return str(t, body["access_token"]), str(t, body["refresh_token"])
}

// parseClaims valida el access token igual que el middleware.
func parseClaims(t *testing.T, env *authtest.Env, raw string) *middleware.Claims {
	t.Helper()
	claims, err := middleware.NewAccessParser(middleware.JWTConfig{Key: env.Cfg.JWTKey, Issuer: env.Cfg.JWTIssuer, Audience: env.Cfg.JWTAudience}).Parse(raw)
	require.NoError(t, err)
	return claims
}

func decodeInto(resp *http.Response, target any) error {
	return json.NewDecoder(resp.Body).Decode(target)
}

// sendJSONRaw manda un cuerpo tal cual, para probar JSON ilegible.
func sendJSONRaw(t *testing.T, app *fiber.App, path, raw string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader([]byte(raw)))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req, int((20 * time.Second).Milliseconds()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// authtestBuild arma una segunda instancia sobre las mismas bases, sin limpiarlas.
func authtestBuild(t *testing.T, env *authtest.Env) *authtest.Env {
	t.Helper()
	return authtest.Build(t, env.Mongo, env.Redis, env.Cfg)
}

// Afirmaciones de tipo comprobadas (errcheck con check-type-assertions).

func str(t *testing.T, v any) string {
	t.Helper()
	s, ok := v.(string)
	require.True(t, ok, "se esperaba string: %v", v)
	return s
}

func boolean(t *testing.T, v any) bool {
	t.Helper()
	b, ok := v.(bool)
	require.True(t, ok, "se esperaba bool: %v", v)
	return b
}

func obj(t *testing.T, v any) map[string]any {
	t.Helper()
	m, ok := v.(map[string]any)
	require.True(t, ok, "se esperaba objeto: %v", v)
	return m
}

func list(t *testing.T, v any) []any {
	t.Helper()
	l, ok := v.([]any)
	require.True(t, ok, "se esperaba lista: %v", v)
	return l
}

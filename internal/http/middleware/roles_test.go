package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
)

func TestRequireRoles_AnonymousIs401(t *testing.T) {
	// Sin token no se consulta Redis; el 403 por rol se prueba en Integration con Redis real.
	jwtCfg := middleware.JWTConfig{Key: []byte(testutil.SigningKey), Issuer: testutil.Issuer, Audience: testutil.Audience}
	app := newTestApp(true, discardLogger(), middleware.JWT(jwtCfg, testutil.DeadRedis(t)))
	app.Get("/admin", middleware.RequireRoles(middleware.RoleAdmin), okHandler)

	anonymous := get(t, app, "/admin")

	assert.Equal(t, http.StatusUnauthorized, anonymous.StatusCode)
}

func TestRequireAuth_AnonymousIs401(t *testing.T) {
	jwtCfg := middleware.JWTConfig{Key: []byte(testutil.SigningKey), Issuer: testutil.Issuer, Audience: testutil.Audience}
	app := newTestApp(true, discardLogger(), middleware.JWT(jwtCfg, testutil.DeadRedis(t)))
	app.Get("/yo", middleware.RequireAuth(), okHandler)

	assert.Equal(t, http.StatusUnauthorized, get(t, app, "/yo").StatusCode)
}

func TestRequireRoles_UnknownRolePanicsAtBuild(t *testing.T) {
	assert.Panics(t, func() { middleware.RequireRoles("superusuario") })
}

func TestJWT_RevocationCheckIsFailClosed(t *testing.T) {
	jwtCfg := middleware.JWTConfig{Key: []byte(testutil.SigningKey), Issuer: testutil.Issuer, Audience: testutil.Audience}
	app := newTestApp(true, discardLogger(), middleware.JWT(jwtCfg, testutil.DeadRedis(t)))
	app.Get("/x", okHandler)
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+testutil.ValidToken(t, "jti-1", middleware.RoleConsumer))

	resp := do(t, app, req)

	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode, "sin Redis no se supone que el token no está revocado")
	assert.Equal(t, "no_disponible", decode(t, resp)["codigo"])
}

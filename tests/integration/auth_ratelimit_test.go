package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
)

func bearer(token string) map[string]string {
	return map[string]string{fiber.HeaderAuthorization: "Bearer " + token}
}

func TestJWT_ValidTokenPassesAndRevokedIs401(t *testing.T) {
	app := newApp("prod", realDeps())
	jti := "jti-integracion-" + time.Now().Format("150405.000")
	token := testutil.ValidToken(t, jti, middleware.RoleConsumer)

	assert.Equal(t, http.StatusOK, request(t, app, http.MethodGet, "/api/v1/ping", bearer(token)).StatusCode)

	require.NoError(t, redisClient.Set(context.Background(), middleware.RevokedKey(jti), "1", time.Minute).Err())
	revoked := request(t, app, http.MethodGet, "/api/v1/ping", bearer(token))
	assert.Equal(t, http.StatusUnauthorized, revoked.StatusCode)
}

func TestRoles_WrongRoleIs403(t *testing.T) {
	app := newApp("prod", realDeps())
	app.Get("/api/v1/solo-admin", middleware.RequireRoles(middleware.RoleAdmin), func(c *fiber.Ctx) error {
		return c.SendStatus(http.StatusNoContent)
	})

	consumer := request(t, app, http.MethodGet, "/api/v1/solo-admin", bearer(testutil.ValidToken(t, "jti-rol-1", middleware.RoleConsumer)))
	admin := request(t, app, http.MethodGet, "/api/v1/solo-admin", bearer(testutil.ValidToken(t, "jti-rol-2", middleware.RoleAdmin)))

	assert.Equal(t, http.StatusForbidden, consumer.StatusCode)
	assert.Equal(t, http.StatusNoContent, admin.StatusCode)
}

func TestRateLimit_CountsInRedisSharedAcrossInstances(t *testing.T) {
	cfg := testutil.Config("prod")
	cfg.RateLimitGlobal = 3
	ctx := context.Background()
	require.NoError(t, redisClient.Del(ctx, "rate:global:0.0.0.0").Err())
	// Dos instancias de la app sobre el mismo Redis: el límite es compartido.
	first, second := newAppWith(cfg), newAppWith(cfg)

	statuses := []int{
		get(t, first, "/api/v1/ping").StatusCode,
		get(t, second, "/api/v1/ping").StatusCode,
		get(t, first, "/api/v1/ping").StatusCode,
		get(t, second, "/api/v1/ping").StatusCode,
	}

	assert.Equal(t, []int{200, 200, 200, 429}, statuses)
	ttl, err := redisClient.TTL(ctx, "rate:global:0.0.0.0").Result()
	require.NoError(t, err)
	assert.Positive(t, ttl, "toda llave de Redis lleva TTL")
}

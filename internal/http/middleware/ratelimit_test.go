package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
)

func TestRateLimit_Uniform429WithRetryAfter(t *testing.T) {
	log, _ := testutil.Logger()
	limiter := middleware.NewLimiter(testutil.DeadRedis(t), log)
	app := newTestApp(true, log, middleware.ProxyHeaders(1), limiter.Window(middleware.PolicyGlobal, 2, time.Minute))
	app.Get("/x", okHandler)

	for i := 0; i < 2; i++ {
		require.Equal(t, http.StatusOK, get(t, app, "/x").StatusCode)
	}
	resp := get(t, app, "/x")
	body := decode(t, resp)

	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.Equal(t, "demasiadas_peticiones", body["codigo"])
	retry, err := strconv.Atoi(resp.Header.Get("Retry-After"))
	require.NoError(t, err)
	assert.InDelta(t, 60, retry, 2)
}

func TestRateLimit_RedisDownFallsBackToMemoryAndWarns(t *testing.T) {
	log, logs := testutil.Logger()
	limiter := middleware.NewLimiter(testutil.DeadRedis(t), log)
	app := newTestApp(true, log, middleware.ProxyHeaders(1),
		limiter.Window(middleware.PolicyGlobal, 100, time.Minute),
		limiter.Concurrent(middleware.PolicyWrites, 10))
	app.Get("/x", okHandler)
	app.Post("/x", okHandler)

	assert.Equal(t, http.StatusOK, get(t, app, "/x").StatusCode, "la API sigue respondiendo")
	assert.Equal(t, http.StatusOK, do(t, app, httptest.NewRequest(http.MethodPost, "/x", nil)).StatusCode)
	assert.Contains(t, logs.String(), "memoria por instancia")
	assert.Equal(t, 1, strings.Count(logs.String(), "Redis no responde"), "el aviso sale una vez, no por petición")

	limiter.Reset()
	assert.Equal(t, http.StatusOK, get(t, app, "/x").StatusCode)
}

func TestRateLimit_ConcurrentWritesOverLimitIs429(t *testing.T) {
	limiter := middleware.NewLimiter(testutil.DeadRedis(t), discardLogger())
	release := make(chan struct{})
	entered := make(chan struct{}, 1)
	app := newTestApp(true, discardLogger(), middleware.ProxyHeaders(1), limiter.Concurrent(middleware.PolicyWrites, 1))
	app.Get("/x", okHandler)
	app.Post("/x", func(c *fiber.Ctx) error {
		entered <- struct{}{}
		<-release
		return c.SendStatus(http.StatusNoContent)
	})

	done := make(chan int)
	go func() {
		resp, err := app.Test(httptest.NewRequest(http.MethodPost, "/x", nil), -1)
		if err == nil {
			_ = resp.Body.Close()
			done <- resp.StatusCode
		}
	}()
	<-entered
	second := do(t, app, httptest.NewRequest(http.MethodPost, "/x", nil))
	close(release)

	assert.Equal(t, http.StatusTooManyRequests, second.StatusCode)
	assert.Equal(t, http.StatusNoContent, <-done)
	assert.Equal(t, http.StatusOK, get(t, app, "/x").StatusCode, "GET no cuenta como escritura")
}

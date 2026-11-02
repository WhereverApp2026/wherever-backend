package middleware_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
)

// newTestApp monta Errors + RequestID + las etapas dadas, como en el pipeline real.
func newTestApp(showDetail bool, log *slog.Logger, handlers ...fiber.Handler) *fiber.App {
	errorHandler := middleware.NewErrorHandler(showDetail, log)
	app := fiber.New(fiber.Config{ErrorHandler: errorHandler, DisableStartupMessage: true})
	app.Use(middleware.Errors(errorHandler, log), middleware.RequestID())
	for _, h := range handlers {
		app.Use(h)
	}
	return app
}

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func do(t *testing.T, app *fiber.App, req *http.Request) *http.Response {
	t.Helper()
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func get(t *testing.T, app *fiber.App, path string) *http.Response {
	return do(t, app, httptest.NewRequest(http.MethodGet, path, nil))
}

// decode lee el cuerpo como mapa para revisar llaves exactas (snake_case, ausencias).
func decode(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	return body
}

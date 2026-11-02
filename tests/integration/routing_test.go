package integration

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRouting_V1ResolvesWithSnakeCase(t *testing.T) {
	resp := get(t, newApp("dev", realDeps()), "/api/v1/ping")
	body := decode(t, resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, map[string]any{"estado": "pong"}, body)
	assertSnakeCase(t, body)
}

func TestRouting_WithoutVersionIs404(t *testing.T) {
	app := newApp("dev", realDeps())

	for _, path := range []string{"/ping", "/api/ping", "/api/v2/ping", "/API/V1/ping", "/api/v1/ping/"} {
		resp := get(t, app, path)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode, path)
		assertSnakeCase(t, decode(t, resp))
	}
}

func TestRouting_DocsOnlyInDev(t *testing.T) {
	for env, want := range map[string]int{"dev": http.StatusOK, "staging": http.StatusNotFound, "prod": http.StatusNotFound} {
		app := newApp(env, realDeps())
		assert.Equal(t, want, get(t, app, "/docs").StatusCode, env+" /docs")
		assert.Equal(t, want, get(t, app, "/openapi.json").StatusCode, env+" /openapi.json")
	}
}

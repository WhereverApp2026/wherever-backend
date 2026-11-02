package integration

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Staging es no-prod (sondas activas, `detalle` visible) y se usa porque no hay servidor de staging.
func TestErrors_StatusesThroughRealApp(t *testing.T) {
	app := newApp("staging", realDeps())
	cases := []struct {
		path   string
		status int
		code   string
	}{
		{"/api/v1/no-existe", http.StatusNotFound, "no_encontrado"},
		{"/api/v1/probe/forbidden", http.StatusForbidden, "prohibido"},
		{"/api/v1/probe/conflict", http.StatusConflict, "conflicto"},
		{"/api/v1/probe/invalid", http.StatusUnprocessableEntity, "validacion"},
		{"/api/v1/probe/timeout", http.StatusRequestTimeout, "tiempo_agotado"},
		{"/api/v1/probe/error", http.StatusInternalServerError, "error_interno"},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			resp := get(t, app, tc.path)
			body := decode(t, resp)

			assert.Equal(t, tc.status, resp.StatusCode)
			assert.Equal(t, tc.code, body["codigo"])
			assert.Equal(t, resp.Header.Get("X-Request-ID"), body["traza_id"])
			assert.Contains(t, body, "campos")
			assertSnakeCase(t, body)
		})
	}
}

func TestErrors_DetailVisibleInStaging(t *testing.T) {
	body := decode(t, get(t, newApp("staging", realDeps()), "/api/v1/probe/error"))

	assert.Equal(t, "sonda de error interno", body["detalle"])
}

func TestErrors_ProdHidesDetailAndProbes(t *testing.T) {
	app := newApp("prod", realDeps())

	resp := get(t, app, "/api/v1/probe/error")
	body := decode(t, resp)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "sondas gateadas en prod")
	assert.NotContains(t, body, "detalle")
}

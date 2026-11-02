package middleware_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

func errorApp(showDetail bool, err error) *fiber.App {
	app := newTestApp(showDetail, discardLogger())
	app.Get("/x", func(*fiber.Ctx) error { return err })
	return app
}

// Tabla de constitución §5.
func TestErrorHandler_MapsEveryKind(t *testing.T) {
	cases := []struct {
		err    *apperr.Error
		status int
		code   string
	}{
		{apperr.BadRequest("x"), http.StatusBadRequest, "solicitud_invalida"},
		{apperr.Unauthorized("x"), http.StatusUnauthorized, "no_autenticado"},
		{apperr.Forbidden("x"), http.StatusForbidden, "prohibido"},
		{apperr.NotFound("x"), http.StatusNotFound, "no_encontrado"},
		{apperr.Timeout("x"), http.StatusRequestTimeout, "tiempo_agotado"},
		{apperr.Conflict("x"), http.StatusConflict, "conflicto"},
		{apperr.Gone("x"), http.StatusGone, "vencido"},
		{apperr.Validation("x"), http.StatusUnprocessableEntity, "validacion"},
		{apperr.Locked("x"), http.StatusLocked, "bloqueado"},
		{apperr.RateLimited(7), http.StatusTooManyRequests, "demasiadas_peticiones"},
		{apperr.Unavailable("x"), http.StatusServiceUnavailable, "no_disponible"},
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			resp := get(t, errorApp(true, tc.err), "/x")
			body := decode(t, resp)

			assert.Equal(t, tc.status, resp.StatusCode)
			assert.Equal(t, tc.code, body["codigo"])
			assert.NotEmpty(t, body["traza_id"])
			assert.Equal(t, resp.Header.Get(middleware.HeaderRequestID), body["traza_id"], "traza_id = request_id")
		})
	}
}

func TestErrorHandler_UnknownErrorIs500(t *testing.T) {
	resp := get(t, errorApp(true, errors.New("fallo de base")), "/x")
	body := decode(t, resp)

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.Equal(t, "error_interno", body["codigo"])
	assert.Equal(t, "error interno", body["mensaje"], "el mensaje público no filtra la causa")
}

func TestErrorHandler_FieldsNeverNullAndDetailOmittedWhenEmpty(t *testing.T) {
	resp := get(t, errorApp(true, apperr.NotFound("no está")), "/x")
	body := decode(t, resp)

	assert.Equal(t, []any{}, body["campos"])
	assert.NotContains(t, body, "detalle")
}

func TestErrorHandler_DetailHiddenInProd(t *testing.T) {
	resp := get(t, errorApp(false, errors.New("cadena interna")), "/x")
	body := decode(t, resp)

	assert.NotContains(t, body, "detalle")
}

func TestErrorHandler_DetailShownOutsideProd(t *testing.T) {
	resp := get(t, errorApp(true, errors.New("cadena interna")), "/x")
	body := decode(t, resp)

	assert.Equal(t, "cadena interna", body["detalle"])
}

func TestErrorHandler_ValidationFieldsAndRetryAfter(t *testing.T) {
	invalid := get(t, errorApp(true, apperr.Validation("x", apperr.FieldError{Field: "nombre", Message: "obligatorio"})), "/x")
	limited := get(t, errorApp(true, apperr.RateLimited(7)), "/x")

	fields, ok := decode(t, invalid)["campos"].([]any)
	require.True(t, ok)
	require.Len(t, fields, 1)
	assert.Equal(t, map[string]any{"campo": "nombre", "mensaje": "obligatorio"}, fields[0])
	assert.Equal(t, "7", limited.Header.Get("Retry-After"))
}

func TestErrors_RecoversPanicAs500(t *testing.T) {
	app := newTestApp(false, discardLogger())
	app.Get("/x", func(*fiber.Ctx) error { panic("boom") })

	resp := get(t, app, "/x")

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.Equal(t, "error_interno", decode(t, resp)["codigo"])
}

func TestErrors_UnknownRouteIs404Uniform(t *testing.T) {
	resp := get(t, newTestApp(false, discardLogger()), "/no-existe")
	body := decode(t, resp)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.Equal(t, "no_encontrado", body["codigo"])
	assert.Equal(t, "recurso no encontrado", body["mensaje"])
}

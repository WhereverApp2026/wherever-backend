package integration

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/moderacion"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
)

// PATCH /api/v1/admin/moderacion/reglas por HTTP (auditoría 2026-11-20: solo tenía prueba de
// seguridad y del service). Contrato en docs/endpoints.md: If-Match obligatorio, sin reglas
// guardadas = 404, versión vieja = 409, 40 ≤ cuarentena < rechazo ≤ 100, `palabras` reemplaza.
func TestModeracionReglasPatch_PorHTTP(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	admin := env.CreateAdmin(t, "admin-reglas-patch@wherever.test")
	token := testutil.ValidTokenFor(t, admin.ID.Hex(), "jti-admin-reglas-patch", middleware.RoleAdmin)
	path := moderacionPath + "/reglas"

	status, raw := sendIfMatch(t, app, http.MethodPatch, path, `"1"`, map[string]any{"cuarentena": 50}, token)
	assert.Equal(t, http.StatusNotFound, status, "sin reglas guardadas se crean con POST: "+raw)

	status, raw = sendIfMatch(t, app, http.MethodPost, path, "", map[string]any{"cuarentena": 45, "rechazo": 85, "palabras": []string{"vendo mota"}}, token)
	require.Equal(t, http.StatusCreated, status, raw)
	require.Equal(t, 1, reglasDe(t, raw).Version)

	status, raw = sendIfMatch(t, app, http.MethodPatch, path, "", map[string]any{"cuarentena": 50}, token)
	assert.Equal(t, http.StatusBadRequest, status, "sin If-Match: "+raw)
	status, raw = sendIfMatch(t, app, http.MethodPatch, path, `"1"`, map[string]any{"cuarentena": 90}, token)
	assert.Equal(t, http.StatusUnprocessableEntity, status, "cuarentena >= rechazo: "+raw)
	status, raw = sendIfMatch(t, app, http.MethodPatch, path, `"1"`, map[string]any{"rechazo": 101}, token)
	assert.Equal(t, http.StatusUnprocessableEntity, status, "rechazo sobre 100: "+raw)

	status, raw = sendIfMatch(t, app, http.MethodPatch, path, `"1"`, map[string]any{"cuarentena": 55, "palabras": []string{"Pulque adulterado", "pulque ADULTERADO"}}, token)
	require.Equal(t, http.StatusOK, status, raw)
	v := reglasDe(t, raw)
	assert.Equal(t, 2, v.Version)
	assert.Equal(t, 55.0, v.Cuarentena)
	assert.Equal(t, 85.0, v.Rechazo, "lo que no llega se queda igual")
	assert.Equal(t, []string{"Pulque adulterado"}, v.Palabras, "palabras reemplaza la lista y quita repetidas")
	assert.Equal(t, admin.ID.Hex(), v.ActualizadoPor)

	status, raw = sendIfMatch(t, app, http.MethodGet, path, "", nil, token)
	require.Equal(t, http.StatusOK, status, raw)
	assert.Equal(t, 2, reglasDe(t, raw).Version, "lo guardado es lo que se lee")

	m := moderacion.New(nil, nil, env.Moderacion, env.Log)
	assert.Equal(t, moderacion.VeredictoRechazado, m.Evaluar(context.Background(), moderacion.Contenido{Texto: "aquí venden pulque adulterado"}).Veredicto,
		"el Moderador usa las palabras nuevas sin reiniciar")
	assert.NotEqual(t, moderacion.VeredictoRechazado, m.Evaluar(context.Background(), moderacion.Contenido{Texto: "vendo mota barata"}).Veredicto,
		"la palabra que se quitó ya no rechaza")

	status, raw = sendIfMatch(t, app, http.MethodPatch, path, `"1"`, map[string]any{"cuarentena": 60}, token)
	assert.Equal(t, http.StatusConflict, status, "versión vieja = 409: "+raw)
}

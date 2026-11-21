package integration

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
)

// GET /negocios/{id}/sello/insignia (nv-76): solo con el Sello vigente y solo para su dueño.
func TestNegociosInsignia_SoloConSelloVigente(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	duena := env.CreateUser(t, "duena-insignia@wherever.test")
	b := env.AltaAprobada(t, duena, "Mezcalería Agave")
	token := merchantToken(t, duena.ID, "jti-insignia")
	admin := env.CreateAdmin(t, "admin-insignia@wherever.test")
	adminToken := testutil.ValidTokenFor(t, admin.ID.Hex(), "jti-admin-insignia", middleware.RoleAdmin)
	path := "/api/v1/negocios/" + b.ID + "/sello/insignia"
	sello := "/api/v1/admin/negocios/" + b.ID + "/sello"

	requireStatus(t, sendJSON(t, app, http.MethodGet, path, nil, token), http.StatusNotFound)

	requireStatus(t, sendJSON(t, app, http.MethodPut, sello, map[string]any{"otorgar": true, "motivo": "Cumple la regla del Sello"}, adminToken), http.StatusOK)

	resp := sendJSON(t, app, http.MethodGet, path, nil, token)
	requireStatus(t, resp, http.StatusOK)
	assert.Equal(t, "private, max-age=3600", resp.Header.Get("Cache-Control"))
	assert.NotEmpty(t, resp.Header.Get("ETag"))
	body := decode(t, resp)
	assert.Equal(t, b.ID, body["negocio_id"])
	assert.Equal(t, "Mezcalería Agave", body["nombre"])
	assert.Equal(t, float64(time.Now().UTC().Year()), body["anio"])
	assert.Contains(t, body["svg"], "Mezcalería Agave")
	assert.Contains(t, body["svg"], "<svg ")

	resp = sendJSON(t, app, http.MethodGet, path+"?formato=svg", nil, token)
	requireStatus(t, resp, http.StatusOK)
	assert.Equal(t, "image/svg+xml; charset=utf-8", resp.Header.Get("Content-Type"))
	assert.Contains(t, resp.Header.Get("Content-Disposition"), `attachment; filename="sello-`)
	svg := readAll(t, resp)
	assert.Equal(t, body["svg"], svg, "el SVG descargado es el mismo de la respuesta JSON")

	requireStatus(t, sendJSON(t, app, http.MethodGet, path+"?formato=png", nil, token), http.StatusUnprocessableEntity)

	// Otro comerciante no la ve: 404, como el resto del panel.
	otra := env.CreateUser(t, "otra-insignia@wherever.test")
	requireStatus(t, sendJSON(t, app, http.MethodGet, path, nil, merchantToken(t, otra.ID, "jti-otra-insignia")), http.StatusNotFound)

	// Retirado el Sello, la insignia deja de existir.
	requireStatus(t, sendJSON(t, app, http.MethodPut, sello, map[string]any{"otorgar": false, "motivo": "Reseñas bajo revisión"}, adminToken), http.StatusOK)
	resp = sendJSON(t, app, http.MethodGet, path, nil, token)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.Contains(t, readAll(t, resp), "Sello de Excelencia")
}

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/moderacion"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
)

// textoFijo es el proveedor de moderación de texto (interfaz externa) con puntajes fijos.
type textoFijo map[string]float64

func (f textoFijo) EvaluarTexto(context.Context, string) (map[string]float64, error) { return f, nil }

func reglasDe(t *testing.T, raw string) moderacion.ReglasView {
	t.Helper()
	var v moderacion.ReglasView
	require.NoError(t, json.Unmarshal([]byte(raw), &v), raw)
	return v
}

func TestModeracionReglasFiltro_UmbralesPropiosYDesactivar(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	admin := env.CreateAdmin(t, "admin-filtros@wherever.test")
	token := testutil.ValidTokenFor(t, admin.ID.Hex(), "jti-admin-filtros", middleware.RoleAdmin)
	path := moderacionPath + "/reglas"
	spam := path + "/spam"

	status, raw := sendIfMatch(t, app, http.MethodPatch, spam, `"1"`, map[string]any{"cuarentena": 70}, token)
	assert.Equal(t, http.StatusNotFound, status, "sin reglas guardadas = 404: "+raw)
	status, raw = sendIfMatch(t, app, http.MethodGet, path, "", nil, token)
	require.Equal(t, http.StatusOK, status, raw)
	def := reglasDe(t, raw)
	require.Len(t, def.Filtros, 8)
	assert.False(t, def.Filtros["spam"].Propio)

	status, raw = sendIfMatch(t, app, http.MethodPost, path, "", map[string]any{"cuarentena": 40, "rechazo": 80, "palabras": []string{}}, token)
	require.Equal(t, http.StatusCreated, status, raw)

	m := moderacion.New(nil, textoFijo{moderacion.FiltroSpam: 65, moderacion.FiltroViolencia: 90}, env.Moderacion, env.Log)
	texto := moderacion.Contenido{Texto: "oferta"}
	assert.Equal(t, moderacion.VeredictoRechazado, m.Evaluar(context.Background(), texto).Veredicto, "violencia 90 rechaza con los globales")

	status, raw = sendIfMatch(t, app, http.MethodPatch, spam, "", map[string]any{"cuarentena": 70}, token)
	assert.Equal(t, http.StatusBadRequest, status, raw)
	status, raw = sendIfMatch(t, app, http.MethodPatch, spam, `"1"`, map[string]any{"cuarentena": 85}, token)
	assert.Equal(t, http.StatusUnprocessableEntity, status, "cuarentena propia >= rechazo global: "+raw)
	status, raw = sendIfMatch(t, app, http.MethodPatch, path+"/palabra_bloqueada", `"1"`, map[string]any{"activa": false}, token)
	assert.Equal(t, http.StatusNotFound, status, raw)
	status, raw = sendIfMatch(t, app, http.MethodPatch, spam, `"1"`, map[string]any{"cuarentena": 70, "rechazo": 95}, token)
	require.Equal(t, http.StatusOK, status, raw)
	got := reglasDe(t, raw)
	assert.Equal(t, 2, got.Version)
	assert.Equal(t, moderacion.FiltroView{Cuarentena: 70, Rechazo: 95, Activa: true, Propio: true}, got.Filtros["spam"])
	assert.Equal(t, moderacion.FiltroView{Cuarentena: 40, Rechazo: 80, Activa: true}, got.Filtros["odio"], "hereda los globales")

	// Desactivar violencia: solo cuenta spam (65 < 70) y se aprueba.
	status, raw = sendIfMatch(t, app, http.MethodDelete, path+"/violencia", `"1"`, nil, token)
	assert.Equal(t, http.StatusConflict, status, "versión vieja: "+raw)
	status, raw = sendIfMatch(t, app, http.MethodDelete, path+"/violencia", `"2"`, nil, token)
	require.Equal(t, http.StatusNoContent, status, raw)
	assert.Equal(t, moderacion.VeredictoAprobado, m.Evaluar(context.Background(), texto).Veredicto)

	status, raw = sendIfMatch(t, app, http.MethodPatch, path+"/violencia", `"3"`, map[string]any{"activa": true}, token)
	require.Equal(t, http.StatusOK, status, raw)
	assert.Equal(t, moderacion.VeredictoRechazado, m.Evaluar(context.Background(), texto).Veredicto, "reactivada")

	// datos_personales no se desactiva.
	status, raw = sendIfMatch(t, app, http.MethodDelete, path+"/datos_personales", `"4"`, nil, token)
	assert.Equal(t, http.StatusUnprocessableEntity, status, raw)
	status, raw = sendIfMatch(t, app, http.MethodPatch, path+"/datos_personales", `"4"`, map[string]any{"activa": false}, token)
	assert.Equal(t, http.StatusUnprocessableEntity, status, raw)
	status, raw = sendIfMatch(t, app, http.MethodPatch, spam, `"4"`, map[string]any{}, token)
	assert.Equal(t, http.StatusUnprocessableEntity, status, "sin campos: "+raw)

	status, raw = sendIfMatch(t, app, http.MethodGet, path, "", nil, token)
	require.Equal(t, http.StatusOK, status, raw)
	final := reglasDe(t, raw)
	assert.True(t, final.Filtros["violencia"].Activa)
	assert.Equal(t, admin.ID.Hex(), final.ActualizadoPor)
}

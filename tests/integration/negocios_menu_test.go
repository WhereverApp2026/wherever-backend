package integration

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios/menu"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
)

func TestNegociosMenu_AltaEdicionOrdenYBusqueda(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	u := env.CreateUser(t, "duena-menu@wherever.test")
	b := env.AltaAprobada(t, u, "Menú Local")
	token := merchantToken(t, u.ID, "jti-menu")
	base := "/api/v1/negocios/" + b.ID + "/menu"
	public := "/api/v1/lugares/" + b.ID + "/menu"

	env.ImagenesNegocio.Publicar(u.ID, "foto-pastor")
	resp := sendJSON(t, app, http.MethodPost, base, map[string]any{"seccion": "Tacos", "nombre": "Pastor", "descripcion": "Con piña",
		"precio": 25, "etiquetas": []string{"picante"}, "imagen_id": "foto-pastor", "orden": 2}, token)
	requireStatus(t, resp, http.StatusCreated)
	pastor := decode(t, resp)
	assertSnakeCase(t, pastor)
	assert.NotNil(t, pastor["foto_url"])
	requireStatus(t, sendJSON(t, app, http.MethodPost, base, map[string]any{"seccion": "Bebidas", "nombre": "Café de olla", "precio": 30, "orden": 1}, token), http.StatusCreated)
	resp = sendJSON(t, app, http.MethodPost, base, map[string]any{"seccion": "Bebidas", "nombre": "Agua de jamaica", "precio": 20, "orden": 3}, token)
	requireStatus(t, resp, http.StatusCreated)
	agua := decode(t, resp)
	requireStatus(t, sendJSON(t, app, http.MethodPost, base, map[string]any{"seccion": "Tacos", "nombre": "Ajeno", "imagen_id": "de-otra"}, token), http.StatusUnprocessableEntity)

	resp = sendJSON(t, app, http.MethodGet, public, nil, "")
	requireStatus(t, resp, http.StatusOK)
	secciones := list(t, decode(t, resp)["secciones"])
	require.Len(t, secciones, 2)
	assert.Equal(t, "Bebidas", obj(t, secciones[0])["nombre"], "sección del primer platillo por orden")
	assert.Len(t, list(t, obj(t, secciones[0])["platillos"]), 2)

	// Editar con If-Match: versión vieja 409; disponible=false lo oculta.
	itemPath := base + "/" + str(t, agua["id"])
	requireStatus(t, sendJSON(t, app, http.MethodPatch, itemPath, map[string]any{"disponible": false}, token), http.StatusBadRequest)
	requireStatus(t, withIfMatch(t, app, http.MethodPatch, itemPath, map[string]any{"disponible": false}, token, 2), http.StatusConflict)
	requireStatus(t, withIfMatch(t, app, http.MethodPatch, itemPath, map[string]any{"disponible": false}, token, 1), http.StatusOK)
	resp = sendJSON(t, app, http.MethodGet, public+"?seccion=Bebidas", nil, "")
	requireStatus(t, resp, http.StatusOK)
	secciones = list(t, decode(t, resp)["secciones"])
	require.Len(t, secciones, 1)
	assert.Len(t, list(t, obj(t, secciones[0])["platillos"]), 1)

	// Búsqueda sin acentos ni mayúsculas con el índice de texto.
	resp = sendJSON(t, app, http.MethodGet, public+"?q=CAFE", nil, "")
	requireStatus(t, resp, http.StatusOK)
	secciones = list(t, decode(t, resp)["secciones"])
	require.Len(t, secciones, 1)
	assert.Equal(t, "Café de olla", obj(t, list(t, obj(t, secciones[0])["platillos"])[0])["nombre"])
	requireStatus(t, sendJSON(t, app, http.MethodGet, public+"?q=a", nil, ""), http.StatusUnprocessableEntity)

	// Borrar con If-Match: 204 sin cuerpo.
	resp = withIfMatch(t, app, http.MethodDelete, base+"/"+str(t, pastor["id"]), nil, token, 1)
	requireStatus(t, resp, http.StatusNoContent)
	assert.Empty(t, readAll(t, resp))
}

func TestNegociosMenu_TopeYNoAprobado(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	ctx := context.Background()
	u := env.CreateUser(t, "duena-tope@wherever.test")
	b := env.AltaAprobada(t, u, "Tope Local")
	token := merchantToken(t, u.ID, "jti-tope")
	for i := range menu.MaxItems {
		_, err := env.Menu.Crear(ctx, authtest.Comerciante(u), b.ID, menu.CrearRequest{Seccion: "Tacos", Nombre: fmt.Sprintf("Taco %d", i)})
		require.NoError(t, err)
	}
	resp := sendJSON(t, app, http.MethodPost, "/api/v1/negocios/"+b.ID+"/menu", map[string]any{"seccion": "Tacos", "nombre": "Uno más"}, token)
	requireStatus(t, resp, http.StatusConflict)
	assert.Contains(t, readAll(t, resp), "limite_menu")

	// Un negocio en revisión no tiene menú público.
	otra := env.AltaEnRevision(t, u, "En Revisión")
	requireStatus(t, sendJSON(t, app, http.MethodGet, "/api/v1/lugares/"+otra.ID+"/menu", nil, ""), http.StatusNotFound)
}

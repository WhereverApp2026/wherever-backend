package integration

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
)

// nv-90: experiencias y año en el paso 1, WhatsApp y redes en el paso 2.
func TestNegociosDatos_CapturaEnElAlta(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	u := env.CreateUser(t, "duena-datos-alta@wherever.test")
	access, _ := loginHTTP(t, app, u.Email)

	resp := sendJSON(t, app, http.MethodPost, altaPath, map[string]any{"nombre": "Fonda Datos", "categoria": "restaurante",
		"experiencias": []string{"comida", "familia"}, "anio_apertura": 2999}, access)
	requireStatus(t, resp, http.StatusUnprocessableEntity)

	resp = sendJSON(t, app, http.MethodPost, altaPath, map[string]any{"nombre": "Fonda Datos", "categoria": "restaurante",
		"experiencias": []string{"comida", "familia"}, "anio_apertura": 1998}, access)
	requireStatus(t, resp, http.StatusCreated)
	body := decode(t, resp)
	assert.Equal(t, []any{"comida", "familia"}, body["experiencias"])
	assert.Equal(t, float64(1998), body["anio_apertura"])
	id := str(t, body["id"])
	access, _ = loginHTTP(t, app, u.Email)

	paso2 := map[string]any{
		"direccion": map[string]any{"calle": "Juárez", "numero": "10", "colonia": "Centro", "municipio": "Cuauhtémoc", "estado": "CDMX", "cp": "06000"},
		"ubicacion": map[string]any{"type": "Point", "coordinates": []float64{-99.1332, 19.4326}},
		"whatsapp":  "+52 55 1234 5678",
		"redes":     map[string]any{"instagram": "fonda.datos", "sitio_web": "https://fonda-datos.mx"},
	}
	paso2["whatsapp"] = "5512345678"
	requireStatus(t, withIfMatch(t, app, http.MethodPatch, altaPath+"/"+id+"/alta/2", paso2, access, 1), http.StatusUnprocessableEntity)
	paso2["whatsapp"] = "+52 55 1234 5678"
	resp = withIfMatch(t, app, http.MethodPatch, altaPath+"/"+id+"/alta/2", paso2, access, 1)
	requireStatus(t, resp, http.StatusOK)
	body = decode(t, resp)
	assertSnakeCase(t, body)
	assert.Equal(t, "+525512345678", body["whatsapp"])
	assert.Equal(t, "fonda.datos", obj(t, body["redes"])["instagram"])
}

// nv-90: con el negocio publicado los cuatro datos esperan al administrador; al aprobarse, la
// ficha los muestra (WhatsApp solo como enlace) y `experiencia=` filtra.
func TestNegociosDatos_CambioPendienteFichaYFiltro(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	u := env.CreateUser(t, "duena-datos-perfil@wherever.test")
	b := env.AltaAprobada(t, u, "Cena Romántica")
	token := merchantToken(t, u.ID, "jti-datos-perfil")
	admin := env.CreateAdmin(t, "admin-datos@wherever.test")
	adminToken := testutil.ValidTokenFor(t, admin.ID.Hex(), "jti-admin-datos", middleware.RoleAdmin)

	cambios := map[string]any{"experiencias": []string{"cena", "romantico"}, "anio_apertura": 2010, "whatsapp": "+527731234567",
		"redes": map[string]any{"instagram": "cena_romantica"}}
	resp := withIfMatch(t, app, http.MethodPatch, "/api/v1/negocios/"+b.ID, cambios, token, b.Version)
	requireStatus(t, resp, http.StatusOK)
	body := decode(t, resp)
	assert.Nil(t, body["experiencias"], "esperan revisión")
	assert.ElementsMatch(t, []any{"experiencias", "anio_apertura", "whatsapp", "redes"}, obj(t, body["cambios_pendientes"])["campos"])
	version := versionDe(t, body)

	ficha := sendJSON(t, app, http.MethodGet, "/api/v1/lugares/"+b.ID, nil, "")
	requireStatus(t, ficha, http.StatusOK)
	assert.NotContains(t, decode(t, ficha), "whatsapp_url", "nada sale antes de aprobar")

	resp = sendJSON(t, app, http.MethodGet, "/api/v1/admin/negocios?estado=cambios_pendientes", nil, adminToken)
	requireStatus(t, resp, http.StatusOK)
	items := list(t, decode(t, resp)["items"])
	require.Len(t, items, 1)
	assert.Equal(t, "+527731234567", obj(t, obj(t, items[0])["cambios_pendientes"])["whatsapp"], "el administrador ve el número")

	resp = withIfMatch(t, app, http.MethodPatch, "/api/v1/admin/negocios/"+b.ID+"/revision", map[string]any{"decision": "aprobar"}, adminToken, version)
	requireStatus(t, resp, http.StatusOK)

	ficha = sendJSON(t, app, http.MethodGet, "/api/v1/lugares/"+b.ID, nil, "")
	requireStatus(t, ficha, http.StatusOK)
	detalle := decode(t, ficha)
	assertSnakeCase(t, detalle)
	assert.Equal(t, []any{"cena", "romantico"}, detalle["experiencias"])
	assert.Equal(t, float64(2010), detalle["anio_apertura"])
	assert.Equal(t, "https://wa.me/527731234567", detalle["whatsapp_url"])
	assert.NotContains(t, detalle, "whatsapp", "el número solo sale como enlace")
	assert.Equal(t, "cena_romantica", obj(t, detalle["redes"])["instagram"])

	const cerca = "/api/v1/lugares?lat=19.4326&lng=-99.1332&limite=50"
	resp = sendJSON(t, app, http.MethodGet, cerca+"&experiencia=romantico", nil, "")
	requireStatus(t, resp, http.StatusOK)
	assert.Len(t, list(t, decode(t, resp)["items"]), 1)
	resp = sendJSON(t, app, http.MethodGet, cerca+"&experiencia=desayuno", nil, "")
	requireStatus(t, resp, http.StatusOK)
	assert.Empty(t, list(t, decode(t, resp)["items"]))
	requireStatus(t, sendJSON(t, app, http.MethodGet, cerca+"&experiencia=brunch", nil, ""), http.StatusUnprocessableEntity)
}

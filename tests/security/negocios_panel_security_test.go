package security

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
)

func TestNegociosPanelSecurity_AuthRolesYDuenoAjeno(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	duena := env.CreateUser(t, "duena-panel-seg@wherever.test")
	b := env.AltaAprobada(t, duena, "Panel Seguro")
	merchantToken := testutil.ValidTokenFor(t, duena.ID.Hex(), "jti-duena-panel-seg", middleware.RoleConsumer, middleware.RoleMerchant)
	otra := env.CreateUser(t, "otra-panel-seg@wherever.test")
	otraToken := testutil.ValidTokenFor(t, otra.ID.Hex(), "jti-otra-panel-seg", middleware.RoleConsumer, middleware.RoleMerchant)
	consumerToken := testutil.ValidTokenFor(t, otra.ID.Hex(), "jti-consumidora-panel-seg", middleware.RoleConsumer)
	admin := env.CreateAdmin(t, "admin-panel-seg@wherever.test")
	adminToken := testutil.ValidTokenFor(t, admin.ID.Hex(), "jti-admin-panel-seg", middleware.RoleAdmin)

	base := "/api/v1/negocios/" + b.ID
	owner := []struct{ method, path string }{
		{http.MethodGet, base + "/panel"},
		{http.MethodPatch, base},
		{http.MethodPost, base + "/pausa"},
		{http.MethodPut, base + "/reservas-config"},
		{http.MethodPost, base + "/menu"},
		{http.MethodPatch, base + "/menu/x"},
		{http.MethodDelete, base + "/menu/x"},
		{http.MethodGet, base + "/qr"},
		{http.MethodDelete, base},
	}
	sello := "/api/v1/admin/negocios/" + b.ID + "/sello"
	for _, r := range append(owner, struct{ method, path string }{http.MethodPut, sello}) {
		assert.Equal(t, http.StatusUnauthorized, send(t, app, r.method, r.path, nil, "").status, r.path)
	}
	for _, r := range owner {
		assert.Equal(t, http.StatusForbidden, send(t, app, r.method, r.path, nil, consumerToken).status, "consumidor: "+r.path)
		assert.Equal(t, http.StatusForbidden, send(t, app, r.method, r.path, nil, adminToken).status, "administrador sin rol de dueño: "+r.path)
	}
	// El Sello solo con administrador.
	body := map[string]any{"otorgar": true, "motivo": "Intento sin permiso"}
	assert.Equal(t, http.StatusForbidden, send(t, app, http.MethodPut, sello, body, merchantToken).status)
	assert.Equal(t, http.StatusForbidden, send(t, app, http.MethodPut, sello, body, consumerToken).status)

	// Un negocio ajeno da 404 para no confirmar que existe.
	assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodGet, base+"/panel", nil, otraToken).status)
	assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodGet, base+"/qr", nil, otraToken).status)
	assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodPost, base+"/pausa", map[string]any{"pausado": true}, otraToken).status)
	assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodPost, base+"/menu", map[string]any{"seccion": "Tacos", "nombre": "Ajeno"}, otraToken).status)
	assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodDelete, base, map[string]any{"confirmacion": "Panel Seguro"}, otraToken).status)

	assert.Equal(t, http.StatusOK, send(t, app, http.MethodGet, base+"/panel", nil, merchantToken).status)
}

func TestNegociosPanelSecurity_PanelNoExponeElExpediente(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	duena := env.CreateUser(t, "duena-panel-priv@wherever.test")
	b := env.AltaAprobada(t, duena, "Panel Privado")
	token := testutil.ValidTokenFor(t, duena.ID.Hex(), "jti-panel-priv", middleware.RoleConsumer, middleware.RoleMerchant)
	base := "/api/v1/negocios/" + b.ID
	require.Equal(t, http.StatusOK, send(t, app, http.MethodPut, base+"/datos-fiscales",
		map[string]string{"rfc": "ABC680524P76", "razon_social": "Panel Privado SA", "regimen_fiscal": "601", "uso_cfdi": "G03", "cp_fiscal": "06600"}, token).status)
	require.Equal(t, http.StatusOK, send(t, app, http.MethodPut, base+"/cuenta-deposito",
		map[string]string{"clabe": "012180000118359713", "titular": "Duena Privada"}, token).status)

	r := send(t, app, http.MethodGet, base+"/panel", nil, token)
	require.Equal(t, http.StatusOK, r.status, r.body)
	assert.Contains(t, r.body, `"datos_fiscales_completos":true`)
	assert.Contains(t, r.body, `"clabe_ultimos4":"9713"`)
	for _, secreto := range []string{"ABC680524P76", "Panel Privado SA", "012180000118359713", "Duena Privada", "ine_frente", "privado/", "titular"} {
		assert.False(t, strings.Contains(r.body, secreto), "el panel no trae "+secreto)
	}
}

package security

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios/alta"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
)

func TestNegociosAltaSecurity_AuthRolesYDuenoAjeno(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	duena := env.CreateUser(t, "duena-seg@wherever.test")
	b := env.AltaEnRevision(t, duena, "Local Seguro")
	merchantToken := testutil.ValidTokenFor(t, duena.ID.Hex(), "jti-duena-seg", middleware.RoleConsumer, middleware.RoleMerchant)
	otra := env.CreateUser(t, "otra-seg@wherever.test")
	otraToken := testutil.ValidTokenFor(t, otra.ID.Hex(), "jti-otra-seg", middleware.RoleConsumer, middleware.RoleMerchant)
	consumerToken := testutil.ValidTokenFor(t, otra.ID.Hex(), "jti-consumidora-seg", middleware.RoleConsumer)
	admin := env.CreateAdmin(t, "admin-seg-alta@wherever.test")
	adminToken := testutil.ValidTokenFor(t, admin.ID.Hex(), "jti-admin-seg-alta", middleware.RoleAdmin)

	base := "/api/v1/negocios/" + b.ID
	owner := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/negocios/mios"},
		{http.MethodPatch, base + "/alta/3"},
		{http.MethodPost, base + "/fotos"},
		{http.MethodPut, base + "/fotos/orden"},
		{http.MethodDelete, base + "/fotos/x"},
		{http.MethodPost, base + "/documentos"},
		{http.MethodGet, base + "/documentos"},
		{http.MethodGet, base + "/documentos/ine_frente"},
		{http.MethodPut, base + "/datos-fiscales"},
		{http.MethodPut, base + "/cuenta-deposito"},
		{http.MethodPost, base + "/enviar-revision"},
	}
	adminRoutes := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/admin/negocios"},
		{http.MethodGet, "/api/v1/admin/negocios/" + b.ID + "/documentos/ine_frente?motivo=Revision+del+alta"},
		{http.MethodPatch, "/api/v1/admin/negocios/" + b.ID + "/revision"},
	}
	for _, r := range append(append([]struct{ method, path string }{{http.MethodPost, "/api/v1/negocios"}}, owner...), adminRoutes...) {
		assert.Equal(t, http.StatusUnauthorized, send(t, app, r.method, r.path, nil, "").status, r.path)
	}
	for _, r := range owner {
		assert.Equal(t, http.StatusForbidden, send(t, app, r.method, r.path, nil, consumerToken).status, "consumidor: "+r.path)
		assert.Equal(t, http.StatusForbidden, send(t, app, r.method, r.path, nil, adminToken).status, "administrador sin rol de dueño: "+r.path)
	}
	for _, r := range adminRoutes {
		assert.Equal(t, http.StatusForbidden, send(t, app, r.method, r.path, nil, merchantToken).status, "comerciante: "+r.path)
		assert.Equal(t, http.StatusForbidden, send(t, app, r.method, r.path, nil, consumerToken).status, "consumidor: "+r.path)
	}

	// Un negocio ajeno da 404 para no confirmar que existe.
	for _, path := range []string{base + "/documentos", base + "/documentos/ine_frente"} {
		assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodGet, path, nil, otraToken).status, path)
	}
	assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodPost, base+"/enviar-revision", nil, otraToken).status)
	assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodPut, base+"/cuenta-deposito", map[string]string{"clabe": "012180000118359713", "titular": "Otra"}, otraToken).status)

	// El dueño sí lee lo suyo.
	assert.Equal(t, http.StatusOK, send(t, app, http.MethodGet, base+"/documentos", nil, merchantToken).status)
}

func TestNegociosAltaSecurity_LugaresNuncaTraenElExpediente(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	duena := env.CreateUser(t, "duena-lugares@wherever.test")
	b := env.AltaEnRevision(t, duena, "Local Publico")
	merchantToken := testutil.ValidTokenFor(t, duena.ID.Hex(), "jti-duena-lugares", middleware.RoleConsumer, middleware.RoleMerchant)
	require.Equal(t, http.StatusOK, send(t, app, http.MethodPut, "/api/v1/negocios/"+b.ID+"/datos-fiscales",
		map[string]string{"rfc": "ABC680524P76", "razon_social": "Publico SA", "regimen_fiscal": "601", "uso_cfdi": "G03", "cp_fiscal": "06600"}, merchantToken).status)
	require.Equal(t, http.StatusOK, send(t, app, http.MethodPut, "/api/v1/negocios/"+b.ID+"/cuenta-deposito",
		map[string]string{"clabe": "012180000118359713", "titular": "Duena Publica"}, merchantToken).status)

	// En revisión no se ve en el catálogo.
	assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodGet, "/api/v1/lugares/"+b.ID, nil, "").status)

	admin := env.CreateAdmin(t, "admin-lugares@wherever.test")
	adminToken := testutil.ValidTokenFor(t, admin.ID.Hex(), "jti-admin-lugares", middleware.RoleAdmin)
	decision := send(t, app, http.MethodPatch, "/api/v1/admin/negocios/"+b.ID+"/revision", map[string]string{"decision": "aprobar"}, adminToken)
	assert.Equal(t, http.StatusBadRequest, decision.status, "sin If-Match")
	_, err := env.Alta.Decidir(t.Context(), authtest.ClaimsDe(admin.ID.Hex(), middleware.RoleAdmin), b.ID, b.Version, alta.DecisionRequest{Decision: alta.DecisionAprobar})
	require.NoError(t, err)

	for _, path := range []string{"/api/v1/lugares/" + b.ID, "/api/v1/lugares?lat=19.4326&lng=-99.1332&radio_m=5000"} {
		got := send(t, app, http.MethodGet, path, nil, "")
		require.Equal(t, http.StatusOK, got.status, got.body)
		for _, secreto := range []string{"documentos", "datos_fiscales", "cuenta_deposito", "clabe", "ABC680524P76", "Duena Publica", "privado/"} {
			assert.False(t, strings.Contains(got.body, secreto), "%s filtra %s", path, secreto)
		}
	}
}

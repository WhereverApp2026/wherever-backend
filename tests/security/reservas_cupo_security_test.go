package security

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios/panel"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/reservastest"
)

// TestCupoSecurity_AuthRolesYDuenoAjeno: el cupo por hora (nv-88) es solo del dueño; un negocio
// ajeno da 404 y no cambia.
func TestCupoSecurity_AuthRolesYDuenoAjeno(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	duena := env.CreateUser(t, "duena-cupo-seg@wherever.test")
	b := reservastest.Negocio(t, env.Mongo.DB(), duena.ID, reservastest.Config{Capacidad: 10})
	duenaToken := testutil.ValidTokenFor(t, duena.ID.Hex(), "jti-duena-cupo-seg", middleware.RoleConsumer, middleware.RoleMerchant)
	otra := env.CreateUser(t, "otra-cupo-seg@wherever.test")
	otraToken := testutil.ValidTokenFor(t, otra.ID.Hex(), "jti-otra-cupo-seg", middleware.RoleConsumer, middleware.RoleMerchant)
	consumidorToken := testutil.ValidTokenFor(t, otra.ID.Hex(), "jti-consumidora-cupo-seg", middleware.RoleConsumer)

	ruta := "/api/v1/negocios/" + b.ID.Hex() + "/cupo"
	cuerpo := map[string]any{"cupo_por_hora": []map[string]any{{"dia": 1, "hora": "14:00", "lugares": 1}}}
	for _, m := range []string{http.MethodGet, http.MethodPut} {
		assert.Equal(t, http.StatusUnauthorized, send(t, app, m, ruta, cuerpo, "").status, m)
		assert.Equal(t, http.StatusForbidden, send(t, app, m, ruta, cuerpo, consumidorToken).status, "consumidor: "+m)
	}
	assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodGet, ruta, nil, otraToken).status)
	// El PUT ajeno con If-Match correcto llega al service y también es 404.
	dia, hora, lugares := 1, "14:00", 1
	_, err := env.Panel.CambiarCupo(t.Context(), authtest.Comerciante(otra), b.ID.Hex(), 1,
		panel.CupoRequest{CupoPorHora: []panel.CupoHoraRequest{{Dia: &dia, Hora: hora, Lugares: &lugares}}})
	var ae *apperr.Error
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, apperr.KindNotFound, ae.Kind)

	// La tabla de la dueña sigue intacta.
	r := send(t, app, http.MethodGet, ruta, nil, duenaToken)
	require.Equal(t, http.StatusOK, r.status)
	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(r.body), &got))
	assert.EqualValues(t, 1, got["version"], "el PUT ajeno no cambió nada")
}

package security

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
)

// nv-90: un comerciante no edita los datos de un negocio ajeno; `javascript:` y `data:` en el
// sitio dan 422; la ficha solo expone el enlace de WhatsApp, nunca el número ni datos de la
// cuenta del dueño.
func TestNegociosDatosSecurity_PropiedadEsquemasYFicha(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	duena := env.CreateUser(t, "duena-datos-seg@wherever.test")
	b := env.AltaAprobada(t, duena, "Datos Seguros")
	duenaToken := testutil.ValidTokenFor(t, duena.ID.Hex(), "jti-duena-datos-seg", middleware.RoleConsumer, middleware.RoleMerchant)
	otra := env.CreateUser(t, "otra-datos-seg@wherever.test")
	otraToken := testutil.ValidTokenFor(t, otra.ID.Hex(), "jti-otra-datos-seg", middleware.RoleConsumer, middleware.RoleMerchant)
	path := "/api/v1/negocios/" + b.ID

	assert.Equal(t, http.StatusUnauthorized, sendIfMatch(t, app, http.MethodPatch, path, map[string]any{"anio_apertura": 2000}, "").status)
	ajeno := sendIfMatch(t, app, http.MethodPatch, path, map[string]any{"whatsapp": "+527731234567"}, otraToken)
	assert.Equal(t, http.StatusNotFound, ajeno.status, "negocio ajeno: 404 sin confirmar que existe")

	for _, sitio := range []string{"javascript:alert(1)", "data:text/html,<script>alert(1)</script>", "http://datos.mx", "https://bit.ly/x"} {
		r := sendIfMatch(t, app, http.MethodPatch, path, map[string]any{"redes": map[string]any{"sitio_web": sitio}}, duenaToken)
		assert.Equal(t, http.StatusUnprocessableEntity, r.status, sitio)
	}

	// Aplicado directo (como si el administrador ya lo aprobó): la ficha no trae el número.
	oid, err := bson.ObjectIDFromHex(b.ID)
	require.NoError(t, err)
	_, err = env.Mongo.DB().Collection(negocios.Collection).UpdateByID(context.Background(), oid,
		bson.D{{Key: "$set", Value: bson.D{{Key: "whatsapp", Value: "+527739876543"}}}})
	require.NoError(t, err)
	env.Negocios.InvalidarFicha(context.Background(), b.ID)
	ficha := send(t, app, http.MethodGet, "/api/v1/lugares/"+b.ID, nil, "")
	require.Equal(t, http.StatusOK, ficha.status)
	assert.Contains(t, ficha.body, `"whatsapp_url":"https://wa.me/527739876543"`)
	assert.NotContains(t, ficha.body, `"whatsapp":`)
	assert.NotContains(t, ficha.body, "+527739876543")
	assert.NotContains(t, ficha.body, "propietario")
	assert.NotContains(t, ficha.body, duena.Email)
}

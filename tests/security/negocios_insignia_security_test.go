package security

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
)

// otorgar deja el Sello vigente como lo guarda PUT /admin/negocios/{id}/sello (nv-56).
func otorgar(t *testing.T, db *mongo.Database, id string) {
	t.Helper()
	oid, err := bson.ObjectIDFromHex(id)
	require.NoError(t, err)
	_, err = db.Collection(negocios.Collection).UpdateByID(context.Background(), oid, bson.D{{Key: "$set", Value: bson.D{
		{Key: "sello_excelencia", Value: true},
		{Key: "sello", Value: negocios.Sello{Otorgado: true, OtorgadoPor: bson.NewObjectID(), OtorgadoEn: time.Now().UTC(), Motivo: "Cumple la regla del Sello"}},
	}}})
	require.NoError(t, err)
}

// Insignia del Sello (nv-76): sesión, rol de dueño, solo la propia y sin marcado inyectado.
func TestNegociosInsigniaSecurity_AuthRolYDueno(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	duena := env.CreateUser(t, "duena-insignia-seg@wherever.test")
	b := env.AltaAprobada(t, duena, "Insignia Segura")
	otra := env.CreateUser(t, "otra-insignia-seg@wherever.test")
	path := "/api/v1/negocios/" + b.ID + "/sello/insignia"
	otorgar(t, env.Mongo.DB(), b.ID)

	for name, token := range map[string]string{
		"sin token": "",
		"vencido":   testutil.ExpiredToken(t),
		"sin firma": testutil.UnsignedToken(t),
		"alg none":  testutil.AlgNoneToken(t),
	} {
		assert.Equal(t, http.StatusUnauthorized, send(t, app, http.MethodGet, path, nil, token).status, name)
	}
	consumidor := testutil.ValidTokenFor(t, duena.ID.Hex(), "jti-consumidora-insignia-seg", middleware.RoleConsumer)
	assert.Equal(t, http.StatusForbidden, send(t, app, http.MethodGet, path, nil, consumidor).status, "sin rol de comerciante")
	ajena := testutil.ValidTokenFor(t, otra.ID.Hex(), "jti-otra-insignia-seg", middleware.RoleMerchant)
	r := send(t, app, http.MethodGet, path, nil, ajena)
	assert.Equal(t, http.StatusNotFound, r.status, "el negocio de otra persona no existe para ella")
	assert.NotContains(t, r.body, "Insignia Segura")
	for _, id := range []string{"no-es-id", "%7B%22%24ne%22%3Anull%7D", bson.NewObjectID().Hex()} {
		assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodGet, "/api/v1/negocios/"+id+"/sello/insignia", nil, ajena).status, id)
	}
	propia := testutil.ValidTokenFor(t, duena.ID.Hex(), "jti-duena-insignia-seg", middleware.RoleMerchant)
	assert.Equal(t, http.StatusOK, send(t, app, http.MethodGet, path, nil, propia).status)
}

// Un nombre de negocio con marcado sale escapado en el SVG, que además lleva la CSP estricta.
func TestNegociosInsigniaSecurity_NombreSinMarcadoYCSP(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	duena := env.CreateUser(t, "duena-xss-insignia@wherever.test")
	b := env.AltaAprobada(t, duena, "Café Seguro")
	oid, err := bson.ObjectIDFromHex(b.ID)
	require.NoError(t, err)
	const malicioso = `<script>alert(1)</script>" onload="x`
	_, err = env.Mongo.DB().Collection(negocios.Collection).UpdateByID(context.Background(), oid, bson.D{{Key: "$set", Value: bson.D{{Key: "nombre", Value: malicioso}}}})
	require.NoError(t, err)
	otorgar(t, env.Mongo.DB(), b.ID)
	token := testutil.ValidTokenFor(t, duena.ID.Hex(), "jti-duena-xss-insignia", middleware.RoleMerchant)

	r := send(t, app, http.MethodGet, "/api/v1/negocios/"+b.ID+"/sello/insignia?formato=svg", nil, token)
	require.Equal(t, http.StatusOK, r.status, r.body)
	assert.NotContains(t, r.body, "<script")
	assert.NotContains(t, r.body, `" onload="`)
	assert.Contains(t, r.body, "&lt;script&gt;")
	assert.Contains(t, r.headers.Get("Content-Security-Policy"), "default-src 'none'")
	assert.Equal(t, "nosniff", r.headers.Get("X-Content-Type-Options"))
}

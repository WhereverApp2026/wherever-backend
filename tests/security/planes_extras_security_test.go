package security

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Alternativas, cambiar parada y el paquete piden sesión (nv-68); populares no, y no da
// datos de personas aunque no haya planes.
func TestPlanesExtrasSecurity_SesionYPopularesSinDatos(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	id, stop := bson.NewObjectID().Hex(), bson.NewObjectID().Hex()
	for _, r := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/planes/" + id + "/paradas/" + stop + "/alternativas"},
		{http.MethodPost, "/api/v1/planes/" + id + "/paradas/" + stop + "/cambiar"},
		{http.MethodGet, "/api/v1/planes/" + id + "/paquete-offline"},
	} {
		assert.Equal(t, http.StatusUnauthorized, send(t, app, r.method, r.path, map[string]string{"negocio_id": id}, "").status, r.path)
	}
	got := send(t, app, http.MethodGet, "/api/v1/planes/populares?lat=19.4326123&lng=-99.1332456", nil, "")
	assert.Equal(t, http.StatusOK, got.status, got.body)
	assert.NotContains(t, got.body, "dueno_id")
	assert.NotContains(t, env.Logs.String(), "19.4326123", "el punto exacto no llega a los logs")
}

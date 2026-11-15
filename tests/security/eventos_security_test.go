package security

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/moderacion"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
)

func eventoSeguro(negocio string) map[string]any {
	inicio := time.Now().Add(24 * time.Hour)
	return map[string]any{
		"titulo": "Feria", "tipo": "feria", "direccion": "Centro", "negocio_id": negocio,
		"ubicacion": map[string]float64{"lat": 19.43, "lng": -99.13},
		"inicio":    inicio.Format(time.RFC3339), "fin": inicio.Add(time.Hour).Format(time.RFC3339), "precio": 0,
	}
}

// Escrituras sin sesión = 401; consumidor no publica; otro comerciante no edita ni borra; el
// borrador ajeno es 404; texto rechazado = 422.
func TestEventosSecurity_RolesYPropiedad(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	id := bson.NewObjectID().Hex()
	for _, r := range [][2]string{{http.MethodPost, "/api/v1/eventos"}, {http.MethodPatch, "/api/v1/eventos/" + id}, {http.MethodDelete, "/api/v1/eventos/" + id}} {
		assert.Equal(t, http.StatusUnauthorized, sendIfMatch(t, app, r[0], r[1], map[string]string{"titulo": "x"}, "").status, r[0]+" "+r[1])
	}
	duena := env.CreateUser(t, "eventos-sec-duena@wherever.test")
	b := env.AltaAprobada(t, duena, "Eventos seguridad")
	owner := testutil.ValidTokenFor(t, duena.ID.Hex(), "jti-eventos-sec-duena", middleware.RoleConsumer, middleware.RoleMerchant)
	consumer := testutil.ValidTokenFor(t, env.CreateUser(t, "eventos-sec-cons@wherever.test").ID.Hex(), "jti-eventos-sec-cons", middleware.RoleConsumer)
	otra := env.CreateUser(t, "eventos-sec-otra@wherever.test")
	env.AltaAprobada(t, otra, "Otro seguridad")
	other := testutil.ValidTokenFor(t, otra.ID.Hex(), "jti-eventos-sec-otra", middleware.RoleConsumer, middleware.RoleMerchant)

	assert.Equal(t, http.StatusForbidden, send(t, app, http.MethodPost, "/api/v1/eventos", eventoSeguro(b.ID), consumer).status)
	borrador := eventoSeguro(b.ID)
	borrador["estado"] = "borrador"
	created := send(t, app, http.MethodPost, "/api/v1/eventos", borrador, owner)
	require.Equal(t, http.StatusCreated, created.status, created.body)
	path := created.headers.Get("Location")
	assert.Equal(t, http.StatusForbidden, sendIfMatch(t, app, http.MethodPatch, path, map[string]string{"titulo": "Mío"}, other).status)
	assert.Equal(t, http.StatusForbidden, sendIfMatch(t, app, http.MethodDelete, path, nil, other).status)
	assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodGet, path, nil, other).status, "borrador ajeno")
	assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodGet, path, nil, "").status)
	assert.NotContains(t, created.body, duena.ID.Hex(), "creado_por no sale")

	env.ModeradorEventos.Fijar(moderacion.VeredictoRechazado)
	t.Cleanup(func() { env.ModeradorEventos.Fijar(moderacion.VeredictoAprobado) })
	assert.Equal(t, http.StatusUnprocessableEntity, send(t, app, http.MethodPost, "/api/v1/eventos", eventoSeguro(b.ID), owner).status)
}

package security

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/reservastest"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

type rutaCupon struct {
	method, path string
	body         any
	negocio      bool
}

func rutasCupones(negocio, id string) []rutaCupon {
	base := "/api/v1/negocios/" + negocio + "/cupones"
	return []rutaCupon{
		{http.MethodPost, base, map[string]any{"titulo": "x", "tipo": "regalo", "relampago": true}, true},
		{http.MethodGet, base, nil, true},
		{http.MethodGet, base + "/" + id + "/estadisticas", nil, true},
		{http.MethodPatch, base + "/" + id, map[string]any{"titulo": "y"}, true},
		{http.MethodDelete, base + "/" + id, nil, true},
		{http.MethodPost, base + "/sugerir-titulo", map[string]any{"tipo": "regalo"}, true},
		{http.MethodPost, base + "/validar", map[string]any{"codigo": "ABC234"}, true},
		{http.MethodPost, "/api/v1/cupones/" + id + "/guardar", nil, false},
		{http.MethodGet, "/api/v1/mis-cupones", nil, false},
		{http.MethodPost, "/api/v1/mis-cupones/" + id + "/canjear", nil, false},
		{http.MethodDelete, "/api/v1/mis-cupones/" + id, nil, false},
	}
}

// Las once rutas con token piden sesión (401), cuenta activa (403) y el rol (403); las del
// negocio exigen comerciante y las de la cartera, consumidor.
func TestCuponesSecurity_SesionRolYCuentaActiva(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	rutas := rutasCupones(bson.NewObjectID().Hex(), bson.NewObjectID().Hex())

	pending := env.CreateUser(t, "pendiente-cup@wherever.test")
	_, err := env.Users.Update(context.Background(), pending.ID, bson.D{{Key: "estado", Value: usuarios.StatusPendingDeletion}})
	require.NoError(t, err)
	pendiente := testutil.ValidTokenFor(t, pending.ID.Hex(), "jti-pendiente-cup", middleware.RoleConsumer, middleware.RoleMerchant)
	c := env.CreateUser(t, "consumidor-cup@wherever.test")
	soloConsumidor := testutil.ValidTokenFor(t, c.ID.Hex(), "jti-consumidor-cup", middleware.RoleConsumer)
	m := env.CreateUser(t, "comerciante-cup@wherever.test")
	soloComerciante := testutil.ValidTokenFor(t, m.ID.Hex(), "jti-comerciante-cup", middleware.RoleMerchant)

	for _, r := range rutas {
		assert.Equal(t, http.StatusUnauthorized, send(t, app, r.method, r.path, r.body, "").status, r.method+" "+r.path)
		assert.Equal(t, http.StatusForbidden, send(t, app, r.method, r.path, r.body, pendiente).status, r.method+" "+r.path)
		otro := soloComerciante
		if r.negocio {
			otro = soloConsumidor
		}
		assert.Equal(t, http.StatusForbidden, send(t, app, r.method, r.path, r.body, otro).status, "rol: "+r.method+" "+r.path)
	}
	// cerca y el detalle son públicos.
	assert.Equal(t, http.StatusOK, send(t, app, http.MethodGet, "/api/v1/cupones/cerca?lat=19.43&lng=-99.13", nil, "").status)
	assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodGet, "/api/v1/cupones/"+bson.NewObjectID().Hex(), nil, "").status)
}

// Validar códigos al azar se frena en 60 por 10 minutos por usuario del negocio.
func TestCuponesSecurity_ValidarLimitado(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	duena := env.CreateUser(t, "duena-lim-sec-cup@wherever.test")
	b := reservastest.Negocio(t, env.Mongo.DB(), duena.ID, reservastest.Config{})
	token := testutil.ValidTokenFor(t, duena.ID.Hex(), "jti-duena-lim-cup", middleware.RoleConsumer, middleware.RoleMerchant)
	path := "/api/v1/negocios/" + b.ID.Hex() + "/cupones/validar"
	for range 60 {
		require.Equal(t, http.StatusConflict, send(t, app, http.MethodPost, path, map[string]any{"codigo": "ZZZZZZ"}, token).status)
	}
	assert.Equal(t, http.StatusTooManyRequests, send(t, app, http.MethodPost, path, map[string]any{"codigo": "ZZZZZZ"}, token).status)
}

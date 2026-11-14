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
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

type rutaPostal struct {
	method, path string
	body         any
	rol          string // el rol que exige; vacío = cualquier sesión
}

// rutasPostales son las 15 rutas de nv-63 con sesión obligatoria (la galería y el detalle de un
// momento tienen sesión opcional).
func rutasPostales(negocio, id string) []rutaPostal {
	c, m := middleware.RoleConsumer, middleware.RoleMerchant
	ifm := map[string]any{"texto": "x"}
	return []rutaPostal{
		{http.MethodPost, "/api/v1/postales", map[string]any{"imagen_id": "ab"}, c},
		{http.MethodGet, "/api/v1/postales", nil, ""},
		{http.MethodGet, "/api/v1/postales/" + id, nil, ""},
		{http.MethodPatch, "/api/v1/postales/" + id, ifm, ""},
		{http.MethodDelete, "/api/v1/postales/" + id, nil, ""},
		{http.MethodGet, "/api/v1/recuerdos", nil, ""},
		{http.MethodPost, "/api/v1/planes/" + id + "/recuerdo", nil, ""},
		{http.MethodGet, "/api/v1/planes/" + id + "/recuerdo", nil, ""},
		{http.MethodDelete, "/api/v1/recuerdos/" + id, nil, ""},
		{http.MethodPost, "/api/v1/momentos", map[string]any{"postal_id": id}, c},
		{http.MethodGet, "/api/v1/momentos/feed", nil, ""},
		{http.MethodPatch, "/api/v1/momentos/" + id, ifm, ""},
		{http.MethodDelete, "/api/v1/momentos/" + id, nil, ""},
		{http.MethodGet, "/api/v1/negocios/" + negocio + "/momentos", nil, m},
		{http.MethodPatch, "/api/v1/negocios/" + negocio + "/momentos/" + id, map[string]any{"destacado": true}, m},
	}
}

// Sin token = 401; cuenta en eliminación pendiente = 403; rol equivocado = 403. Las dos rutas de
// sesión opcional responden 404 a un id que no existe, con o sin token: no revelan nada.
func TestPostalesSecurity_SesionRolYCuentaActiva(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	rutas := rutasPostales(bson.NewObjectID().Hex(), bson.NewObjectID().Hex())

	pending := env.CreateUser(t, "pendiente-postal@wherever.test")
	_, err := env.Users.Update(context.Background(), pending.ID, bson.D{{Key: "estado", Value: usuarios.StatusPendingDeletion}})
	require.NoError(t, err)
	pendiente := testutil.ValidTokenFor(t, pending.ID.Hex(), "jti-pendiente-postal", middleware.RoleConsumer, middleware.RoleMerchant)
	c := env.CreateUser(t, "consumidor-postal@wherever.test")
	soloConsumidor := testutil.ValidTokenFor(t, c.ID.Hex(), "jti-consumidor-postal", middleware.RoleConsumer)
	m := env.CreateUser(t, "comerciante-postal@wherever.test")
	soloComerciante := testutil.ValidTokenFor(t, m.ID.Hex(), "jti-comerciante-postal", middleware.RoleMerchant)

	for _, r := range rutas {
		assert.Equal(t, http.StatusUnauthorized, send(t, app, r.method, r.path, r.body, "").status, r.method+" "+r.path)
		assert.Equal(t, http.StatusForbidden, send(t, app, r.method, r.path, r.body, pendiente).status, r.method+" "+r.path)
		switch r.rol {
		case middleware.RoleConsumer:
			assert.Equal(t, http.StatusForbidden, send(t, app, r.method, r.path, r.body, soloComerciante).status, "rol: "+r.method+" "+r.path)
		case middleware.RoleMerchant:
			assert.Equal(t, http.StatusForbidden, send(t, app, r.method, r.path, r.body, soloConsumidor).status, "rol: "+r.method+" "+r.path)
		}
	}
	for _, token := range []string{"", soloConsumidor} {
		assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodGet, "/api/v1/lugares/"+bson.NewObjectID().Hex()+"/postales", nil, token).status)
		assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodGet, "/api/v1/momentos/"+bson.NewObjectID().Hex(), nil, token).status)
	}
	// Postal y recuerdo ajenos o inexistentes = 404, igual que por visibilidad.
	assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodGet, "/api/v1/postales/"+bson.NewObjectID().Hex(), nil, soloConsumidor).status)
	assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodGet, "/api/v1/planes/"+bson.NewObjectID().Hex()+"/recuerdo", nil, soloConsumidor).status)
}

package security

import (
	"context"
	"encoding/json"
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

type rutaResena struct {
	method, path string
	body         any
	rol          string // el rol que exige; vacío = cualquier sesión
}

func rutasResenas(negocio, id string) []rutaResena {
	base := "/api/v1/negocios/" + negocio + "/resenas/"
	c, m := middleware.RoleConsumer, middleware.RoleMerchant
	return []rutaResena{
		{http.MethodPost, "/api/v1/checkins", map[string]any{"negocio_id": negocio, "metodo": "qr_negocio", "codigo": "x"}, c},
		{http.MethodGet, "/api/v1/checkins", nil, c},
		{http.MethodGet, "/api/v1/resenas/pendientes", nil, c},
		{http.MethodGet, "/api/v1/resenas/mias", nil, c},
		{http.MethodPost, "/api/v1/resenas", map[string]any{"checkin_id": id}, c},
		{http.MethodPatch, "/api/v1/resenas/" + id, map[string]any{"calificacion": 3}, c},
		{http.MethodDelete, "/api/v1/resenas/" + id, nil, c},
		{http.MethodPut, "/api/v1/resenas/" + id + "/util", map[string]any{"util": true}, ""},
		{http.MethodPost, "/api/v1/resenas/" + id + "/reportes", map[string]any{"motivo": "spam"}, ""},
		{http.MethodGet, base + "resumen", nil, m},
		{http.MethodPut, base + id + "/respuesta", map[string]any{"texto": "Gracias"}, m},
		{http.MethodPost, base + id + "/sugerir-respuesta", nil, m},
	}
}

// Las doce rutas con token piden sesión (401) y cuenta activa (403); las de la persona exigen
// consumidor y las del negocio, comerciante.
func TestResenasSecurity_SesionRolYCuentaActiva(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	rutas := rutasResenas(bson.NewObjectID().Hex(), bson.NewObjectID().Hex())

	pending := env.CreateUser(t, "pendiente-res@wherever.test")
	_, err := env.Users.Update(context.Background(), pending.ID, bson.D{{Key: "estado", Value: usuarios.StatusPendingDeletion}})
	require.NoError(t, err)
	pendiente := testutil.ValidTokenFor(t, pending.ID.Hex(), "jti-pendiente-res", middleware.RoleConsumer, middleware.RoleMerchant)
	c := env.CreateUser(t, "consumidor-res@wherever.test")
	soloConsumidor := testutil.ValidTokenFor(t, c.ID.Hex(), "jti-consumidor-res", middleware.RoleConsumer)
	m := env.CreateUser(t, "comerciante-res@wherever.test")
	soloComerciante := testutil.ValidTokenFor(t, m.ID.Hex(), "jti-comerciante-res", middleware.RoleMerchant)

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
	// La lista pública no pide sesión; negocio inexistente = 404.
	assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodGet, "/api/v1/lugares/"+bson.NewObjectID().Hex()+"/resenas", nil, "").status)
}

// El check-in se frena en 10 por hora y los reportes en 20 por día, por usuario.
func TestResenasSecurity_Limites(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	duena := env.CreateUser(t, "duena-lim-res@wherever.test")
	b := reservastest.Negocio(t, env.Mongo.DB(), duena.ID, reservastest.Config{})
	u := env.CreateUser(t, "cli-lim-res@wherever.test")
	token := testutil.ValidTokenFor(t, u.ID.Hex(), "jti-cli-lim-res", middleware.RoleConsumer)
	lejos := map[string]any{"negocio_id": b.ID.Hex(), "metodo": "ubicacion", "lat": 19.5, "lng": -99.1332, "precision_m": 10}
	for range 10 {
		require.Equal(t, http.StatusUnprocessableEntity, send(t, app, http.MethodPost, "/api/v1/checkins", lejos, token).status)
	}
	assert.Equal(t, http.StatusTooManyRequests, send(t, app, http.MethodPost, "/api/v1/checkins", lejos, token).status)

	path := "/api/v1/resenas/" + bson.NewObjectID().Hex() + "/reportes"
	for range 20 {
		require.Equal(t, http.StatusNotFound, send(t, app, http.MethodPost, path, map[string]any{"motivo": "spam"}, token).status)
	}
	assert.Equal(t, http.StatusTooManyRequests, send(t, app, http.MethodPost, path, map[string]any{"motivo": "spam"}, token).status)
}

// La lista pública nunca expone correo, teléfono, el check-in ni el id de la persona.
func TestResenasSecurity_ListaPublicaSinDatosPersonales(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	duena := env.CreateUser(t, "duena-pub-res@wherever.test")
	b := reservastest.Negocio(t, env.Mongo.DB(), duena.ID, reservastest.Config{})
	u := env.CreateUser(t, "cli-pub-res@wherever.test")
	token := testutil.ValidTokenFor(t, u.ID.Hex(), "jti-cli-pub-res", middleware.RoleConsumer)
	r := send(t, app, http.MethodPost, "/api/v1/checkins", map[string]any{"negocio_id": b.ID.Hex(), "metodo": "qr_negocio", "codigo": b.QRCheckin}, token)
	require.Equal(t, http.StatusCreated, r.status)
	var creado struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal([]byte(r.body), &creado))
	checkin := creado.ID
	r = send(t, app, http.MethodPost, "/api/v1/resenas", map[string]any{
		"checkin_id": checkin, "calificacion": 5, "comentario": "Excelente",
		"aspectos": map[string]any{"sabor": 5, "atencion": 5, "ambiente": 5, "espera": 5, "precio_calidad": 5},
	}, token)
	require.Equal(t, http.StatusCreated, r.status)

	r = send(t, app, http.MethodGet, "/api/v1/lugares/"+b.ID.Hex()+"/resenas", nil, "")
	require.Equal(t, http.StatusOK, r.status)
	raw := r.body
	for _, prohibido := range []string{"cli-pub-res@wherever.test", u.ID.Hex(), checkin, "checkin_id", "correo", "telefono"} {
		assert.NotContains(t, raw, prohibido)
	}
}

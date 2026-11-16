package security

import (
	"context"
	"net/http"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/reservastest"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// Consejo y descripción: sesión (401), cuenta activa y rol comerciante (403) y dueño ajeno (404).
func TestConsejosSecurity_SesionRolYDuenoAjeno(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	duena := env.CreateUser(t, "duena-consejo-seg@wherever.test")
	b := reservastest.Negocio(t, env.Mongo.DB(), duena.ID, reservastest.Config{})
	duenaTok := testutil.ValidTokenFor(t, duena.ID.Hex(), "jti-duena-consejo-seg", middleware.RoleConsumer, middleware.RoleMerchant)
	otra := env.CreateUser(t, "otra-consejo-seg@wherever.test")
	otraTok := testutil.ValidTokenFor(t, otra.ID.Hex(), "jti-otra-consejo-seg", middleware.RoleMerchant)
	consumidor := testutil.ValidTokenFor(t, otra.ID.Hex(), "jti-cons-consejo-seg", middleware.RoleConsumer)
	pending := env.CreateUser(t, "pendiente-consejo-seg@wherever.test")
	_, err := env.Users.Update(context.Background(), pending.ID, bson.D{{Key: "estado", Value: usuarios.StatusPendingDeletion}})
	require.NoError(t, err)
	pendiente := testutil.ValidTokenFor(t, pending.ID.Hex(), "jti-pend-consejo-seg", middleware.RoleMerchant)

	base := "/api/v1/negocios/" + b.ID.Hex()
	rutas := []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, base + "/consejo", nil},
		{http.MethodPost, base + "/ia/descripcion", map[string]any{"palabras": []string{"tacos", "salsa", "cerveza"}}},
	}
	for _, r := range rutas {
		assert.Equal(t, http.StatusUnauthorized, send(t, app, r.method, r.path, r.body, "").status, r.path)
		assert.Equal(t, http.StatusForbidden, send(t, app, r.method, r.path, r.body, consumidor).status, "consumidor: "+r.path)
		assert.Equal(t, http.StatusForbidden, send(t, app, r.method, r.path, r.body, pendiente).status, "cuenta no activa: "+r.path)
		assert.Equal(t, http.StatusNotFound, send(t, app, r.method, r.path, r.body, otraTok).status, "ajeno: "+r.path)
		assert.Equal(t, http.StatusOK, send(t, app, r.method, r.path, r.body, duenaTok).status, "dueña: "+r.path)
	}
	assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodGet, "/api/v1/negocios/no-es-id/consejo", nil, duenaTok).status)
}

// Al modelo no le llegan nombres, correos, ids ni frases de reseñas, y los logs no llevan el
// contexto mandado ni la salida.
func TestConsejosSecurity_PrivacidadDelContextoYLogs(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	duena := env.CreateUser(t, "duena-consejo-priv@wherever.test")
	b := reservastest.Negocio(t, env.Mongo.DB(), duena.ID, reservastest.Config{})
	token := testutil.ValidTokenFor(t, duena.ID.Hex(), "jti-duena-consejo-priv", middleware.RoleMerchant)
	cliente := bson.NewObjectID()
	frase := "La mesera Rocío Pérez fue grosera, llámenme al 7731234567"
	_, err := env.Mongo.DB().Collection("resenas").InsertOne(context.Background(), bson.D{{Key: "_id", Value: bson.NewObjectID()}, {Key: "negocio_id", Value: b.ID},
		{Key: "usuario_id", Value: cliente}, {Key: "comentario", Value: frase}, {Key: "estado", Value: "publicada"}, {Key: "creado_en", Value: time.Now().UTC().Add(-time.Hour)}})
	require.NoError(t, err)
	salida := "Responde hoy tu reseña pendiente: una respuesta amable genera confianza."
	env.IA.Responder(`{"texto":"` + salida + `","accion":"responder_resenas"}`)

	r := send(t, app, http.MethodGet, "/api/v1/negocios/"+b.ID.Hex()+"/consejo", nil, token)
	require.Equal(t, http.StatusOK, r.status)
	env.IA.Responder(`{"descripcion":"Restaurante con tacos al pastor y salsas de la casa, ideal para compartir con amigos."}`)
	r = send(t, app, http.MethodPost, "/api/v1/negocios/"+b.ID.Hex()+"/ia/descripcion",
		map[string]any{"palabras": []string{"tacos", "Rocío", "rocio@correo.mx"}}, token)
	require.Equal(t, http.StatusUnprocessableEntity, r.status, "un correo no pasa como palabra")
	r = send(t, app, http.MethodPost, "/api/v1/negocios/"+b.ID.Hex()+"/ia/descripcion",
		map[string]any{"palabras": []string{"tacos", "salsas", "amigos"}}, token)
	require.Equal(t, http.StatusOK, r.status)

	peticiones := env.IA.Peticiones()
	require.Len(t, peticiones, 2)
	hex := regexp.MustCompile(`[0-9a-fA-F]{24}`)
	for _, p := range peticiones {
		todo := p.Contexto + " " + p.TextoUsuario
		for _, prohibido := range []string{b.Name, "duena-consejo-priv", "Rocío", "Pérez", "7731234567", "grosera", "@"} {
			assert.NotContains(t, todo, prohibido)
		}
		assert.False(t, hex.MatchString(todo), "sin ids de 24 hex")
	}
	assert.Empty(t, peticiones[0].TextoUsuario, "el consejo no manda texto del usuario")

	logs := env.Logs.String()
	assert.NotContains(t, logs, peticiones[0].Contexto)
	assert.NotContains(t, logs, salida)
	assert.NotContains(t, logs, "tacos al pastor")
}

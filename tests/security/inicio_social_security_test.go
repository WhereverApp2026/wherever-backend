package security

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/postales"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

var rutasInicio84 = []string{"/api/v1/planes/amigos", "/api/v1/momentos/historias", "/api/v1/votaciones/pendientes"}

// planSembrado84 inserta un plan activo de dueño con la visibilidad dada, en tres días.
func planSembrado84(t *testing.T, env *authtest.Env, dueno bson.ObjectID, nombre, vis string) planes.Plan {
	t.Helper()
	now := time.Now().UTC()
	p := planes.Plan{ID: bson.NewObjectID(), OwnerID: dueno, Nombre: nombre, Fecha: now.AddDate(0, 0, 3), Personas: 2, Movilidad: planes.MobilityWalk,
		Paradas: []planes.Parada{}, Traslados: []planes.Traslado{}, Estado: planes.StatusActive, Origen: planes.OriginManual, Version: 1,
		Visibilidad: vis, Zona: &planes.GeoPoint{Type: "Point", Coordinates: []float64{-99.34, 20.05}}, CreadoEn: now, ActualizadoEn: now}
	_, err := env.Mongo.DB().Collection(planes.Collection).InsertOne(context.Background(), p)
	require.NoError(t, err)
	return p
}

// Sin token = 401 y con la cuenta en eliminación pendiente = 403 en las 3 rutas de nv-84.
func TestInicioSocialSecurity_SesionYCuentaActiva(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	pending := env.CreateUser(t, "pendiente-ini84@wherever.test")
	_, err := env.Users.Update(context.Background(), pending.ID, bson.D{{Key: "estado", Value: usuarios.StatusPendingDeletion}})
	require.NoError(t, err)
	pendiente := testutil.ValidTokenFor(t, pending.ID.Hex(), "jti-pendiente-ini84", middleware.RoleConsumer)
	for _, r := range rutasInicio84 {
		assert.Equal(t, http.StatusUnauthorized, send(t, app, http.MethodGet, r, nil, "").status, r)
		assert.Equal(t, http.StatusForbidden, send(t, app, http.MethodGet, r, nil, pendiente).status, r)
	}
}

// Nadie ve un plan o una historia que su autor no compartió con él: privado, de no amigo,
// bloqueado, `actividad: nadie` o `compartir_planes=false`; y sin integrantes ni ubicación.
func TestInicioSocialSecurity_SoloLoCompartido(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	db := env.Mongo.DB()
	yo := env.CreateUser(t, "yo-ini84@wherever.test")
	ana := env.CreateUser(t, "ana-ini84@wherever.test")
	extrana := env.CreateUser(t, "extrana-ini84@wherever.test")
	bloq := env.CreateUser(t, "bloq-ini84@wherever.test")
	relacionar(t, env, yo.ID, ana.ID, amistades.StatusAccepted)
	relacionar(t, env, bloq.ID, yo.ID, amistades.StatusBlocked)
	tok := testutil.ValidTokenFor(t, yo.ID.Hex(), "jti-yo-ini84", middleware.RoleConsumer)

	compartido := planSembrado84(t, env, ana.ID, "Compartido", planes.VisibilityFriends)
	privado := planSembrado84(t, env, ana.ID, "Privado", planes.VisibilityPrivate)
	ajeno := planSembrado84(t, env, extrana.ID, "De extraña", planes.VisibilityFriends)
	bloqueado := planSembrado84(t, env, bloq.ID, "Bloqueado", planes.VisibilityPublic)
	momentoSembrado(t, env, extrana.ID, postales.VisPublica)
	momentoSembrado(t, env, bloq.ID, postales.VisPublica)
	momentoSembrado(t, env, ana.ID, postales.VisPrivada)

	out := send(t, app, http.MethodGet, "/api/v1/planes/amigos", nil, tok)
	require.Equal(t, http.StatusOK, out.status, out.body)
	assert.Contains(t, out.body, compartido.ID.Hex(), "la prueba no pasa por un dato mal sembrado")
	for _, p := range []planes.Plan{privado, ajeno, bloqueado} {
		assert.NotContains(t, out.body, p.ID.Hex(), p.Nombre)
		assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodGet, "/api/v1/planes/"+p.ID.Hex(), nil, tok).status, p.Nombre)
	}
	for _, campo := range []string{"integrantes", "zona", "coordinates", "correo", "telefono", "@wherever.test"} {
		assert.NotContains(t, out.body, campo)
	}
	hist := send(t, app, http.MethodGet, "/api/v1/momentos/historias", nil, tok)
	require.Equal(t, http.StatusOK, hist.status, hist.body)
	assert.JSONEq(t, `{"personas":[]}`, hist.body, "privado, extraña y bloqueada no salen")

	for _, cambio := range []bson.E{{Key: "privacidad.social.compartir_planes", Value: false}, {Key: "privacidad.social.actividad", Value: usuarios.ActivityNobody}} {
		_, err := db.Collection("usuarios").UpdateOne(context.Background(), bson.D{{Key: "_id", Value: ana.ID}}, bson.D{{Key: "$set", Value: bson.D{cambio}}})
		require.NoError(t, err)
		out = send(t, app, http.MethodGet, "/api/v1/planes/amigos", nil, tok)
		require.Equal(t, http.StatusOK, out.status, out.body)
		assert.NotContains(t, out.body, compartido.ID.Hex(), cambio.Key)
		assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodGet, "/api/v1/planes/"+compartido.ID.Hex(), nil, tok).status, cambio.Key)
		_, err = db.Collection("usuarios").UpdateOne(context.Background(), bson.D{{Key: "_id", Value: ana.ID}}, bson.D{{Key: "$unset", Value: bson.D{{Key: cambio.Key, Value: ""}}}})
		require.NoError(t, err)
	}
}

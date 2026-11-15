package security

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/grupos"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// extrasGroup siembra un plan activo de mañana con una parada y su grupo: organizador y dos
// integrantes confirmados.
func extrasGroup(t *testing.T, env *authtest.Env, prefix string) (grupos.Group, planes.Plan, []string) {
	t.Helper()
	org := env.CreateUser(t, prefix+"-org@wherever.test")
	a := env.CreateUser(t, prefix+"-a@wherever.test")
	b := env.CreateUser(t, prefix+"-b@wherever.test")
	now := time.Now().UTC()
	day := now.AddDate(0, 0, 1)
	p := planes.Plan{
		ID: bson.NewObjectID(), OwnerID: org.ID, Nombre: "Secreto", Fecha: day, Personas: 3, Movilidad: planes.MobilityWalk,
		Paradas: []planes.Parada{{ID: bson.NewObjectID(), NegocioID: bson.NewObjectID(), Llega: day, Minutos: 60}},
		Estado:  planes.StatusActive, Origen: planes.OriginManual, Version: 1, CreadoEn: now, ActualizadoEn: now,
	}
	_, err := env.Mongo.DB().Collection(planes.Collection).InsertOne(context.Background(), p)
	require.NoError(t, err)
	g := grupos.Group{
		ID: bson.NewObjectID(), PlanID: p.ID, OrganizerID: org.ID, Occupied: 3, CreatedAt: now, UpdatedAt: now,
		Members: []grupos.Member{
			{UserID: org.ID, Role: grupos.RoleOrganizer, Status: grupos.StatusConfirmed, Since: now},
			{UserID: a.ID, Role: grupos.RoleMember, Status: grupos.StatusConfirmed, Since: now},
			{UserID: b.ID, Role: grupos.RoleMember, Status: grupos.StatusConfirmed, Since: now},
		},
	}
	require.NoError(t, grupos.NewRepository(env.Mongo.DB()).Create(context.Background(), &g))
	tokens := []string{}
	for i, u := range []bson.ObjectID{org.ID, a.ID, b.ID} {
		tokens = append(tokens, testutil.ValidTokenFor(t, u.Hex(), prefix+"-jti-"+string(rune('0'+i)), middleware.RoleConsumer))
	}
	return g, p, tokens
}

// Las rutas de nv-69 piden sesión; las escrituras, además, cuenta activa.
func TestGruposExtrasSecurity_SesionYCuentaActiva(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	id := bson.NewObjectID().Hex()
	g := "/api/v1/grupos/" + id
	writes := []struct {
		method, path string
		body         any
	}{
		{http.MethodPatch, g, map[string]string{"nombre": "x"}},
		{http.MethodDelete, g, nil},
		{http.MethodPut, g + "/punto-encuentro", map[string]any{"nombre": "x", "lat": 1, "lng": 1, "hora": "2026-11-20T12:00:00Z"}},
		{http.MethodPut, g + "/sorpresa", map[string]any{"activo": false}},
		{http.MethodPost, "/api/v1/planes/" + id + "/me-apunto", nil},
		{http.MethodDelete, "/api/v1/planes/" + id + "/me-apunto", nil},
	}
	pending := env.CreateUser(t, "pendiente-extras@wherever.test")
	_, err := env.Users.Update(context.Background(), pending.ID, bson.D{{Key: "estado", Value: usuarios.StatusPendingDeletion}})
	require.NoError(t, err)
	token := testutil.ValidTokenFor(t, pending.ID.Hex(), "jti-pendiente-extras", middleware.RoleConsumer)
	for _, r := range writes {
		assert.Equal(t, http.StatusUnauthorized, send(t, app, r.method, r.path, r.body, "").status, r.method+" "+r.path)
		assert.Equal(t, http.StatusForbidden, send(t, app, r.method, r.path, r.body, token).status, r.method+" "+r.path)
	}
	for _, path := range []string{g + "/punto-encuentro/sugerido", g + "/punto-encuentro/traslados"} {
		assert.Equal(t, http.StatusUnauthorized, send(t, app, http.MethodGet, path, nil, "").status, path)
	}
}

// Un integrante no renombra, no disuelve ni activa la sorpresa (403).
func TestGruposExtrasSecurity_SoloElOrganizador(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	g, _, tokens := extrasGroup(t, env, "solo-org")
	path := "/api/v1/grupos/" + g.ID.Hex()
	member := tokens[1]
	assert.Equal(t, http.StatusForbidden, sendIfMatch(t, app, http.MethodPatch, path, map[string]string{"nombre": "Mío"}, member).status)
	assert.Equal(t, http.StatusForbidden, sendIfMatch(t, app, http.MethodDelete, path, nil, member).status)
	assert.Equal(t, http.StatusForbidden, send(t, app, http.MethodPut, path+"/sorpresa", map[string]any{"activo": false}, member).status)
}

// La ubicación compartida no sale en ninguna respuesta ni en los logs, y la sorpresa no se
// filtra por las lecturas del plan.
func TestGruposExtrasSecurity_UbicacionYSorpresaNoSeFiltran(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	g, p, tokens := extrasGroup(t, env, "fuga")
	path := "/api/v1/grupos/" + g.ID.Hex()
	exact := map[string]any{"ubicacion": map[string]float64{"lat": 19.4326123, "lng": -99.1332456}}
	bodies := []string{
		send(t, app, http.MethodPatch, path+"/integrantes/yo", exact, tokens[1]).body,
		send(t, app, http.MethodPatch, path+"/integrantes/yo", map[string]any{"ubicacion": map[string]float64{"lat": 19.4411111, "lng": -99.1422222}}, tokens[2]).body,
		send(t, app, http.MethodGet, path, nil, tokens[0]).body,
		send(t, app, http.MethodGet, path+"/punto-encuentro/sugerido", nil, tokens[0]).body,
		send(t, app, http.MethodGet, "/api/v1/grupos", nil, tokens[1]).body,
	}
	for _, body := range bodies {
		for _, leak := range []string{"ubicacion_compartida", "19.4326123", "19.433", "19.441", "-99.142"} {
			assert.NotContains(t, body, leak)
		}
	}
	for _, leak := range []string{"19.4326123", "19.4411111", "-99.1332456"} {
		assert.NotContains(t, env.Logs.String(), leak, "las coordenadas no llegan a los logs")
	}

	reveal := time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339)
	if _, end := dayEnd(p.Fecha); time.Now().Add(2 * time.Hour).After(end) {
		reveal = end.Add(-time.Minute).UTC().Format(time.RFC3339)
	}
	set := send(t, app, http.MethodPut, path+"/sorpresa", map[string]any{"activo": true, "para_usuario_id": g.Members[1].UserID.Hex(), "revelar_en": reveal}, tokens[0])
	require.Equal(t, http.StatusOK, set.status, set.body)
	stop := p.Paradas[0]
	for _, read := range []string{"/api/v1/planes/" + p.ID.Hex(), "/api/v1/planes/" + p.ID.Hex() + "/paquete-offline", path, path + "/votaciones"} {
		got := send(t, app, http.MethodGet, read, nil, tokens[1])
		// El lugar de la primera parada sí puede salir como punto (spec); la parada, no.
		assert.NotContains(t, got.body, stop.ID.Hex(), read)
		assert.NotContains(t, got.body, `"paradas":[{`, read)
	}
}

// dayEnd es el fin del día del plan en la hora de México.
func dayEnd(fecha time.Time) (time.Time, time.Time) {
	loc := time.FixedZone("MX", -6*3600)
	f := fecha.In(loc)
	start := time.Date(f.Year(), f.Month(), f.Day(), 0, 0, 0, 0, loc)
	return start, start.AddDate(0, 0, 1)
}

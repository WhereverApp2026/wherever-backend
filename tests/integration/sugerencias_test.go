package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/neo4j"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

const sugerenciasPath = "/api/v1/amistades/sugerencias"

// grafoEnv es amistadesEnv con Neo4j real conectado a las sugerencias (nv-53).
func grafoEnv(t *testing.T) (*authtest.Env, *fiber.App) {
	t.Helper()
	env, _ := amistadesEnv(t)
	emptyGraph(t)
	env.Sugerencias.WithGraph(neo4jDriver)
	return env, env.App()
}

// relacion guarda en Mongo una relación de a con b en estado (en bloqueada, a bloqueó).
func relacion(t *testing.T, env *authtest.Env, a, b usuarios.User, estado string) {
	t.Helper()
	now := time.Now().UTC()
	f := amistades.Friendship{ID: bson.NewObjectID(), From: a.ID, To: b.ID, Status: estado, Pair: amistades.PairKey(a.ID, b.ID), CreatedAt: now, UpdatedAt: now}
	require.NoError(t, amistades.NewRepository(env.Mongo.DB()).Insert(context.Background(), &f))
}

// proyectar deja Neo4j igual a Mongo, como `make reconstruir-grafo`.
func proyectar(t *testing.T, env *authtest.Env) {
	t.Helper()
	_, err := grafoWorker(env, neo4jDriver).Rebuild(context.Background(), env.Mongo.DB())
	require.NoError(t, err)
}

func fueA(t *testing.T, who usuarios.User, negocio string) {
	t.Helper()
	cypher(t, "MERGE (u:Usuario {id: $u}) MERGE (n:Negocio {id: $n}) MERGE (u)-[:FUE_A]->(n)", map[string]any{"u": who.ID.Hex(), "n": negocio})
}

// sugeridos devuelve los nombres de usuario sugeridos con sus amigos en común.
func sugeridos(t *testing.T, app *fiber.App, who amigo) ([]string, []float64, bool) {
	t.Helper()
	resp := sendJSON(t, app, http.MethodGet, sugerenciasPath, nil, who.token)
	assert.Equal(t, "no-store", resp.Header.Get(fiber.HeaderCacheControl))
	body := decodeOK(t, resp)
	names, common := []string{}, []float64{}
	for _, item := range list(t, body["resultados"]) {
		names = append(names, str(t, obj(t, obj(t, item)["persona"])["nombre_usuario"]))
		n, ok := obj(t, item)["amigos_en_comun"].(float64)
		require.True(t, ok)
		common = append(common, n)
	}
	degraded, ok := body["degradado"].(bool)
	require.True(t, ok)
	return names, common, degraded
}

func decodeOK(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	raw := readAll(t, resp)
	require.Equal(t, http.StatusOK, resp.StatusCode, raw)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &out))
	assertSnakeCase(t, out)
	return out
}

func TestSugerencias_OrdenPorAmigosEnComun(t *testing.T) {
	env, app := grafoEnv(t)
	ana := seedAmigo(t, env, "ana_sug")
	b, c, d := seedAmigo(t, env, "b_sug"), seedAmigo(t, env, "c_sug"), seedAmigo(t, env, "d_sug")
	x, y, z := seedAmigo(t, env, "x_sug"), seedAmigo(t, env, "y_sug"), seedAmigo(t, env, "z_sug")
	for _, f := range []amigo{b, c, d} {
		relacion(t, env, ana.user, f.user, amistades.StatusAccepted)
		relacion(t, env, f.user, x.user, amistades.StatusAccepted)
	}
	relacion(t, env, b.user, y.user, amistades.StatusAccepted)
	relacion(t, env, b.user, z.user, amistades.StatusAccepted)
	relacion(t, env, c.user, z.user, amistades.StatusAccepted)
	proyectar(t, env)

	names, common, degraded := sugeridos(t, app, ana)
	assert.False(t, degraded)
	assert.Equal(t, []string{"x_sug", "z_sug", "y_sug"}, names)
	assert.Equal(t, []float64{3, 2, 1}, common)
}

func TestSugerencias_ExcluyeAmigosPendientesBloqueadosYNoBuscables(t *testing.T) {
	env, app := grafoEnv(t)
	ana, b := seedAmigo(t, env, "ana_exc"), seedAmigo(t, env, "b_exc")
	amiga := seedAmigo(t, env, "amiga_exc")
	pendiente, bloqueo, bloqueada := seedAmigo(t, env, "pend_exc"), seedAmigo(t, env, "bloq_exc"), seedAmigo(t, env, "bloqd_exc")
	oculta := seedPersona(t, env, "oculta_exc@amigos.mx", "oculta_exc", personaOpts{hideName: true})
	inactiva := seedPersona(t, env, "inact_exc@amigos.mx", "inact_exc", personaOpts{status: usuarios.StatusPendingDeletion})
	visible := seedAmigo(t, env, "visible_exc")

	relacion(t, env, ana.user, b.user, amistades.StatusAccepted)
	relacion(t, env, ana.user, amiga.user, amistades.StatusAccepted)
	relacion(t, env, ana.user, pendiente.user, amistades.StatusPending)
	relacion(t, env, ana.user, bloqueada.user, amistades.StatusBlocked)
	relacion(t, env, bloqueo.user, ana.user, amistades.StatusBlocked)
	for _, u := range []usuarios.User{amiga.user, pendiente.user, bloqueo.user, bloqueada.user, oculta, inactiva, visible.user} {
		relacion(t, env, b.user, u, amistades.StatusAccepted)
	}
	proyectar(t, env)

	names, _, _ := sugeridos(t, app, ana)
	assert.Equal(t, []string{"visible_exc"}, names)
}

func TestSugerencias_TopeDeVeinte(t *testing.T) {
	env, app := grafoEnv(t)
	ana, b := seedAmigo(t, env, "ana_tope"), seedAmigo(t, env, "b_tope")
	relacion(t, env, ana.user, b.user, amistades.StatusAccepted)
	for i := range 22 {
		u := seedPersona(t, env, "tope"+string(rune('a'+i))+"@amigos.mx", "tope_"+string(rune('a'+i)), personaOpts{})
		relacion(t, env, b.user, u, amistades.StatusAccepted)
	}
	proyectar(t, env)

	names, _, _ := sugeridos(t, app, ana)
	assert.Len(t, names, 20)
}

func TestSugerencias_Neo4jCaidoEsDegradadoNoError(t *testing.T) {
	env, _ := amistadesEnv(t)
	dead, err := neo4j.New("neo4j://127.0.0.1:1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = dead.Close(context.Background()) })
	env.Sugerencias.WithGraph(dead)
	ana := seedAmigo(t, env, "ana_caido")

	names, _, degraded := sugeridos(t, env.App(), ana)
	assert.True(t, degraded)
	assert.Empty(t, names)
}

func TestSugerencias_SinSesion401(t *testing.T) {
	_, app := grafoEnv(t)
	resp := sendJSON(t, app, http.MethodGet, sugerenciasPath, nil, "")
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// amigosEn devuelve total y nombres de usuario de GET /lugares/{id}/amigos.
func amigosEn(t *testing.T, app *fiber.App, who amigo, place string) (float64, []string) {
	t.Helper()
	body := decodeOK(t, sendJSON(t, app, http.MethodGet, "/api/v1/lugares/"+place+"/amigos", nil, who.token))
	total, ok := body["total"].(float64)
	require.True(t, ok)
	names := []string{}
	for _, p := range list(t, body["amigos"]) {
		names = append(names, str(t, obj(t, p)["nombre_usuario"]))
	}
	return total, names
}

func lugarAprobado(t *testing.T, env *authtest.Env, slug string) negocios.Business {
	t.Helper()
	place := placeDoc(placeSpec{slug: slug, category: "cafeteria", windows: weekdays()})
	_, err := env.Mongo.DB().Collection(negocios.Collection).InsertOne(context.Background(), place)
	require.NoError(t, err)
	return place
}

func TestSenalSocial_AmigoConPlanGuardado(t *testing.T) {
	env, ana, beto, _ := amigosAceptados(t)
	env.Sugerencias.WithGraph(neo4jDriver)
	place := lugarAprobado(t, env, "senal-cafe")
	_, err := env.Planes.Create(context.Background(), beto.id(), planes.CreateRequest{
		Nombre: "Café", Fecha: lunesMediodia.Format(time.DateOnly), Personas: 2, Movilidad: planes.MobilityWalk,
		Paradas: []planes.StopRequest{{NegocioID: place.ID.Hex(), Llega: lunesMediodia.Format(time.RFC3339), Minutos: 60, Precio: 100}},
	})
	require.NoError(t, err)
	drain(t, env, grafoWorker(env, neo4jDriver))

	total, names := amigosEn(t, env.App(), ana, place.ID.Hex())
	assert.InDelta(t, 1, total, 0)
	assert.Equal(t, []string{"beto_grafo"}, names)
	total, _ = amigosEn(t, env.App(), beto, place.ID.Hex())
	assert.Zero(t, total, "el propio plan no cuenta")
}

func TestSenalSocial_MasDeTresYSoloAmigos(t *testing.T) {
	env, app := grafoEnv(t)
	ana, extrano := seedAmigo(t, env, "ana_senal"), seedAmigo(t, env, "extrano_senal")
	place := lugarAprobado(t, env, "senal-tacos")
	friends := []usuarios.User{}
	for i := range 5 {
		f := seedAmigo(t, env, "f"+string(rune('a'+i))+"_senal")
		relacion(t, env, ana.user, f.user, amistades.StatusAccepted)
		friends = append(friends, f.user)
	}
	relacion(t, env, extrano.user, ana.user, amistades.StatusPending)
	proyectar(t, env)
	for _, u := range append(friends, extrano.user) {
		fueA(t, u, place.ID.Hex())
	}

	total, names := amigosEn(t, app, ana, place.ID.Hex())
	assert.InDelta(t, 5, total, 0)
	assert.Len(t, names, 3)
	assert.NotContains(t, names, "extrano_senal")
}

func TestSenalSocial_NingunoY404(t *testing.T) {
	env, app := grafoEnv(t)
	ana, b := seedAmigo(t, env, "ana_ninguno"), seedAmigo(t, env, "b_ninguno")
	relacion(t, env, ana.user, b.user, amistades.StatusAccepted)
	proyectar(t, env)
	place := lugarAprobado(t, env, "senal-museo")

	total, names := amigosEn(t, app, ana, place.ID.Hex())
	assert.Zero(t, total)
	assert.Empty(t, names)

	resp := sendJSON(t, app, http.MethodGet, "/api/v1/lugares/"+bson.NewObjectID().Hex()+"/amigos", nil, ana.token)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	neo4jdrv "github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/grafo"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/neo4j"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
)

// grafoWorker aplica la cola del Env contra driver (Neo4j real o caído).
func grafoWorker(env *authtest.Env, driver *neo4j.Driver) *grafo.Worker {
	db := env.Mongo.DB()
	return grafo.NewWorker(env.Grafo, amistades.NewRepository(db), planes.NewRepository(db), driver, env.Log)
}

// emptyGraph deja Neo4j vacío y con el esquema.
func emptyGraph(t *testing.T) {
	t.Helper()
	cypher(t, "MATCH (n) DETACH DELETE n", nil)
	require.NoError(t, neo4jDriver.EnsureSchema(context.Background()))
}

func cypher(t *testing.T, query string, params map[string]any) []*neo4jdrv.Record {
	t.Helper()
	res, err := neo4jdrv.ExecuteQuery(context.Background(), neo4jDriver.DriverWithContext, query, params, neo4jdrv.EagerResultTransformer)
	require.NoError(t, err)
	return res.Records
}

// neighbors son los ids al otro lado de rel (AMIGO_DE sin dirección, FUE_A saliente).
func neighbors(t *testing.T, id, rel string) []string {
	t.Helper()
	pattern := "MATCH (:Usuario {id: $id})-[:AMIGO_DE]-(x) RETURN x.id AS id ORDER BY id"
	if rel == "FUE_A" {
		pattern = "MATCH (:Usuario {id: $id})-[:FUE_A]->(x) RETURN x.id AS id ORDER BY id"
	}
	out := []string{}
	for _, r := range cypher(t, pattern, map[string]any{"id": id}) {
		v, _ := r.Get("id")
		s, ok := v.(string)
		require.True(t, ok)
		out = append(out, s)
	}
	return out
}

func countGraph(t *testing.T, query string) int64 {
	t.Helper()
	v, _ := cypher(t, query, nil)[0].Get("total")
	n, ok := v.(int64)
	require.True(t, ok)
	return n
}

func drain(t *testing.T, env *authtest.Env, w *grafo.Worker) {
	t.Helper()
	_, err := w.Drain(context.Background())
	require.NoError(t, err)
	pending, err := env.Grafo.Pending(context.Background())
	require.NoError(t, err)
	require.Zero(t, pending, "la cola queda vacía")
}

// amigosAceptados deja a ana y beto como amigos por la API y devuelve el id de la amistad.
func amigosAceptados(t *testing.T) (*authtest.Env, amigo, amigo, string) {
	t.Helper()
	env, app := amistadesEnv(t)
	emptyGraph(t)
	ana, beto := seedAmigo(t, env, "ana_grafo"), seedAmigo(t, env, "beto_grafo")
	id := str(t, solicitar(t, app, ana, beto, http.StatusCreated)["id"])
	responder(t, app, beto, id, amistades.ActionAccept, http.StatusOK)
	return env, ana, beto, id
}

func TestProyeccion_AceptarProyectaAmigoDe(t *testing.T) {
	env, ana, beto, _ := amigosAceptados(t)

	drain(t, env, grafoWorker(env, neo4jDriver))

	assert.Equal(t, []string{beto.id()}, neighbors(t, ana.id(), "AMIGO_DE"))
	assert.Equal(t, []string{ana.id()}, neighbors(t, beto.id(), "AMIGO_DE"))
	assert.Equal(t, int64(1), countGraph(t, "MATCH ()-[r:AMIGO_DE]->() RETURN count(r) AS total"))
	// Sin datos personales: los nodos solo tienen id.
	for _, r := range cypher(t, "MATCH (u:Usuario) RETURN keys(u) AS llaves", nil) {
		keys, _ := r.Get("llaves")
		assert.Equal(t, []any{"id"}, keys)
	}
}

func TestProyeccion_EliminarQuitaRelacionYNodos(t *testing.T) {
	env, ana, _, id := amigosAceptados(t)
	w := grafoWorker(env, neo4jDriver)
	drain(t, env, w)

	amistadCall(t, env.App(), http.MethodDelete, amistadesPath+"/"+id, nil, ana.token, http.StatusNoContent)
	drain(t, env, w)

	assert.Empty(t, neighbors(t, ana.id(), "AMIGO_DE"))
	assert.Equal(t, int64(0), countGraph(t, "MATCH (n:Usuario) RETURN count(n) AS total"), "un nodo sin relaciones sale del grafo")
}

func TestProyeccion_BloquearQuitaAmigoDe(t *testing.T) {
	env, _, beto, id := amigosAceptados(t)
	w := grafoWorker(env, neo4jDriver)
	drain(t, env, w)

	amistadCall(t, env.App(), http.MethodPost, amistadesPath+"/"+id+"/bloquear", nil, beto.token, http.StatusNoContent)
	drain(t, env, w)

	assert.Equal(t, int64(0), countGraph(t, "MATCH ()-[r:AMIGO_DE]->() RETURN count(r) AS total"))
}

func TestProyeccion_RepetirNoDuplica(t *testing.T) {
	env, ana, beto, _ := amigosAceptados(t)
	w := grafoWorker(env, neo4jDriver)
	drain(t, env, w)

	for _, u := range []amigo{ana, beto, ana} {
		require.NoError(t, env.Grafo.Touch(context.Background(), u.user.ID))
		drain(t, env, w)
	}

	assert.Equal(t, int64(1), countGraph(t, "MATCH ()-[r:AMIGO_DE]->() RETURN count(r) AS total"))
	assert.Equal(t, int64(2), countGraph(t, "MATCH (n:Usuario) RETURN count(n) AS total"))
}

func TestProyeccion_Neo4jCaidoReintentaYLaAmistadSeGuarda(t *testing.T) {
	// La aceptación ya respondió 200 en amigosAceptados, con la cola pendiente.
	env, ana, beto, _ := amigosAceptados(t)
	dead, err := neo4j.New("neo4j://127.0.0.1:1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = dead.Close(context.Background()) })

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	applied, err := grafoWorker(env, dead).Drain(ctx)
	require.Error(t, err)
	assert.Zero(t, applied)

	var doc bson.M
	require.NoError(t, env.Mongo.DB().Collection(grafo.Collection).FindOne(context.Background(), bson.D{{Key: "_id", Value: beto.user.ID}}).Decode(&doc))
	assert.EqualValues(t, 1, doc["intentos"], "queda en la cola con un intento")
	f, ok, err := amistades.NewRepository(env.Mongo.DB()).FindByPair(context.Background(), ana.user.ID, beto.user.ID)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, amistades.StatusAccepted, f.Status, "Mongo manda: la amistad sigue aceptada")

	// Sin esperar la pausa del reintento: Neo4j vuelve y se aplica.
	_, err = env.Mongo.DB().Collection(grafo.Collection).UpdateMany(context.Background(), bson.D{},
		bson.D{{Key: "$set", Value: bson.D{{Key: "disponible_en", Value: time.Now().Add(-time.Minute)}}}})
	require.NoError(t, err)
	drain(t, env, grafoWorker(env, neo4jDriver))
	assert.Equal(t, []string{ana.id()}, neighbors(t, beto.id(), "AMIGO_DE"))
}

func TestProyeccion_PlanGuardadoProyectaFueA(t *testing.T) {
	env, ana, _, _ := amigosAceptados(t)
	place := placeDoc(placeSpec{slug: "grafo-cafe", category: "cafeteria", windows: weekdays()})
	_, err := env.Mongo.DB().Collection(negocios.Collection).InsertOne(context.Background(), place)
	require.NoError(t, err)
	w := grafoWorker(env, neo4jDriver)

	plan, err := env.Planes.Create(context.Background(), ana.id(), planes.CreateRequest{
		Nombre: "Domingo", Fecha: lunesMediodia.Format(time.DateOnly), Personas: 2, Movilidad: planes.MobilityWalk,
		Paradas: []planes.StopRequest{{NegocioID: place.ID.Hex(), Llega: lunesMediodia.Format(time.RFC3339), Minutos: 60, Precio: 100}},
	})
	require.NoError(t, err)
	drain(t, env, w)
	assert.Equal(t, []string{place.ID.Hex()}, neighbors(t, ana.id(), "FUE_A"))

	require.NoError(t, env.Planes.Delete(context.Background(), ana.id(), plan.ID.Hex(), plan.Version))
	drain(t, env, w)
	assert.Empty(t, neighbors(t, ana.id(), "FUE_A"))
	assert.Equal(t, int64(0), countGraph(t, "MATCH (n:Negocio) RETURN count(n) AS total"))
}

func TestReconstruir_CoincideConMongo(t *testing.T) {
	env, ana, beto, _ := amigosAceptados(t)
	// Basura que Mongo no respalda: una amistad inventada.
	cypher(t, "MERGE (a:Usuario {id: $a}) MERGE (b:Usuario {id: 'fantasma'}) MERGE (a)-[:AMIGO_DE]->(b)", map[string]any{"a": ana.id()})

	users, err := grafoWorker(env, neo4jDriver).Rebuild(context.Background(), env.Mongo.DB())
	require.NoError(t, err)

	assert.Equal(t, 2, users)
	assert.Equal(t, []string{beto.id()}, neighbors(t, ana.id(), "AMIGO_DE"))
	assert.Equal(t, int64(2), countGraph(t, "MATCH (n:Usuario) RETURN count(n) AS total"))
}

func TestReconstruir_IdempotenteYConservaLaDemostracion(t *testing.T) {
	env, _, _, _ := amigosAceptados(t)
	require.NoError(t, neo4jDriver.SeedDemo(context.Background()))
	w := grafoWorker(env, neo4jDriver)

	_, err := w.Rebuild(context.Background(), env.Mongo.DB())
	require.NoError(t, err)
	first, err := neo4jDriver.GraphSize(context.Background())
	require.NoError(t, err)
	_, err = w.Rebuild(context.Background(), env.Mongo.DB())
	require.NoError(t, err)
	second, err := neo4jDriver.GraphSize(context.Background())
	require.NoError(t, err)

	assert.Equal(t, first, second)
	assert.Equal(t, int64(3), countGraph(t, "MATCH (u:Usuario) WHERE u.id STARTS WITH 'demo-' RETURN count(u) AS total"))
}

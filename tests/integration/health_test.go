package integration

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/indexes"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
)

func TestHealth_LivenessWithStatusAndVersion(t *testing.T) {
	deps := realDeps()
	// /health no consulta bases: responde 200 aunque todas estén caídas.
	deps.Mongo, deps.Neo4j, deps.Redis = testutil.DownPinger{}, testutil.DownPinger{}, testutil.DeadRedis(t)

	resp := get(t, newApp("prod", deps), "/health")

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, map[string]any{"estado": "ok", "version": testVersion}, decode(t, resp))
}

func TestReady_AllThreeUp(t *testing.T) {
	resp := get(t, newApp("prod", realDeps()), "/ready")
	body := decode(t, resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "ok", body["estado"])
	assert.Equal(t, map[string]any{"mongodb": "ok", "redis": "ok", "neo4j": "ok"}, body["dependencias"])
}

func TestReady_MongoDownIs503(t *testing.T) {
	deadMongo, err := mongo.New("mongodb://" + testutil.DeadAddr)
	require.NoError(t, err)
	deps := realDeps()
	deps.Mongo = deadMongo

	resp := get(t, newApp("prod", deps), "/ready")
	raw := readAll(t, resp)

	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	assert.JSONEq(t, `{"estado":"error","dependencias":{"mongodb":"error","redis":"ok","neo4j":"ok"}}`, raw)
	assert.NotContains(t, raw, "127.0.0.1", "sin cadenas de conexión ni trazas en la respuesta")
}

func TestReady_RedisDownIsDegraded200(t *testing.T) {
	deps := realDeps()
	deps.Redis = testutil.DeadRedis(t)

	resp := get(t, newApp("prod", deps), "/ready")
	body := decode(t, resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "degraded", body["estado"])
	assert.Equal(t, map[string]any{"mongodb": "ok", "redis": "error", "neo4j": "ok"}, body["dependencias"])
}

func TestReady_Neo4jDownIs503(t *testing.T) {
	deps := realDeps()
	deps.Neo4j = testutil.DownPinger{}

	resp := get(t, newApp("prod", deps), "/ready")

	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	statuses, ok := decode(t, resp)["dependencias"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "error", statuses["neo4j"])
}

// La API no crea índices: si nadie corrió scripts/indexes, /ready da 503 hasta que existan.
func TestReady_MissingIndexesIs503UntilScriptRuns(t *testing.T) {
	ctx := context.Background()
	db := mongoClient.DB()
	require.NoError(t, db.Drop(ctx))
	deps := realDeps()
	deps.MongoIndexes = mongo.NewIndexVerifier(db, indexes.All()...)
	app := newApp("prod", deps)

	missing := get(t, app, "/ready")
	raw := readAll(t, missing)
	assert.Equal(t, http.StatusServiceUnavailable, missing.StatusCode)
	assert.JSONEq(t, `{"estado":"error","dependencias":{"mongodb":"ok","redis":"ok","neo4j":"ok","indices_mongodb":"error"}}`, raw)

	require.NoError(t, mongo.EnsureIndexes(ctx, db, indexes.All()...))
	ready := get(t, app, "/ready")
	assert.Equal(t, http.StatusOK, ready.StatusCode)
	assert.Equal(t, "ok", obj(t, decode(t, ready)["dependencias"])["indices_mongodb"])
}

// Un resultado correcto se recuerda unos minutos (los índices solo cambian con el script);
// Reset obliga a volver a revisar y un índice borrado vuelve a dar 503.
func TestReady_IndicesRecuerdaElResultadoCorrecto(t *testing.T) {
	ctx := context.Background()
	db := mongoClient.DB()
	require.NoError(t, db.Drop(ctx))
	require.NoError(t, mongo.EnsureIndexes(ctx, db, indexes.All()...))
	verifier := mongo.NewIndexVerifier(db, indexes.All()...)
	require.NoError(t, verifier.Ping(ctx))

	require.NoError(t, db.Collection("usuarios").Indexes().DropOne(ctx, "ux_correo"))
	assert.NoError(t, verifier.Ping(ctx), "dentro de la ventana no vuelve a listar")
	verifier.Reset()
	assert.ErrorContains(t, verifier.Ping(ctx), "usuarios.ux_correo")
	require.NoError(t, mongo.EnsureIndexes(ctx, db, indexes.All()...))
}

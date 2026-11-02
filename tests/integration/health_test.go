package integration

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

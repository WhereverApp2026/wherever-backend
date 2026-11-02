// Package integration prueba la app real en proceso contra MongoDB, Redis y Neo4j reales
// levantados con Testcontainers en puertos fijos. Correr con `go test -p 1`.
package integration

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	httpapi "github.com/WhereverApp2026/wherever-backend/internal/http"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/neo4j"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/redis"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
)

const testVersion = "v0.0.0-test"

var (
	stack       *testutil.Stack
	mongoClient *mongo.Client
	redisClient *goredis.Client
	neo4jDriver *neo4j.Driver
)

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	// Ryuk (el limpiador de Testcontainers) no arranca en Docker Desktop para Windows;
	// ahí se desactiva y los contenedores se apagan en el defer de abajo. En CI (Linux) sigue activo.
	if runtime.GOOS == "windows" && os.Getenv("TESTCONTAINERS_RYUK_DISABLED") == "" {
		_ = os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var err error
	if stack, err = testutil.StartStack(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "no se pudieron levantar las bases:", err)
		return 1
	}
	defer func() { _ = stack.Stop(context.Background()) }()

	if mongoClient, err = mongo.New(stack.MongoURI); err != nil {
		fmt.Fprintln(os.Stderr, "mongodb:", err)
		return 1
	}
	if redisClient, err = redis.New(stack.RedisAddr); err != nil {
		fmt.Fprintln(os.Stderr, "redis:", err)
		return 1
	}
	if neo4jDriver, err = neo4j.New(stack.Neo4jURI); err != nil {
		fmt.Fprintln(os.Stderr, "neo4j:", err)
		return 1
	}
	defer func() {
		_ = mongoClient.Close(context.Background())
		_ = redisClient.Close()
		_ = neo4jDriver.Close(context.Background())
	}()
	return m.Run()
}

// realDeps son las tres bases reales; cada prueba puede cambiar una por una caída.
func realDeps() httpapi.Deps {
	log, _ := testutil.Logger()
	return httpapi.Deps{Logger: log, Version: testVersion, Redis: redisClient, Mongo: mongoClient, Neo4j: neo4jDriver}
}

func newApp(env string, deps httpapi.Deps) *fiber.App {
	return httpapi.NewApp(testutil.Config(env), deps)
}

func newAppWith(cfg config.Config) *fiber.App {
	return httpapi.NewApp(cfg, realDeps())
}

func request(t *testing.T, app *fiber.App, method, path string, headers map[string]string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := app.Test(req, int((10 * time.Second).Milliseconds()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func get(t *testing.T, app *fiber.App, path string) *http.Response {
	return request(t, app, http.MethodGet, path, nil)
}

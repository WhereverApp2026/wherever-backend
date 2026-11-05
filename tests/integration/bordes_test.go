package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/neo4j"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/redis"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
)

const (
	// Cada dependencia tiene 1 s en /ready y 2 s de selección en Mongo; esto es el techo con holgura.
	readyLimit = 4 * time.Second
	routeLimit = 5 * time.Second
)

// hangingAddr acepta conexiones y nunca responde: una base lenta, no una caída.
func hangingAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			t.Cleanup(func() { _ = conn.Close() })
		}
	}()
	return ln.Addr().String()
}

func mongoAt(t *testing.T, addr string) *mongo.Client {
	t.Helper()
	client, err := mongo.New("mongodb://" + addr + "/wherever_test")
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close(context.Background()) })
	return client
}

func redisAt(t *testing.T, addr string) *goredis.Client {
	t.Helper()
	client, err := redis.New(addr)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	return client
}

// assertContract comprueba el formato de error y que no se filtren direcciones. Vale para prod:
// fuera de prod `detalle` trae el texto del driver a propósito (constitución §2).
func assertContract(t *testing.T, status int, raw string) {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &body), "error %d sin JSON: %q", status, raw)
	for _, key := range []string{"codigo", "mensaje", "campos", "traza_id"} {
		assert.Contains(t, body, key, "error %d: %q", status, raw)
	}
	assert.NotContains(t, raw, "127.0.0.1", "sin direcciones en la respuesta")
}

func TestBordes_ReadyConDependenciaLenta(t *testing.T) {
	t.Run("mongo", func(t *testing.T) {
		deps := realDeps()
		deps.Mongo = mongoAt(t, hangingAddr(t))
		assertReadyWithin(t, newApp("prod", deps), http.StatusServiceUnavailable, "mongodb")
	})
	t.Run("redis", func(t *testing.T) {
		deps := realDeps()
		deps.Redis = redisAt(t, hangingAddr(t))
		assertReadyWithin(t, newApp("prod", deps), http.StatusOK, "redis")
	})
	t.Run("neo4j", func(t *testing.T) {
		driver, err := neo4j.New("neo4j://" + hangingAddr(t))
		require.NoError(t, err)
		t.Cleanup(func() { _ = driver.Close(context.Background()) })
		deps := realDeps()
		deps.Neo4j = driver
		assertReadyWithin(t, newApp("prod", deps), http.StatusServiceUnavailable, "neo4j")
	})
}

func assertReadyWithin(t *testing.T, app *fiber.App, want int, failing string) {
	t.Helper()
	start := time.Now()
	resp := get(t, app, "/ready")
	assert.Less(t, time.Since(start), readyLimit, "una base lenta no cuelga /ready")
	assert.Equal(t, want, resp.StatusCode)
	raw := readAll(t, resp)
	assert.NotContains(t, raw, "127.0.0.1")
	var body struct {
		Dependencias map[string]string `json:"dependencias"`
	}
	require.NoError(t, json.Unmarshal([]byte(raw), &body))
	assert.Equal(t, "error", body.Dependencias[failing], raw)
}

// assertBounded manda la petición y exige una respuesta controlada dentro del límite.
func assertBounded(t *testing.T, app *fiber.App, method, path string, body any, token string) {
	t.Helper()
	start := time.Now()
	resp := sendJSON(t, app, method, path, body, token)
	assert.Less(t, time.Since(start), routeLimit, "%s %s no debe colgarse", method, path)
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode, path)
	assertContract(t, resp.StatusCode, readAll(t, resp))
}

// El auth con Mongo caído ya lo cubre TestAuth_MongoDownIs503; aquí, lo lento y el resto de módulos.
func TestBordes_MongoCaidoOLentoEnUsuariosYCiudades(t *testing.T) {
	for name, addr := range map[string]func(*testing.T) string{
		"caido": func(*testing.T) string { return testutil.DeadAddr },
		"lento": hangingAddr,
	} {
		t.Run(name, func(t *testing.T) {
			env := authEnvWith(t, testutil.Config(config.EnvProd))
			env.CreateUser(t, ana)
			access, _ := loginHTTP(t, env.App(), ana)
			app := authtest.Build(t, mongoAt(t, addr(t)), redisClient, env.Cfg).App()

			assertBounded(t, app, http.MethodPost, "/api/v1/auth/login", loginBody(ana, authtest.Password), "")
			assertBounded(t, app, http.MethodGet, "/api/v1/usuarios/yo", nil, access)
			assertBounded(t, app, http.MethodGet, "/api/v1/usuarios/correo-disponible?correo=a@b.co", nil, "")
			assertBounded(t, app, http.MethodGet, "/api/v1/ciudades/autocompletar?q=guada", nil, "")
		})
	}
}

// Redis lento se trata igual que caído: revocados y bloqueo son fail-closed (503) y lo público sigue.
func TestBordes_RedisLento(t *testing.T) {
	env := authEnvWith(t, testutil.Config(config.EnvProd))
	env.CreateUser(t, ana)
	access, _ := loginHTTP(t, env.App(), ana)
	app := authtest.Build(t, mongoClient, redisAt(t, hangingAddr(t)), env.Cfg).App()

	assertBounded(t, app, http.MethodGet, "/api/v1/usuarios/yo", nil, access)
	assertBounded(t, app, http.MethodPost, "/api/v1/auth/login", loginBody(ana, authtest.Password), "")

	start := time.Now()
	resp := sendJSON(t, app, http.MethodGet, "/api/v1/usuarios/correo-disponible?correo=a@b.co", nil, "")
	assert.Equal(t, http.StatusOK, resp.StatusCode, "el límite pasa a memoria y lo público sigue respondiendo")
	assert.Less(t, time.Since(start), routeLimit)
}

// Neo4j caído hace que /ready dé 503, pero ningún módulo actual lo usa: el negocio sigue.
func TestBordes_Neo4jCaidoNoFrenaLoDemas(t *testing.T) {
	env := authEnv(t)
	env.CreateUser(t, ana)
	app := env.App() // env.App monta Neo4j como testutil.DownPinger

	assert.Equal(t, http.StatusServiceUnavailable, get(t, app, "/ready").StatusCode)
	access, _ := loginHTTP(t, app, ana)
	assert.Equal(t, http.StatusOK, sendJSON(t, app, http.MethodGet, "/api/v1/usuarios/yo", nil, access).StatusCode)
}

// Con un tiempo máximo corto, una base lenta termina en un error de contrato, no en una espera.
func TestBordes_TiempoDeEspera(t *testing.T) {
	cfg := testutil.Config(config.EnvProd)
	cfg.RequestTimeout = 200 * time.Millisecond
	env := authtest.Build(t, mongoAt(t, hangingAddr(t)), redisClient, cfg)
	require.NoError(t, redisClient.FlushDB(context.Background()).Err())

	start := time.Now()
	resp := sendJSON(t, env.App(), http.MethodPost, "/api/v1/auth/login", loginBody(ana, authtest.Password), "")
	assert.Less(t, time.Since(start), 2*time.Second, "respeta el tiempo máximo")
	assert.Contains(t, []int{http.StatusRequestTimeout, http.StatusServiceUnavailable}, resp.StatusCode)
	assertContract(t, resp.StatusCode, readAll(t, resp))
}

var hostileBodies = map[string]string{
	"json roto":     `{`,
	"nulo":          `null`,
	"lista":         `[]`,
	"tipo erroneo":  `{"correo":123,"password":["a"],"dispositivo":"x","token":{},"nombre":true,"categorias":"x","modo":1}`,
	"operador":      `{"correo":{"$ne":null},"token":{"$gt":""},"nombre":{"$where":"sleep(1000)"}}`,
	"utf8 invalido": "{\"nombre\":\"\xff\xfe\"}",
	"byte nulo":     "{\"nombre\":\"a\\u0000b\",\"correo\":\"a\\u0000@b.co\"}",
	"numero enorme": `{"presupuesto_min":1e400,"presupuesto_max":-1e400}`,
	"anidado":       strings.Repeat(`{"a":`, 5000),
	"largo":         `{"nombre":"` + strings.Repeat("a", 200000) + `"}`,
}

var pathParam = regexp.MustCompile(`:[a-z_]+`)

func sendRaw(t *testing.T, app *fiber.App, method, path, body, token string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer "+token)
	resp, err := app.Test(req, int((20 * time.Second).Milliseconds()))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(raw)
}

// Las rutas con sesión no se alcanzan en FuzzHandlers (Redis caído): aquí sí, con bases reales.
func TestBordes_PayloadsHostilesEnTodasLasRutas(t *testing.T) {
	env := authEnvWith(t, testutil.Config(config.EnvProd))
	env.CreateUser(t, ana)
	app := env.App()
	checked := 0
	for _, route := range app.GetRoutes(true) {
		if route.Method == http.MethodHead || route.Path == "/ready" || route.Path == "/health" {
			continue
		}
		path := pathParam.ReplaceAllString(route.Path, "no-es-un-id")
		env.ClearRateLimits(t)
		access, _ := loginHTTP(t, app, ana) // cada ruta con sesión propia: logout cierra la anterior
		for name, body := range hostileBodies {
			env.ClearRateLimits(t)
			status, raw := sendRaw(t, app, route.Method, path, body, access)
			assert.Less(t, status, http.StatusInternalServerError, "%s %s con %s: %s", route.Method, path, name, raw)
			if status >= http.StatusBadRequest {
				assertContract(t, status, raw)
			}
			checked++
		}
	}
	assert.Greater(t, checked, 200, "debe recorrer las rutas reales")
}

// Un cuerpo sobre el límite se rechaza con el formato de error. Con servidor real: app.Test no
// reproduce este caso, porque el rechazo ocurre al leer la conexión.
func TestBordes_CuerpoSobreElLimite(t *testing.T) {
	cfg := testutil.Config(config.EnvProd)
	cfg.BodyLimit = 1024
	app := authEnvWith(t, cfg).App()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() { _ = app.Listener(ln) }()
	t.Cleanup(func() { _ = app.Shutdown() })

	url := "http://" + ln.Addr().String() + "/api/v1/auth/login"
	resp, err := http.Post(url, fiber.MIMEApplicationJSON, strings.NewReader(strings.Repeat("a", 2048))) //nolint:gosec // URL local de la prueba
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	assert.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode, string(raw))
	assertContract(t, resp.StatusCode, string(raw))
}

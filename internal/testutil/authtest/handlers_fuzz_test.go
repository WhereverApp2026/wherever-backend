package authtest_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	platformmongo "github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
)

var routeParam = regexp.MustCompile(`:[a-z_]+`)

// downApp arma la app real con sus dependencias caídas: sin red, sin Docker y sin mocks de
// negocio. La entrada inválida se rechaza antes de tocar una base, y la válida llega hasta el
// repositorio real y recibe 503. Las rutas con sesión no pasan de la revisión en Redis: esas
// las cubre TestBordes_PayloadsHostilesEnTodasLasRutas contra bases reales.
func downApp(tb testing.TB) *fiber.App {
	tb.Helper()
	client, err := platformmongo.New("mongodb://" + testutil.DeadAddr + "/fuzz")
	require.NoError(tb, err)
	cfg := testutil.Config(config.EnvProd)
	cfg.RequestTimeout = 50 * time.Millisecond
	// Cada ejecución usa otra IP; si no, el límite por IP respondería 429 a casi todo.
	cfg.TrustedProxies = []string{"0.0.0.0/0"}
	return authtest.Build(tb, client, testutil.DeadRedis(tb), cfg).App()
}

type fuzzRoute struct{ method, path string }

func appRoutes(app *fiber.App) []fuzzRoute {
	var out []fuzzRoute
	for _, r := range app.GetRoutes(true) {
		if r.Method != http.MethodHead && r.Method != fiber.MethodConnect && r.Method != http.MethodTrace {
			out = append(out, fuzzRoute{r.Method, r.Path})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].method+out[i].path < out[j].method+out[j].path })
	return out
}

func assertErrorContract(t *testing.T, status int, raw []byte) {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.Unmarshal(raw, &body), "error %d sin JSON: %q", status, raw)
	for _, key := range []string{"codigo", "mensaje", "campos", "traza_id"} {
		assert.Contains(t, body, key, "error %d sin %s: %q", status, key, raw)
	}
}

// FuzzHandlers: ninguna petición hostil provoca pánico ni 5xx, salvo el 503 de las bases caídas.
func FuzzHandlers(f *testing.F) {
	app := downApp(f)
	routes := appRoutes(app)
	require.NotEmpty(f, routes)
	bodies := []string{
		`{"correo":"a@b.co","password":"Wherever2026","dispositivo":{"modelo":"m","sistema":"s"}}`,
		`{`, `null`, `[]`, `"x"`, `1e999`, `{"correo":{"$ne":null}}`, `{"token":"` + strings.Repeat("a", 5000) + `"}`,
		strings.Repeat(`{"a":`, 2000), "\x00\xff", ``,
	}
	for i := range routes {
		for _, body := range bodies {
			f.Add(uint8(i), "6728f0c1a4b2c3d4e5f60718", "correo=a@b.co&q=gu", "application/json", "", []byte(body))
		}
	}
	f.Add(uint8(0), "../..", "%zz", "multipart/form-data; boundary=x", "Bearer x.y.z", []byte("--x"))
	f.Add(uint8(1), "%00", "q=\x00", "application/xml", "Basic abc", []byte("<a/>"))

	var seq int
	f.Fuzz(func(t *testing.T, which uint8, param, query, ctype, auth string, body []byte) {
		seq++
		route := routes[int(which)%len(routes)]
		target := routeParam.ReplaceAllString(route.path, url.PathEscape(param)) + "?" + query
		req, err := http.NewRequest(route.method, "http://localhost"+target, strings.NewReader(string(body)))
		if err != nil {
			return // no es una petición HTTP válida: no llegaría a la API
		}
		req.Header.Set("Content-Type", ctype)
		req.Header.Set("Authorization", auth)
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("10.%d.%d.%d", seq>>16&255, seq>>8&255, seq&255))

		resp, err := app.Test(req, -1)
		if err != nil {
			return // fasthttp rechazó la forma de la petición antes de la app
		}
		defer func() { _ = resp.Body.Close() }()
		raw, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		if resp.StatusCode >= 500 {
			assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode, "%s %s: %s", route.method, target, raw)
		}
		isReady := route.path == "/ready"
		if resp.StatusCode >= 400 && route.method != http.MethodOptions && !isReady {
			assertErrorContract(t, resp.StatusCode, raw)
		}
	})
}

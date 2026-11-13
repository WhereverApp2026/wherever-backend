package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/errores"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
)

const (
	erroresPath      = "/api/v1/errores"
	adminErroresPath = "/api/v1/admin/errores"
)

// erroresEnv siembra a un consumidor que reporta y a un administrador que consulta (nv-64).
func erroresEnv(t *testing.T) (*authtest.Env, *fiber.App, string, string) {
	t.Helper()
	env := authEnv(t)
	env.ClearRateLimits(t)
	user := env.CreateUser(t, "reporta@errores.mx")
	admin := env.CreateAdmin(t, "admin@errores.mx")
	return env, env.App(),
		testutil.ValidTokenFor(t, user.ID.Hex(), "jti-reporta", middleware.RoleConsumer),
		testutil.ValidTokenFor(t, admin.ID.Hex(), "jti-admin-errores", middleware.RoleAdmin)
}

func reportar(t *testing.T, app *fiber.App, token string, body any, want int) {
	t.Helper()
	resp := sendJSON(t, app, http.MethodPost, erroresPath, body, token)
	raw := readAll(t, resp)
	require.Equal(t, want, resp.StatusCode, raw)
}

func appError(module, message string) map[string]string {
	return map[string]string{"modulo": module, "mensaje": message, "version": "1.4.0"}
}

func storedErrors(t *testing.T, env *authtest.Env) []errores.Report {
	t.Helper()
	cur, err := env.Mongo.DB().Collection(errores.Collection).Find(context.Background(), bson.D{})
	require.NoError(t, err)
	out := []errores.Report{}
	require.NoError(t, cur.All(context.Background(), &out))
	return out
}

func adminErrores(t *testing.T, app *fiber.App, token string, q url.Values, want int) map[string]any {
	t.Helper()
	resp := sendJSON(t, app, http.MethodGet, adminErroresPath+"?"+q.Encode(), nil, token)
	raw := readAll(t, resp)
	require.Equal(t, want, resp.StatusCode, raw)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &out))
	assertSnakeCase(t, out)
	return out
}

func modulesOf(t *testing.T, page map[string]any) []string {
	t.Helper()
	out := []string{}
	for _, item := range list(t, page["items"]) {
		out = append(out, str(t, obj(t, item)["modulo"]))
	}
	return out
}

func TestErrores_MismoErrorSumaConteoSinDatosPersonales(t *testing.T) {
	env, app, token, _ := erroresEnv(t)
	msg := "pago rechazado para ana@correo.mx tel +52 55 1234 5678 token eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjMifQ.c2lnbmF0dXJhLWZhbHNh"
	reportar(t, app, token, appError("pagos", msg), http.StatusAccepted)
	other := appError("pagos", "pago rechazado para beto@otro.mx tel 5598765432 token eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiI0NTYifQ.b3RyYS1maXJtYS1mYWxzYQ")
	other["traza_id"] = "req-2"
	reportar(t, app, token, other, http.StatusAccepted)

	rows := storedErrors(t, env)
	require.Len(t, rows, 1, "los dos mensajes filtrados son iguales: un solo grupo")
	row := rows[0]
	assert.Equal(t, int64(2), row.Count)
	assert.Equal(t, errores.OriginApp, row.Origin)
	assert.Equal(t, "req-2", row.TraceID)
	for _, secret := range []string{"ana@correo.mx", "beto@otro.mx", "1234", "5598765432", "eyJ"} {
		assert.NotContains(t, row.Message, secret)
	}
	assert.False(t, row.LastAt.Before(row.CreatedAt))

	raw, err := env.Mongo.DB().Collection(errores.Collection).FindOne(context.Background(), bson.D{}).Raw()
	require.NoError(t, err)
	_, lookupErr := raw.LookupErr("usuario_id")
	assert.Error(t, lookupErr, "sin usuario_id: nada que purgar ni exportar")
}

func TestErrores_ValidacionYCuerpo(t *testing.T) {
	env, app, token, _ := erroresEnv(t)
	extra := appError("pagos", "falló")
	extra["usuario"] = "x"
	reportar(t, app, token, extra, http.StatusBadRequest)
	reportar(t, app, token, appError("Pagos!", "falló"), http.StatusUnprocessableEntity)
	assert.Empty(t, storedErrors(t, env))
}

func TestErrores_AdminFiltraPorModuloFechaYCursor(t *testing.T) {
	env, app, token, adminToken := erroresEnv(t)
	for i, m := range []string{"pagos", "sincronizacion", "pagos"} {
		reportar(t, app, token, appError(m, "error "+string(rune('a'+i))), http.StatusAccepted)
	}
	old := time.Now().UTC().Add(-10 * 24 * time.Hour)
	_, err := env.Mongo.DB().Collection(errores.Collection).UpdateOne(context.Background(),
		bson.D{{Key: "modulo", Value: "sincronizacion"}}, bson.D{{Key: "$set", Value: bson.D{{Key: "ultimo_en", Value: old}}}})
	require.NoError(t, err)

	all := adminErrores(t, app, adminToken, url.Values{}, http.StatusOK)
	assert.Equal(t, []string{"pagos", "pagos", "sincronizacion"}, modulesOf(t, all), "del más reciente al más viejo")
	assert.Equal(t, []string{"sincronizacion"}, modulesOf(t, adminErrores(t, app, adminToken, url.Values{"modulo": {"sincronizacion"}}, http.StatusOK)))
	since := url.Values{"desde": {time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)}}
	assert.Equal(t, []string{"pagos", "pagos"}, modulesOf(t, adminErrores(t, app, adminToken, since, http.StatusOK)))

	first := adminErrores(t, app, adminToken, url.Values{"limite": {"2"}}, http.StatusOK)
	require.Len(t, list(t, first["items"]), 2)
	next := str(t, first["siguiente_cursor"])
	require.NotEmpty(t, next)
	second := adminErrores(t, app, adminToken, url.Values{"limite": {"2"}, "cursor": {next}}, http.StatusOK)
	assert.Equal(t, []string{"sincronizacion"}, modulesOf(t, second))
	assert.Empty(t, second["siguiente_cursor"])

	adminErrores(t, app, adminToken, url.Values{"modulo": {"pagos"}, "cursor": {next}}, http.StatusUnprocessableEntity)
	adminErrores(t, app, token, url.Values{}, http.StatusForbidden)
}

func TestErrores_LimitePorHora(t *testing.T) {
	_, app, token, _ := erroresEnv(t)
	for i := 0; i < 30; i++ {
		reportar(t, app, token, appError("pagos", "falló"), http.StatusAccepted)
	}
	resp := sendJSON(t, app, http.MethodPost, erroresPath, appError("pagos", "falló"), token)
	require.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.NotEmpty(t, resp.Header.Get(fiber.HeaderRetryAfter))
	reportar(t, app, "", appError("pagos", "falló"), http.StatusUnauthorized)
}

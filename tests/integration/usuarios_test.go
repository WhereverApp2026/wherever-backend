package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/ciudades"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/mailer"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
)

// Pruebas HTTP de usuarios y ciudades (spec nv-05 "Integration/Usuarios" y "Ciudades") con la
// app real, MongoDB y Redis reales.

func registerBody(email string) map[string]any {
	return map[string]any{
		"nombre": "Lú", "fecha_nacimiento": "1998-04-12", "correo": email, "password": authtest.Password,
		"acepta_terminos": true, "version_terminos": "2026-10", "promociones_correo": false, "dispositivo": deviceJSON,
	}
}

func registerHTTP(t *testing.T, app *fiber.App, email string) string {
	t.Helper()
	resp := sendJSON(t, app, http.MethodPost, "/api/v1/usuarios", registerBody(email), "")
	raw := readAll(t, resp)
	require.Equal(t, http.StatusCreated, resp.StatusCode, raw)
	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &body))
	return str(t, body["access_token"])
}

func sendWithHeaders(t *testing.T, app *fiber.App, method, path string, body any, token string, headers map[string]string) *http.Response {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer "+token)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := app.Test(req, int((20 * time.Second).Milliseconds()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func TestUsuarios_RegisterThenProfile(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	access := registerHTTP(t, app, lu)

	resp := sendJSON(t, app, http.MethodGet, "/api/v1/usuarios/yo", nil, access)

	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, `"1"`, resp.Header.Get(fiber.HeaderETag))
	raw := readAll(t, resp)
	for _, secret := range []string{"password_hash", "telefono_hash", "google_id", "argon2id"} {
		assert.NotContains(t, raw, secret)
	}
	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &body))
	assertSnakeCase(t, body)
	assert.Equal(t, lu, body["correo"])
	assert.Equal(t, float64(50), body["puntos"])
	assert.Equal(t, false, body["correo_verificado"])
}

// El índice único decide la carrera: de cinco registros simultáneos con el mismo correo, uno gana.
func TestUsuarios_DuplicateEmailRaceHasOneWinner(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	statuses := make([]int, 5)
	var wg sync.WaitGroup
	for i := range statuses {
		wg.Add(1)
		go func() {
			defer wg.Done()
			statuses[i] = sendJSON(t, app, http.MethodPost, "/api/v1/usuarios", registerBody(lu), "").StatusCode
		}()
	}
	wg.Wait()

	created, conflicts := 0, 0
	for _, s := range statuses {
		switch s {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			conflicts++
		}
	}
	assert.Equal(t, 1, created, "%v", statuses)
	assert.Equal(t, 4, conflicts, "%v", statuses)
}

func TestUsuarios_PatchUsesVersionAndIfMatch(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	access := registerHTTP(t, app, lu)
	body := map[string]any{"nombre": "Lucía", "foto_url": "https://cdn.wherever.test/f/lu.jpg"}

	assert.Equal(t, http.StatusBadRequest, sendWithHeaders(t, app, http.MethodPatch, "/api/v1/usuarios/yo", body, access, nil).StatusCode)
	first := sendWithHeaders(t, app, http.MethodPatch, "/api/v1/usuarios/yo", body, access, map[string]string{fiber.HeaderIfMatch: `"1"`})
	require.Equal(t, http.StatusOK, first.StatusCode)
	assert.Equal(t, `"2"`, first.Header.Get(fiber.HeaderETag))
	stale := sendWithHeaders(t, app, http.MethodPatch, "/api/v1/usuarios/yo", map[string]any{"nombre": "Otra"}, access, map[string]string{fiber.HeaderIfMatch: `"1"`})
	assert.Equal(t, http.StatusConflict, stale.StatusCode, "otro dispositivo ya cambió el perfil")
	insecure := sendWithHeaders(t, app, http.MethodPatch, "/api/v1/usuarios/yo", map[string]any{"foto_url": "http://x.test/a.jpg"}, access, map[string]string{fiber.HeaderIfMatch: `"2"`})
	assert.Equal(t, http.StatusUnprocessableEntity, insecure.StatusCode)
}

func TestUsuarios_PatchCityMustExist(t *testing.T) {
	env := authEnv(t)
	importCities(t, env)
	app := env.App()
	access := registerHTTP(t, app, lu)
	var cities []map[string]any
	require.NoError(t, decodeInto(sendJSON(t, app, http.MethodGet, "/api/v1/ciudades/autocompletar?q=guada", nil, ""), &cities))
	require.Len(t, cities, 1)
	ifMatch := func(v string) map[string]string { return map[string]string{fiber.HeaderIfMatch: v} }

	missing := sendWithHeaders(t, app, http.MethodPatch, "/api/v1/usuarios/yo", map[string]string{"ciudad_id": "65a1f0c2e4b0a1b2c3d4e5f6"}, access, ifMatch(`"1"`))
	assert.Equal(t, http.StatusUnprocessableEntity, missing.StatusCode)
	empty := sendWithHeaders(t, app, http.MethodPatch, "/api/v1/usuarios/yo", map[string]string{}, access, ifMatch(`"1"`))
	assert.Equal(t, http.StatusUnprocessableEntity, empty.StatusCode, "sin campos no hay nada que cambiar")
	ok := sendWithHeaders(t, app, http.MethodPatch, "/api/v1/usuarios/yo", map[string]string{"ciudad_id": str(t, cities[0]["id"])}, access, ifMatch(`W/"1"`))
	require.Equal(t, http.StatusOK, ok.StatusCode)
	assert.Equal(t, "Guadalajara", obj(t, decode(t, ok)["ciudad"])["nombre"])
}

func TestUsuarios_Tastes(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	access := registerHTTP(t, app, lu)

	two := sendJSON(t, app, http.MethodPut, "/api/v1/usuarios/yo/gustos", map[string]any{"categorias": []string{"cafe", "bar"}}, access)
	assert.Equal(t, http.StatusUnprocessableEntity, two.StatusCode)
	resp := sendJSON(t, app, http.MethodPut, "/api/v1/usuarios/yo/gustos", map[string]any{
		"categorias": []string{"cafe", "tacos", "museos"}, "presupuesto_min": 100, "presupuesto_max": 300, "restricciones": []string{"vegetariano"},
	}, access)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, float64(300), decode(t, resp)["presupuesto_max"])
	backwards := sendJSON(t, app, http.MethodPut, "/api/v1/usuarios/yo/gustos", map[string]any{
		"categorias": []string{"a", "b", "c"}, "presupuesto_min": 500, "presupuesto_max": 100,
	}, access)
	assert.Equal(t, http.StatusUnprocessableEntity, backwards.StatusCode)
}

func TestUsuarios_PartialPreferencesAndPrivacy(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	access := registerHTTP(t, app, lu)

	prefs := sendJSON(t, app, http.MethodPatch, "/api/v1/usuarios/yo/preferencias", map[string]any{
		"tema": "oscuro", "horario_silencioso": map[string]string{"desde": "22:00", "hasta": "07:30"},
	}, access)
	require.Equal(t, http.StatusOK, prefs.StatusCode)
	body := decode(t, prefs)
	assert.Equal(t, "oscuro", body["tema"])
	assert.Equal(t, "km", body["unidades"], "lo que no llega no cambia")
	off := sendJSON(t, app, http.MethodPatch, "/api/v1/usuarios/yo/preferencias", map[string]any{"notificaciones": map[string]bool{"reservas": false}}, access)
	assert.Equal(t, http.StatusUnprocessableEntity, off.StatusCode)

	priv := sendJSON(t, app, http.MethodPatch, "/api/v1/usuarios/yo/privacidad", map[string]any{"estado_animo": true, "version_terminos": "2026-11"}, access)
	require.Equal(t, http.StatusOK, priv.StatusCode)
	pb := decode(t, priv)
	assert.Equal(t, true, pb["estado_animo"])
	assert.Equal(t, "2026-11", pb["version_terminos"])
}

func TestUsuarios_PhoneDuplicateIs409(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	ana := registerHTTP(t, app, lu)
	beto := registerHTTP(t, app, "beto@wherever.test")
	phone := map[string]string{"telefono": "+5215512345678"}

	assert.Equal(t, http.StatusUnprocessableEntity, sendJSON(t, app, http.MethodPut, "/api/v1/usuarios/yo/telefono", map[string]string{"telefono": "55 1234 5678"}, ana).StatusCode)
	assert.Equal(t, http.StatusOK, sendJSON(t, app, http.MethodPut, "/api/v1/usuarios/yo/telefono", phone, ana).StatusCode)
	assert.Equal(t, http.StatusConflict, sendJSON(t, app, http.MethodPut, "/api/v1/usuarios/yo/telefono", phone, beto).StatusCode)
}

func TestUsuarios_ModeChange(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	access := registerHTTP(t, app, lu)

	assert.Equal(t, http.StatusForbidden, sendJSON(t, app, http.MethodPost, "/api/v1/usuarios/yo/modo", map[string]string{"modo": "negocio"}, access).StatusCode)
	assert.Equal(t, http.StatusUnprocessableEntity, sendJSON(t, app, http.MethodPost, "/api/v1/usuarios/yo/modo", map[string]string{"modo": "admin"}, access).StatusCode)
	resp := sendJSON(t, app, http.MethodPost, "/api/v1/usuarios/yo/modo", map[string]string{"modo": "explorar"}, access)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "explorar", decode(t, resp)["modo_actual"])
}

// Eliminar cierra todas las sesiones; para cancelar hay que volver a entrar, y eso sí se permite.
func TestUsuarios_DeleteClosesSessionsThenCancel(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	access := registerHTTP(t, app, lu)

	assert.Equal(t, http.StatusUnprocessableEntity, sendJSON(t, app, http.MethodDelete, "/api/v1/usuarios/yo", map[string]string{"confirmacion": "si"}, access).StatusCode)
	resp := sendJSON(t, app, http.MethodDelete, "/api/v1/usuarios/yo", map[string]string{"confirmacion": "ELIMINAR"}, access)
	require.Equal(t, http.StatusAccepted, resp.StatusCode)
	assert.NotEmpty(t, decode(t, resp)["eliminar_en"])
	assert.Equal(t, http.StatusUnauthorized, sendJSON(t, app, http.MethodGet, "/api/v1/usuarios/yo", nil, access).StatusCode)

	again, _ := loginHTTP(t, app, lu)
	cancel := sendJSON(t, app, http.MethodPost, "/api/v1/usuarios/yo/cancelar-eliminacion", nil, again)
	require.Equal(t, http.StatusOK, cancel.StatusCode)
	assert.Equal(t, "activo", decode(t, cancel)["estado"])
	assert.Equal(t, http.StatusConflict, sendJSON(t, app, http.MethodPost, "/api/v1/usuarios/yo/cancelar-eliminacion", nil, again).StatusCode)
}

func TestUsuarios_EmailAvailability(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	registerHTTP(t, app, lu)

	taken := sendJSON(t, app, http.MethodGet, "/api/v1/usuarios/correo-disponible?correo=LU%40wherever.test", nil, "")
	require.Equal(t, http.StatusOK, taken.StatusCode)
	assert.Equal(t, false, decode(t, taken)["disponible"])
	free := sendJSON(t, app, http.MethodGet, "/api/v1/usuarios/correo-disponible?correo=libre%40wherever.test", nil, "")
	assert.Equal(t, true, decode(t, free)["disponible"])
	assert.Equal(t, http.StatusUnprocessableEntity, sendJSON(t, app, http.MethodGet, "/api/v1/usuarios/correo-disponible?correo=x", nil, "").StatusCode)
}

func TestUsuarios_ExportAndHistory(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	access := registerHTTP(t, app, lu)

	assert.Equal(t, http.StatusNoContent, sendJSON(t, app, http.MethodDelete, "/api/v1/usuarios/yo/historial", nil, access).StatusCode)
	assert.Equal(t, http.StatusAccepted, sendJSON(t, app, http.MethodPost, "/api/v1/usuarios/yo/exportar", nil, access).StatusCode)
	again := sendJSON(t, app, http.MethodPost, "/api/v1/usuarios/yo/exportar", nil, access)
	assert.Equal(t, http.StatusTooManyRequests, again.StatusCode)
	assert.NotEmpty(t, again.Header.Get("Retry-After"))

	_, err := env.Job.RunOnce(context.Background())
	require.NoError(t, err)
	link, ok := env.Mail.Last(lu, mailer.KindExport)
	require.True(t, ok)
	download := sendJSON(t, app, http.MethodPost, "/api/v1/usuarios/exportaciones/descargar", map[string]string{"token": link.Token}, "")
	require.Equal(t, http.StatusOK, download.StatusCode)
	assert.Contains(t, download.Header.Get(fiber.HeaderContentDisposition), "attachment")
	var data map[string]any
	require.NoError(t, json.NewDecoder(download.Body).Decode(&data))
	assert.Contains(t, data, "usuarios")
	assert.Contains(t, data, "sesiones")
}

func importCities(t *testing.T, env *authtest.Env) {
	t.Helper()
	csv := "nombre,estado,latitud,longitud,con_cobertura\n" +
		"Tula de Allende,Hidalgo,20.0536,-99.3414,true\n" +
		"Tulancingo,Hidalgo,20.0833,-98.3667,false\n" +
		"Tultitlán,Estado de México,19.6450,-99.1694,true\n" +
		"Guadalajara,Jalisco,20.6767,-103.3475,true\n"
	n, err := env.Cities.Import(context.Background(), strings.NewReader(csv), "csv")
	require.NoError(t, err)
	require.Equal(t, 4, n)
	n, err = env.Cities.Import(context.Background(), strings.NewReader(csv), "csv")
	require.NoError(t, err)
	require.Equal(t, 4, n, "reimportar actualiza, no duplica")
}

func TestCiudades_AutocompleteByPrefixCoveredFirst(t *testing.T) {
	env := authEnv(t)
	importCities(t, env)
	app := env.App()

	resp := sendJSON(t, app, http.MethodGet, "/api/v1/ciudades/autocompletar?q=TUL", nil, "")

	require.Equal(t, http.StatusOK, resp.StatusCode)
	var out []map[string]any
	require.NoError(t, decodeInto(resp, &out))
	require.Len(t, out, 3)
	names := []any{out[0]["nombre"], out[1]["nombre"], out[2]["nombre"]}
	assert.Equal(t, []any{"Tula de Allende", "Tultitlán", "Tulancingo"}, names, "primero las de cobertura")
	limited := sendJSON(t, app, http.MethodGet, "/api/v1/ciudades/autocompletar?q=tul&limite=1", nil, "")
	require.NoError(t, decodeInto(limited, &out))
	assert.Len(t, out, 1)
	assert.Equal(t, http.StatusUnprocessableEntity, sendJSON(t, app, http.MethodGet, "/api/v1/ciudades/autocompletar?q=t", nil, "").StatusCode)
	assert.Equal(t, int64(4), mustCount(t, env))
}

func TestCiudades_PrefixQueryUsesUxCiudad(t *testing.T) {
	env := authEnv(t)
	importCities(t, env)

	var plan bson.M
	err := env.Mongo.DB().RunCommand(context.Background(), bson.D{
		{Key: "explain", Value: bson.D{
			{Key: "find", Value: ciudades.Collection},
			{Key: "filter", Value: bson.D{{Key: "nombre_normalizado", Value: bson.D{{Key: "$regex", Value: "^tul"}}}}},
		}},
		{Key: "verbosity", Value: "queryPlanner"},
	}).Decode(&plan)
	require.NoError(t, err)

	raw, err := json.Marshal(plan)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"IXSCAN"`)
	assert.Contains(t, string(raw), `"ux_ciudad"`)
}

func TestCiudades_UnknownIsEmptyListAndRegexIsEscaped(t *testing.T) {
	env := authEnv(t)
	importCities(t, env)
	app := env.App()

	for _, q := range []string{"zzz", ".%2A", "%5Bt", "tu(l"} {
		resp := sendJSON(t, app, http.MethodGet, "/api/v1/ciudades/autocompletar?q="+q, nil, "")
		require.Equal(t, http.StatusOK, resp.StatusCode, q)
		assert.Equal(t, "[]", strings.TrimSpace(readAll(t, resp)), q)
	}
}

func mustCount(t *testing.T, env *authtest.Env) int64 {
	t.Helper()
	n, err := env.Mongo.DB().Collection(ciudades.Collection).CountDocuments(context.Background(), bson.D{})
	require.NoError(t, err)
	return n
}

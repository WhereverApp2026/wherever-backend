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

	"github.com/WhereverApp2026/wherever-backend/internal/ciudades"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/climatest"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

const (
	inicioPath = "/api/v1/inicio?lat=19.4326&lng=-99.1332"
	climaPath  = "/api/v1/contexto/clima?lat=19.4326&lng=-99.1332"
)

// inicioEnv carga los 10 negocios de nv-23 con el reloj del lunes a mediodía.
func inicioEnv(t *testing.T) *authtest.Env {
	t.Helper()
	env := insertRecoEnv(t)
	env.Recomendaciones.WithClock(func() time.Time { return lunesMediodia })
	return env
}

func getBody(t *testing.T, app *fiber.App, path, token string) map[string]any {
	t.Helper()
	resp := sendJSON(t, app, http.MethodGet, path, nil, token)
	raw := readAll(t, resp)
	require.Equal(t, http.StatusOK, resp.StatusCode, raw)
	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &body))
	assertSnakeCase(t, body)
	return body
}

func inicioSlugs(t *testing.T, body map[string]any) []string {
	t.Helper()
	cards := []map[string]any{}
	for _, item := range list(t, body["recomendaciones"]) {
		cards = append(cards, obj(t, item))
	}
	return cardSlugs(t, cards)
}

// Integration/Inicio: saludo y chips por franja con hora fija.
func TestInicio_SaludoYChipsPorFranja(t *testing.T) {
	app := inicioEnv(t).App()
	cases := map[string][2]string{
		"08:00": {"manana", "Buenos días"},
		"15:00": {"tarde", "Buenas tardes"},
		"21:00": {"noche", "Buenas noches"},
	}
	for hora, want := range cases {
		body := getBody(t, app, inicioPath+"&hora="+hora, "")
		assert.Equal(t, want[0], body["franja"], hora)
		assert.Equal(t, want[1], body["saludo"], hora)
		assert.Len(t, list(t, body["chips"]), 3, hora)
		assert.LessOrEqual(t, len(list(t, body["recomendaciones"])), 5, hora)
	}
	body := getBody(t, app, inicioPath+"&hora=21:00", "")
	assert.Equal(t, "bares", obj(t, list(t, body["chips"])[1])["id"])
}

// Integration/Inicio: sin sesión, lo general (sin el filtro de alergias de nadie).
func TestInicio_SinSesionRecibeLoGeneral(t *testing.T) {
	env := inicioEnv(t)
	app := env.App()
	anon := inicioSlugs(t, getBody(t, app, inicioPath+"&hora=12:00", ""))
	require.Len(t, anon, 5)
	assert.Contains(t, anon, "cafe-cacahuate")

	u := env.CreateUser(t, "inicio-alergia@wherever.test")
	_, err := env.Users.Update(context.Background(), u.ID, bson.D{
		{Key: "gustos", Value: fullTastes()}, {Key: "privacidad", Value: usuarios.Privacy{Learning: true, SensitiveData: true}},
	})
	require.NoError(t, err)
	token := testutil.ValidTokenFor(t, u.ID.Hex(), "jti-inicio-alergia", middleware.RoleConsumer)
	mine := inicioSlugs(t, getBody(t, app, inicioPath+"&hora=12:00", token))
	assert.NotContains(t, mine, "cafe-cacahuate", "con sesión aplica su alergia")
	assert.Equal(t, "cafe-ideal", mine[0])
}

// Gate «Clima caído»: Inicio responde sin clima.
func TestInicio_ClimaCaidoRespondeSinClima(t *testing.T) {
	env := inicioEnv(t)
	env.OpenMeteo.Fallar(http.StatusServiceUnavailable)
	body := getBody(t, env.App(), inicioPath+"&hora=12:00", "")
	assert.Equal(t, map[string]any{"disponible": false}, obj(t, body["clima"]))
	assert.Len(t, list(t, body["recomendaciones"]), 5)
}

// Integration/Inicio: validación y token inválido.
func TestInicio_ValidacionYTokenInvalido(t *testing.T) {
	app := inicioEnv(t).App()
	assert.Equal(t, http.StatusUnprocessableEntity, sendJSON(t, app, http.MethodGet, "/api/v1/inicio?lat=19.4", nil, "").StatusCode)
	assert.Equal(t, http.StatusUnauthorized, sendJSON(t, app, http.MethodGet, inicioPath, nil, "no-es-un-jwt").StatusCode)
}

// Integration/Clima: respuesta normal, por ciudad y caché de 30 min.
func TestClima_RespuestaNormalYCache(t *testing.T) {
	env := negociosEnv(t)
	env.OpenMeteo.Responder(climatest.Actual{TemperaturaC: 18, Codigo: 63, PrecipitacionMM: 2})
	app := env.App()
	body := getBody(t, app, climaPath, "")
	assert.Equal(t, true, body["disponible"])
	assert.Equal(t, "lluvia", body["estado"])
	assert.Equal(t, true, body["adverso"])

	city := bson.NewObjectID()
	_, err := env.Mongo.DB().Collection(ciudades.Collection).InsertOne(context.Background(), ciudades.City{
		ID: city, Name: "Ciudad de México", State: "CDMX", NormalizedName: "ciudad de mexico",
		Center: ciudades.Point{Type: "Point", Coordinates: []float64{-99.1332, 19.4326}}, Covered: true,
	})
	require.NoError(t, err)
	assert.Equal(t, "lluvia", getBody(t, app, "/api/v1/contexto/clima?ciudad_id="+city.Hex(), "")["estado"])
	assert.Equal(t, 1, env.OpenMeteo.Llamadas(), "misma celda: de la caché")
}

// Integration/Clima: servicio lento (Inicio omite el factor) y caído.
func TestClima_LentoYCaido(t *testing.T) {
	env := inicioEnv(t)
	env.Clima.WithPlazo(100 * time.Millisecond)
	env.OpenMeteo.Tardar(2 * time.Second)
	body := getBody(t, env.App(), inicioPath+"&hora=12:00", "")
	assert.Equal(t, false, obj(t, body["clima"])["disponible"])
	assert.Len(t, list(t, body["recomendaciones"]), 5)

	env.Clima.Reset()
	env.OpenMeteo.Fallar(http.StatusInternalServerError)
	assert.Equal(t, map[string]any{"disponible": false}, getBody(t, env.App(), climaPath, ""))
}

func TestClima_Validacion(t *testing.T) {
	app := negociosEnv(t).App()
	for _, path := range []string{"/api/v1/contexto/clima", "/api/v1/contexto/clima?lat=91&lng=0", "/api/v1/contexto/clima?ciudad_id=nope", "/api/v1/contexto/clima?ciudad_id=" + bson.NewObjectID().Hex()} {
		assert.Equal(t, http.StatusUnprocessableEntity, sendJSON(t, app, http.MethodGet, path, nil, "").StatusCode, path)
	}
}

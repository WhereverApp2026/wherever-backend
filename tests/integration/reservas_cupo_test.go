package integration

import (
	"context"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/reservastest"
)

func diaDe(hora time.Time) int { return int(hora.In(negocios.MexicoCity()).Weekday()) }

// disponibilidadDe trae las filas de la disponibilidad de `fecha` por hora local ("15:04").
func disponibilidadDe(t *testing.T, app *fiber.App, b negocios.Business, fecha, extra string) map[string]map[string]any {
	t.Helper()
	resp := get(t, app, "/api/v1/lugares/"+b.ID.Hex()+"/disponibilidad?fecha="+fecha+"&personas=1"+extra)
	requireStatus(t, resp, http.StatusOK)
	horarios, ok := decode(t, resp)["horarios"].([]any)
	require.True(t, ok)
	out := map[string]map[string]any{}
	for _, h := range horarios {
		row := obj(t, h)
		hora, err := time.Parse(time.RFC3339, str(t, row["hora"]))
		require.NoError(t, err)
		out[hora.In(negocios.MexicoCity()).Format("15:04")] = row
	}
	return out
}

func putCupo(t *testing.T, app *fiber.App, b negocios.Business, token string, version int, cupos []map[string]any) *http.Response {
	t.Helper()
	return sendWithHeaders(t, app, http.MethodPut, "/api/v1/negocios/"+b.ID.Hex()+"/cupo", map[string]any{"cupo_por_hora": cupos}, token,
		map[string]string{"If-Match": strconv.Itoa(version)})
}

func cupo(dia int, hora string, lugares int) map[string]any {
	return map[string]any{"dia": dia, "hora": hora, "lugares": lugares}
}

func horasDe(t *testing.T, body map[string]any, dia int) []any {
	t.Helper()
	dias, ok := body["dias"].([]any)
	require.True(t, ok)
	require.Len(t, dias, 7)
	horas, ok := obj(t, dias[dia])["horas"].([]any)
	require.True(t, ok)
	return horas
}

func TestCupo_PorHoraYDiaConIfMatchYDisponibilidad(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	b, tokenDueno := dueno(t, env, "duena-cupo@wherever.test", reservastest.Config{Capacidad: 10})
	horarios := make([]negocios.Horario, 0, 7)
	for d := range 7 {
		horarios = append(horarios, negocios.Horario{Dia: d, Abre: "12:00", Cierra: "20:00"})
	}
	_, err := env.Mongo.DB().Collection(negocios.Collection).UpdateByID(context.Background(), b.ID, bson.D{{Key: "$set", Value: bson.D{{Key: "horarios", Value: horarios}}}})
	require.NoError(t, err)
	_, tokenCliente := cliente(t, env, "cliente-cupo@wherever.test")
	manana := reservastest.Manana(14)
	pasado := manana.AddDate(0, 0, 1)
	hoyDia, otroDia := diaDe(manana), diaDe(pasado)

	// Sin entradas: las horas abiertas con la capacidad general.
	resp := sendJSON(t, app, http.MethodGet, "/api/v1/negocios/"+b.ID.Hex()+"/cupo", nil, tokenDueno)
	requireStatus(t, resp, http.StatusOK)
	body := decode(t, resp)
	assertSnakeCase(t, body)
	assert.EqualValues(t, 10, body["capacidad_por_horario"])
	horas := horasDe(t, body, hoyDia)
	require.Len(t, horas, 8, "12:00 a 19:00")
	assert.Equal(t, "12:00", obj(t, horas[0])["hora"])
	assert.EqualValues(t, 10, obj(t, horas[0])["lugares"])
	version := versionDe(t, body)

	// Fuera de horario, repetida o sin If-Match: no cambia nada.
	requireStatus(t, putCupo(t, app, b, tokenDueno, version, []map[string]any{cupo(hoyDia, "21:00", 3)}), http.StatusUnprocessableEntity)
	requireStatus(t, putCupo(t, app, b, tokenDueno, version, []map[string]any{cupo(hoyDia, "14:00", 3), cupo(hoyDia, "14:00", 4)}), http.StatusUnprocessableEntity)
	resp = sendJSON(t, app, http.MethodPut, "/api/v1/negocios/"+b.ID.Hex()+"/cupo", map[string]any{"cupo_por_hora": []any{}}, tokenDueno)
	requireStatus(t, resp, http.StatusBadRequest)

	resp = putCupo(t, app, b, tokenDueno, version, []map[string]any{cupo(hoyDia, "14:00", 3), cupo(hoyDia, "15:00", 0), cupo(otroDia, "14:00", 8)})
	requireStatus(t, resp, http.StatusOK)
	body = decode(t, resp)
	assert.EqualValues(t, version+1, body["version"])
	horas = horasDe(t, body, hoyDia)
	assert.EqualValues(t, 3, obj(t, horas[2])["lugares"], "14:00")
	assert.EqualValues(t, 0, obj(t, horas[3])["lugares"], "15:00 cerrada")

	// La versión vieja ya no sirve.
	requireStatus(t, putCupo(t, app, b, tokenDueno, version, []map[string]any{}), http.StatusConflict)

	// La disponibilidad usa el cupo de cada hora y día.
	filas := disponibilidadDe(t, app, b, reservastest.FechaManana(), "")
	assert.EqualValues(t, 10, filas["13:00"]["lugares_libres"])
	assert.EqualValues(t, 3, filas["14:00"]["lugares_libres"])
	assert.EqualValues(t, 0, filas["15:00"]["lugares_libres"])
	assert.Equal(t, false, filas["15:00"]["disponible"])
	filas = disponibilidadDe(t, app, b, pasado.In(negocios.MexicoCity()).Format("2006-01-02"), "")
	assert.EqualValues(t, 8, filas["14:00"]["lugares_libres"], "otro día, otro cupo")

	// Ninguna reserva supera el cupo de su hora.
	requireStatus(t, reservar(t, app, tokenCliente, llave(8801), cuerpoReserva(b, manana, 4)), http.StatusConflict)
	requireStatus(t, reservar(t, app, tokenCliente, llave(8802), cuerpoReserva(b, manana, 3)), http.StatusCreated)
	requireStatus(t, reservar(t, app, tokenCliente, llave(8803), cuerpoReserva(b, manana.Add(time.Hour), 1)), http.StatusConflict)
	assert.EqualValues(t, 0, disponibilidadDe(t, app, b, reservastest.FechaManana(), "")["14:00"]["lugares_libres"])
}

func TestCupo_AfluenciaYSugerencia(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	b, _ := dueno(t, env, "duena-cupo-sug@wherever.test", reservastest.Config{Capacidad: 10})
	dia := diaDe(reservastest.Manana(14))
	curva := make([][]int, 7)
	for d := range curva {
		curva[d] = make([]int, 24)
		for h := range curva[d] {
			curva[d][h] = 90
		}
	}
	curva[dia][14] = 20
	curva[dia][22] = 5
	_, err := env.Mongo.DB().Collection(negocios.Collection).UpdateByID(context.Background(), b.ID, bson.D{{Key: "$set", Value: bson.D{{Key: "afluencia", Value: curva}}}})
	require.NoError(t, err)

	filas := disponibilidadDe(t, app, b, reservastest.FechaManana(), "&hora=15:00")
	assert.Equal(t, negocios.CrowdPeak, filas["13:00"]["afluencia"])
	assert.Equal(t, negocios.CrowdLow, filas["14:00"]["afluencia"])
	assert.Equal(t, true, filas["14:00"]["sugerida"], "la más tranquila cerca de las 15:00")
	assert.Equal(t, false, filas["22:00"]["sugerida"], "más tranquila, pero lejos de la pedida")
	sugeridas := 0
	for _, f := range filas {
		if f["sugerida"] == true {
			sugeridas++
		}
	}
	assert.Equal(t, 1, sugeridas)

	todo := disponibilidadDe(t, app, b, reservastest.FechaManana(), "")
	assert.Equal(t, true, todo["22:00"]["sugerida"], "sin hora pedida mira todo el día")

	resp := get(t, app, "/api/v1/lugares/"+b.ID.Hex()+"/disponibilidad?fecha="+reservastest.FechaManana()+"&personas=1&hora=25:00")
	requireStatus(t, resp, http.StatusUnprocessableEntity)
}

func TestCupo_MesasYMaxPersonas(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	b, tokenDueno := dueno(t, env, "duena-cupo-max@wherever.test", reservastest.Config{Capacidad: 10})
	dia := diaDe(reservastest.Manana(14))
	resp := putCupo(t, app, b, tokenDueno, b.Version, []map[string]any{cupo(dia, "14:00", 30)})
	requireStatus(t, resp, http.StatusOK)
	version := versionDe(t, decode(t, resp))

	cfg := map[string]any{"acepta_reservas": true, "capacidad_por_horario": 10, "duracion_turno_min": 60,
		"anticipo": map[string]any{"tipo": "fijo", "valor": 0}, "politica_cancelacion": "flexible", "confirmacion": "automatica", "permite_cambios": false,
		"mesas": 6, "max_personas": 20}
	ruta := "/api/v1/negocios/" + b.ID.Hex() + "/reservas-config"
	resp = sendWithHeaders(t, app, http.MethodPut, ruta, cfg, tokenDueno, map[string]string{"If-Match": strconv.Itoa(version)})
	requireStatus(t, resp, http.StatusUnprocessableEntity) // la hora de 30 supera el máximo

	cfg["max_personas"] = 40
	resp = sendWithHeaders(t, app, http.MethodPut, ruta, cfg, tokenDueno, map[string]string{"If-Match": strconv.Itoa(version)})
	requireStatus(t, resp, http.StatusOK)
	body := decode(t, resp)
	rc := obj(t, body["reservas_config"])
	assert.EqualValues(t, 6, rc["mesas"])
	assert.EqualValues(t, 40, rc["max_personas"])
	version = versionDe(t, body)

	resp = sendJSON(t, app, http.MethodGet, "/api/v1/negocios/"+b.ID.Hex()+"/cupo", nil, tokenDueno)
	requireStatus(t, resp, http.StatusOK)
	horas := horasDe(t, decode(t, resp), dia)
	assert.EqualValues(t, 30, obj(t, horas[14])["lugares"], "guardar «Horas y cupo» no borra el cupo por hora")

	requireStatus(t, putCupo(t, app, b, tokenDueno, version, []map[string]any{cupo(dia, "14:00", 41)}), http.StatusUnprocessableEntity)
}

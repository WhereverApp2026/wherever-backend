package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/vistas"
)

func conteo(t *testing.T, app *fiber.App, query string) (int, negocios.CountResponse) {
	t.Helper()
	resp := sendJSON(t, app, http.MethodGet, "/api/v1/lugares/conteo?"+query, nil, "")
	raw := readAll(t, resp)
	var out negocios.CountResponse
	if resp.StatusCode == http.StatusOK {
		require.NoError(t, json.Unmarshal([]byte(raw), &out), raw)
	}
	return resp.StatusCode, out
}

func listaTotal(t *testing.T, app *fiber.App, query string) int {
	t.Helper()
	resp := sendJSON(t, app, http.MethodGet, "/api/v1/lugares?limite=50&"+query, nil, "")
	raw := readAll(t, resp)
	require.Equal(t, http.StatusOK, resp.StatusCode, raw)
	var out struct {
		Items []json.RawMessage `json:"items"`
	}
	require.NoError(t, json.Unmarshal([]byte(raw), &out))
	return len(out.Items)
}

func TestLugaresConteo_IgualQueLaLista(t *testing.T) {
	env := negociosEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	insertPlaces(t, env, []placeSpec{
		{slug: "c1"}, {slug: "c2", price: 4}, {slug: "c3", category: "cafeteria"},
		{slug: "c4", dLat: 0.05}, {slug: "c5", state: negocios.StatusReview}, {slug: "c6", deleted: true},
		{slug: "c7", windows: []negocios.VentanaMin{}},
	})
	at := fmt.Sprintf("lat=%v&lng=%v", centerLat, centerLng)
	for _, q := range []string{"", "categoria=cafeteria", "precio_max=2", "abierto_ahora=true", at, at + "&radio_m=1000", at + "&categoria=restaurante&precio_max=3"} {
		status, got := conteo(t, app, q)
		require.Equal(t, http.StatusOK, status, q)
		assert.Equal(t, int64(listaTotal(t, app, q)), got.Total, "conteo = lista con %q", q)
		assert.True(t, got.Exact)
	}
	for _, extra := range []string{"cursor=abc", "orden=precio", "limite=5"} {
		status, _ := conteo(t, app, extra)
		assert.Equal(t, http.StatusBadRequest, status, extra)
	}
	status, _ := conteo(t, app, "lat=19.4")
	assert.Equal(t, http.StatusUnprocessableEntity, status, "lat sin lng")
	status, _ = conteo(t, app, "categoria=inventada")
	assert.Equal(t, http.StatusUnprocessableEntity, status)
}

func TestLugaresConteo_TopeDeMil(t *testing.T) {
	env := negociosEnv(t)
	app := env.App()
	specs := make([]placeSpec, 0, negocios.MaxConteo+5)
	for i := range negocios.MaxConteo + 5 {
		specs = append(specs, placeSpec{slug: "tope-" + strconv.Itoa(i)})
	}
	insertPlaces(t, env, specs)
	status, got := conteo(t, app, "")
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, int64(negocios.MaxConteo), got.Total)
	assert.False(t, got.Exact, "«1000+»")
}

func TestLugaresAfluencia_BarrasYSuficiente(t *testing.T) {
	env := negociosEnv(t)
	app := env.App()
	places := insertPlaces(t, env, []placeSpec{{slug: "af1"}, {slug: "af2"}, {slug: "af3", state: negocios.StatusReview}})
	matrix := make([][]int, 7)
	for d := range matrix {
		matrix[d] = make([]int, 24)
	}
	matrix[2][13] = 70
	_, err := env.Mongo.DB().Collection(negocios.Collection).UpdateOne(context.Background(), bson.D{{Key: "_id", Value: places[0].ID}}, bson.D{{Key: "$set", Value: bson.D{{Key: "afluencia", Value: matrix}}}})
	require.NoError(t, err)
	get := func(id bson.ObjectID, q string) (int, negocios.TrafficResponse) {
		resp := sendJSON(t, app, http.MethodGet, "/api/v1/lugares/"+id.Hex()+"/afluencia"+q, nil, "")
		raw := readAll(t, resp)
		var out negocios.TrafficResponse
		if resp.StatusCode == http.StatusOK {
			require.NoError(t, json.Unmarshal([]byte(raw), &out), raw)
		}
		return resp.StatusCode, out
	}
	status, got := get(places[0].ID, "?dia=2")
	require.Equal(t, http.StatusOK, status)
	assert.True(t, got.Enough)
	assert.Equal(t, 70, got.Hours[13])
	_, zero := get(places[0].ID, "?dia=3")
	assert.False(t, zero.Enough)
	_, missing := get(places[1].ID, "")
	assert.False(t, missing.Enough)
	assert.Len(t, missing.Hours, 24)
	require.NotNil(t, missing.CurrentHour, "sin dia = hoy, con hora_actual")
	status, _ = get(places[0].ID, "?dia=7")
	assert.Equal(t, http.StatusUnprocessableEntity, status)
	status, _ = get(places[2].ID, "")
	assert.Equal(t, http.StatusNotFound, status, "no visible")
}

func vistaCall(t *testing.T, app *fiber.App, who amigo, id bson.ObjectID, body any) int {
	t.Helper()
	resp := sendJSON(t, app, http.MethodPost, "/api/v1/lugares/"+id.Hex()+"/vistas", body, who.token)
	_ = readAll(t, resp)
	return resp.StatusCode
}

func recientes(t *testing.T, app *fiber.App, who amigo, q string) []string {
	t.Helper()
	resp := sendJSON(t, app, http.MethodGet, "/api/v1/lugares/recientes"+q, nil, who.token)
	raw := readAll(t, resp)
	require.Equal(t, http.StatusOK, resp.StatusCode, raw)
	var out vistas.RecientesResponse
	require.NoError(t, json.Unmarshal([]byte(raw), &out))
	ids := []string{}
	for _, it := range out.Items {
		assert.False(t, it.VistoEn.IsZero())
		ids = append(ids, it.ID)
	}
	return ids
}

func TestVistas_RegistrarRecientesYBorrar(t *testing.T) {
	env, app := amistadesEnv(t)
	me := seedAmigo(t, env, "vistas_yo")
	other := seedAmigo(t, env, "vistas_otra")
	places := insertPlaces(t, env, []placeSpec{{slug: "v1"}, {slug: "v2"}, {slug: "v3"}, {slug: "v4", state: negocios.StatusReview}})
	col := env.Mongo.DB().Collection(vistas.Collection)

	assert.Equal(t, http.StatusNoContent, vistaCall(t, app, me, places[0].ID, map[string]string{"origen": "mapa"}))
	assert.Equal(t, http.StatusNoContent, vistaCall(t, app, me, places[0].ID, nil), "otra vez antes de 10 min")
	var v vistas.Vista
	require.NoError(t, col.FindOne(context.Background(), bson.D{{Key: "usuario_id", Value: me.user.ID}}).Decode(&v))
	assert.Equal(t, "mapa", v.Origen, "antes de 10 min no se escribe nada")
	first := v.Fecha
	n, err := col.CountDocuments(context.Background(), bson.D{{Key: "usuario_id", Value: me.user.ID}})
	require.NoError(t, err)
	assert.Equal(t, int64(1), n, "sin duplicar")

	// Después de 10 min sí se actualiza (se simula moviendo la fecha).
	_, err = col.UpdateOne(context.Background(), bson.D{{Key: "_id", Value: v.ID}}, bson.D{{Key: "$set", Value: bson.D{{Key: "fecha", Value: first.Add(-11 * time.Minute)}}}})
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, vistaCall(t, app, me, places[0].ID, map[string]string{"origen": "inicio"}))
	require.NoError(t, col.FindOne(context.Background(), bson.D{{Key: "_id", Value: v.ID}}).Decode(&v))
	assert.Equal(t, "inicio", v.Origen)
	assert.WithinDuration(t, time.Now(), v.ExpiraEn.Add(-vistas.Retention), time.Minute)

	time.Sleep(5 * time.Millisecond)
	assert.Equal(t, http.StatusNoContent, vistaCall(t, app, me, places[1].ID, nil))
	time.Sleep(5 * time.Millisecond)
	assert.Equal(t, http.StatusNoContent, vistaCall(t, app, me, places[2].ID, nil))
	assert.Equal(t, http.StatusNotFound, vistaCall(t, app, me, places[3].ID, nil), "no visible")
	assert.Equal(t, http.StatusNotFound, vistaCall(t, app, me, bson.NewObjectID(), nil))
	assert.Equal(t, http.StatusUnprocessableEntity, vistaCall(t, app, me, places[1].ID, map[string]string{"origen": "otro"}))

	assert.Equal(t, []string{places[2].ID.Hex(), places[1].ID.Hex(), places[0].ID.Hex()}, recientes(t, app, me, ""))
	assert.Len(t, recientes(t, app, me, "?limite=2"), 2)
	assert.Empty(t, recientes(t, app, other, ""), "nadie ve las vistas de otro")

	// Un lugar que deja de ser visible se omite.
	_, err = env.Mongo.DB().Collection(negocios.Collection).UpdateOne(context.Background(), bson.D{{Key: "_id", Value: places[1].ID}}, bson.D{{Key: "$set", Value: bson.D{{Key: "estado", Value: negocios.StatusPaused}}}})
	require.NoError(t, err)
	assert.Equal(t, []string{places[2].ID.Hex(), places[0].ID.Hex()}, recientes(t, app, me, ""))

	resp := sendJSON(t, app, http.MethodDelete, "/api/v1/lugares/recientes", nil, me.token)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.Empty(t, recientes(t, app, me, ""))
}

func TestVistas_TopePorHora(t *testing.T) {
	env, app := amistadesEnv(t)
	me := seedAmigo(t, env, "vistas_tope")
	specs := make([]placeSpec, 0, vistas.MaxPerHour+1)
	for i := range vistas.MaxPerHour + 1 {
		specs = append(specs, placeSpec{slug: "tope-vista-" + strconv.Itoa(i)})
	}
	places := insertPlaces(t, env, specs)
	for i := range vistas.MaxPerHour {
		require.Equal(t, http.StatusNoContent, vistaCall(t, app, me, places[i].ID, nil), i)
	}
	resp := sendJSON(t, app, http.MethodPost, "/api/v1/lugares/"+places[vistas.MaxPerHour].ID.Hex()+"/vistas", nil, me.token)
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.Equal(t, "3600", resp.Header.Get(fiber.HeaderRetryAfter))
	assert.Equal(t, http.StatusNoContent, vistaCall(t, app, me, places[0].ID, nil), "volver a uno ya visto no cuenta")
}

func TestPrivacidadVistas_HistorialCuentaYExportar(t *testing.T) {
	env, app := amistadesEnv(t)
	me := seedAmigo(t, env, "vistas_arco")
	places := insertPlaces(t, env, []placeSpec{{slug: "arco-v1"}, {slug: "arco-v2"}})
	for _, p := range places {
		require.Equal(t, http.StatusNoContent, vistaCall(t, app, me, p.ID, map[string]string{"origen": "busqueda"}))
	}
	priv := vistas.NewPrivacy(vistas.NewRepository(env.Mongo.DB()), negocios.NewRepository(env.Mongo.DB()))
	out, err := priv.Export(context.Background(), me.user.ID)
	require.NoError(t, err)
	raw, err := json.Marshal(out)
	require.NoError(t, err)
	assert.Contains(t, string(raw), "Lugar arco-v1")
	assert.Contains(t, string(raw), `"origen":"busqueda"`)

	col := env.Mongo.DB().Collection(vistas.Collection)
	require.NoError(t, priv.PurgeHistory(context.Background(), me.user.ID))
	n, err := col.CountDocuments(context.Background(), bson.D{{Key: "usuario_id", Value: me.user.ID}})
	require.NoError(t, err)
	assert.Zero(t, n)
	require.Equal(t, http.StatusNoContent, vistaCall(t, app, me, places[0].ID, nil))
	require.NoError(t, priv.PurgeAccount(context.Background(), me.user.ID))
	n, err = col.CountDocuments(context.Background(), bson.D{{Key: "usuario_id", Value: me.user.ID}})
	require.NoError(t, err)
	assert.Zero(t, n)
}

// nv-66: una vista cuesta lo mismo que cualquier escritura autenticada sin política propia y el
// conteo lo mismo que la lista pública (sin caché).
func TestPresupuestoRedis_VistasNV66(t *testing.T) {
	env, app := amistadesEnv(t)
	env.ClearRateLimits(t)
	me := seedAmigo(t, env, "presupuesto66")
	places := insertPlaces(t, env, []placeSpec{{slug: "pres66"}})
	write := commandsOf(t, func() {
		resp := sendJSON(t, app, http.MethodPatch, "/api/v1/usuarios/yo/privacidad", map[string]any{"datos_sensibles": false}, me.token)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
	assert.Equal(t, write, commandsOf(t, func() {
		assert.Equal(t, http.StatusNoContent, vistaCall(t, app, me, places[0].ID, nil))
	}), "vista = escritura sin política propia")
	public := commandsOf(t, func() {
		assert.Equal(t, http.StatusOK, sendJSON(t, app, http.MethodGet, "/api/v1/lugares", nil, "").StatusCode)
	})
	assert.Equal(t, public, commandsOf(t, func() {
		status, _ := conteo(t, app, "")
		assert.Equal(t, http.StatusOK, status)
	}), "conteo = lectura pública")
}

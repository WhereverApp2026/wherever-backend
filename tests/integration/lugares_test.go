package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
)

// Centro de la búsqueda: Zócalo de la CDMX. 0.001° de latitud ≈ 111 m.
const (
	centerLat = 19.4326
	centerLng = -99.1332
)

// lunesMediodia es lunes 2026-11-09 a las 12:00 en Ciudad de México.
var lunesMediodia = time.Date(2026, 11, 9, 18, 0, 0, 0, time.UTC)

// weekdays abre los siete días de 09:00 a 18:00.
func weekdays() []negocios.VentanaMin {
	out := make([]negocios.VentanaMin, 0, 7)
	for d := 0; d < 7; d++ {
		out = append(out, negocios.VentanaMin{Dia: d, Desde: 9 * 60, Hasta: 18 * 60})
	}
	return out
}

type placeSpec struct {
	slug     string
	state    string
	category string
	price    int
	avgPrice int
	rating   float64
	dLat     float64
	windows  []negocios.VentanaMin
	deleted  bool
}

func placeDoc(s placeSpec) negocios.Business {
	if s.state == "" {
		s.state = negocios.StatusApproved
	}
	if s.category == "" {
		s.category = "restaurante"
	}
	if s.price == 0 {
		s.price = 2
	}
	if s.windows == nil {
		s.windows = weekdays()
	}
	b := negocios.Business{
		ID: bson.NewObjectID(), Slug: s.slug, QRCheckin: "qr-" + s.slug, Name: "Lugar " + s.slug,
		Category: s.category, PriceLevel: s.price, AveragePrice: s.avgPrice, State: s.state,
		Location:    negocios.Location{Type: "Point", Coordinates: []float64{centerLng, centerLat + s.dLat}},
		ScheduleMin: s.windows, Rating: negocios.Calificacion{Promedio: s.rating},
		Photos: []negocios.Foto{{URL: "https://example.test/" + s.slug + "-2.jpg", Orden: 2}, {URL: "https://example.test/" + s.slug + "-1.jpg", Orden: 1}},
	}
	if s.deleted {
		now := time.Now().UTC()
		b.DeletedAt = &now
	}
	return b
}

func insertPlaces(t testing.TB, env *authtest.Env, specs []placeSpec) []negocios.Business {
	t.Helper()
	docs := make([]any, 0, len(specs))
	out := make([]negocios.Business, 0, len(specs))
	for _, s := range specs {
		b := placeDoc(s)
		docs = append(docs, b)
		out = append(out, b)
	}
	_, err := env.Mongo.DB().Collection(negocios.Collection).InsertMany(context.Background(), docs)
	require.NoError(t, err)
	return out
}

// thirtyPlaces son 25 aprobados (a 100 m de distancia entre sí, calificaciones con empates)
// más 5 que nunca deben salir: borrador, en revisión, rechazado, pausado y eliminado.
func thirtyPlaces() []placeSpec {
	specs := make([]placeSpec, 0, 30)
	categories := []string{"restaurante", "cafeteria", "bar"}
	for i := 0; i < 25; i++ {
		specs = append(specs, placeSpec{
			slug: fmt.Sprintf("lugar-%02d", i), category: categories[i%3], price: i%4 + 1,
			avgPrice: 100 + (i%5)*50, rating: float64(i%5) + 0.5, dLat: float64(i) * 0.0009,
		})
	}
	for i, state := range []string{negocios.StatusDraft, negocios.StatusReview, negocios.StatusRejected, negocios.StatusPaused} {
		specs = append(specs, placeSpec{slug: fmt.Sprintf("oculto-%d", i), state: state, rating: 5})
	}
	return append(specs, placeSpec{slug: "oculto-eliminado", rating: 5, deleted: true})
}

func lugaresEnv(t *testing.T) (*authtest.Env, *fiber.App) {
	t.Helper()
	env := negociosEnv(t)
	env.Negocios.WithClock(func() time.Time { return lunesMediodia })
	return env, env.App()
}

func listPage(t *testing.T, app *fiber.App, query url.Values) (int, pagination.PageResult[negocios.PlaceItem]) {
	t.Helper()
	resp := get(t, app, "/api/v1/lugares?"+query.Encode())
	var page pagination.PageResult[negocios.PlaceItem]
	if resp.StatusCode == http.StatusOK {
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))
	}
	return resp.StatusCode, page
}

// allPages recorre el cursor y devuelve los slugs en orden.
func allPages(t *testing.T, app *fiber.App, query url.Values) []negocios.PlaceItem {
	t.Helper()
	var items []negocios.PlaceItem
	for range 20 {
		status, page := listPage(t, app, query)
		require.Equal(t, http.StatusOK, status)
		items = append(items, page.Items...)
		if page.NextCursor == "" {
			return items
		}
		query.Set("cursor", page.NextCursor)
	}
	t.Fatal("el cursor no terminó")
	return nil
}

func slugs(items []negocios.PlaceItem) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.Slug)
	}
	return out
}

func near(extra url.Values) url.Values {
	q := url.Values{"lat": {fmt.Sprint(centerLat)}, "lng": {fmt.Sprint(centerLng)}}
	for k, v := range extra {
		q[k] = v
	}
	return q
}

func TestLugares_OnlyApprovedAndRatingOrderWithoutLocation(t *testing.T) {
	env, app := lugaresEnv(t)
	insertPlaces(t, env, thirtyPlaces())

	items := allPages(t, app, url.Values{"limite": {"7"}})
	require.Len(t, items, 25, "solo los 25 aprobados y no eliminados")
	got := slugs(items)
	assert.Len(t, slices.Compact(slices.Sorted(slices.Values(got))), 25, "el cursor no repite con empates de calificación")
	for _, it := range items {
		assert.NotContains(t, it.Slug, "oculto")
		assert.Nil(t, it.DistanceM, "sin ubicación no hay distancia")
	}
	assert.True(t, sort.SliceIsSorted(items, func(i, j int) bool {
		return items[i].Rating.Average > items[j].Rating.Average
	}), "por defecto ordena por calificación descendente")
	assert.Equal(t, "https://example.test/"+items[0].Slug+"-1.jpg", items[0].Cover)
}

func TestLugares_DistanceOrderRadiusAndCursor(t *testing.T) {
	env, app := lugaresEnv(t)
	specs := thirtyPlaces()
	// Dos negocios en el mismo punto: el cursor por distancia debe desempatar por _id.
	specs = append(specs, placeSpec{slug: "gemelo-a", dLat: 0.0009}, placeSpec{slug: "gemelo-b", dLat: 0.0009})
	insertPlaces(t, env, specs)

	items := allPages(t, app, near(url.Values{"radio_m": {"1450"}, "limite": {"4"}}))
	// 0.0009° ≈ 100 m: caben los índices 0..14 (≤ 1,450 m) más los dos gemelos.
	require.Len(t, items, 17)
	assert.Len(t, slices.Compact(slices.Sorted(slices.Values(slugs(items)))), 17)
	for i, it := range items {
		require.NotNil(t, it.DistanceM)
		assert.LessOrEqual(t, *it.DistanceM, 1450.0)
		if i > 0 {
			assert.GreaterOrEqual(t, *it.DistanceM, *items[i-1].DistanceM)
		}
	}
	assert.Equal(t, "lugar-00", items[0].Slug)
}

func TestLugares_DefaultRadiusIsFiveKm(t *testing.T) {
	env, app := lugaresEnv(t)
	insertPlaces(t, env, []placeSpec{{slug: "cerca", dLat: 0.04}, {slug: "lejos", dLat: 0.1}})
	items := allPages(t, app, near(nil))
	assert.Equal(t, []string{"cerca"}, slugs(items))

	items = allPages(t, app, near(url.Values{"radio_m": {"20000"}}))
	assert.Equal(t, []string{"cerca", "lejos"}, slugs(items))
}

func TestLugares_CombinedFilters(t *testing.T) {
	env, app := lugaresEnv(t)
	tuesdayOnly := []negocios.VentanaMin{{Dia: 2, Desde: 0, Hasta: 1440}}
	insertPlaces(t, env, []placeSpec{
		{slug: "match", category: "cafeteria", price: 2},
		{slug: "caro", category: "cafeteria", price: 4},
		{slug: "otro-giro", category: "bar", price: 1},
		{slug: "cerrado", category: "cafeteria", price: 1, windows: tuesdayOnly},
		{slug: "lejano", category: "cafeteria", price: 1, dLat: 0.5},
	})
	q := near(url.Values{"categoria": {"cafeteria"}, "precio_max": {"2"}, "abierto_ahora": {"true"}})
	items := allPages(t, app, q)
	assert.Equal(t, []string{"match"}, slugs(items))
	assert.True(t, items[0].OpenNow)
}

func TestLugares_OpenNowCrossesMidnight(t *testing.T) {
	env, app := lugaresEnv(t)
	// Viernes 20:00 a sábado 03:00, consultado el sábado a la 01:00 (07:00 UTC).
	env.Negocios.WithClock(func() time.Time { return time.Date(2026, 11, 14, 7, 0, 0, 0, time.UTC) })
	insertPlaces(t, env, []placeSpec{
		{slug: "nocturno", windows: []negocios.VentanaMin{{Dia: 5, Desde: 20 * 60, Hasta: 27 * 60}}},
		{slug: "diurno"},
	})
	items := allPages(t, app, url.Values{"abierto_ahora": {"true"}})
	assert.Equal(t, []string{"nocturno"}, slugs(items))
}

func TestLugares_PriceOrderWithCursor(t *testing.T) {
	env, app := lugaresEnv(t)
	insertPlaces(t, env, thirtyPlaces())
	items := allPages(t, app, url.Values{"orden": {"precio"}, "limite": {"6"}})
	require.Len(t, items, 25)
	assert.True(t, sort.SliceIsSorted(items, func(i, j int) bool { return items[i].AveragePrice < items[j].AveragePrice }))
}

func TestLugares_ValidationErrors(t *testing.T) {
	_, app := lugaresEnv(t)
	status, _ := listPage(t, app, url.Values{"orden": {"calificacion"}, "limite": {"1"}})
	require.Equal(t, http.StatusOK, status)

	cases := map[string]url.Values{
		"distancia sin ubicación": {"orden": {"distancia"}},
		"lat sin lng":             {"lat": {"19.4"}},
		"radio mayor a 20 km":     near(url.Values{"radio_m": {"20001"}}),
		"categoría desconocida":   {"categoria": {"spa"}},
		"precio fuera de rango":   {"precio_max": {"5"}},
		"latitud inválida":        {"lat": {"91"}, "lng": {"0"}},
		"orden desconocido":       {"orden": {"popularidad"}},
		"cursor ilegible":         {"cursor": {"no-es-cursor"}},
		"cursor de otro orden": {"orden": {"precio"}, "cursor": {pagination.Encode(pagination.Cursor{
			ID: bson.NewObjectID().Hex(), Sort: "calificacion:4.5",
		})}},
	}
	for name, q := range cases {
		t.Run(name, func(t *testing.T) {
			status, _ := listPage(t, app, q)
			assert.Equal(t, http.StatusUnprocessableEntity, status)
		})
	}
}

func TestLugares_LocationNeverLogged(t *testing.T) {
	env, app := lugaresEnv(t)
	insertPlaces(t, env, []placeSpec{{slug: "privado"}})
	status, _ := listPage(t, app, url.Values{"lat": {"19.4326789"}, "lng": {"-99.1332789"}})
	require.Equal(t, http.StatusOK, status)
	status, _ = listPage(t, app, url.Values{"lat": {"19.4326789"}})
	require.Equal(t, http.StatusUnprocessableEntity, status)
	assert.NotContains(t, env.Logs.String(), "19.4326789")
	assert.NotContains(t, env.Logs.String(), "99.1332789")
}

func TestLugares_DetailWithOpenNowAndNoControlFields(t *testing.T) {
	env, app := lugaresEnv(t)
	placed := insertPlaces(t, env, []placeSpec{{slug: "ficha", rating: 4.2}})

	resp := get(t, app, "/api/v1/lugares/"+placed[0].ID.Hex())
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body := decode(t, resp)
	assertSnakeCase(t, body)
	assert.Equal(t, "ficha", body["slug"])
	assert.Equal(t, true, body["abierto_ahora"])
	for _, hidden := range []string{"qr_checkin", "propietario_id", "revision", "plan", "version", "horario_min"} {
		assert.NotContains(t, body, hidden)
	}
}

func TestLugares_DetailHidesNonApproved(t *testing.T) {
	env, app := lugaresEnv(t)
	placed := insertPlaces(t, env, thirtyPlaces())
	for _, b := range placed {
		if b.State == negocios.StatusApproved && b.DeletedAt == nil {
			continue
		}
		resp := get(t, app, "/api/v1/lugares/"+b.ID.Hex())
		assert.Equal(t, http.StatusNotFound, resp.StatusCode, b.Slug)
	}
	assert.Equal(t, http.StatusNotFound, get(t, app, "/api/v1/lugares/no-es-id").StatusCode)
	assert.Equal(t, http.StatusNotFound, get(t, app, "/api/v1/lugares/"+bson.NewObjectID().Hex()).StatusCode)
}

func TestLugares_OptionalAuthAcceptsAnonymousAndRejectsBadToken(t *testing.T) {
	env, app := lugaresEnv(t)
	insertPlaces(t, env, []placeSpec{{slug: "anonimo"}})
	assert.Equal(t, http.StatusOK, get(t, app, "/api/v1/lugares").StatusCode)
	bad := request(t, app, http.MethodGet, "/api/v1/lugares", map[string]string{"Authorization": "Bearer no-es-jwt"})
	assert.Equal(t, http.StatusUnauthorized, bad.StatusCode)
}

// grid son n negocios aprobados en una cuadrícula de 0.002° (≈ 220 m) desde el centro.
func grid(n int) []placeSpec {
	specs := make([]placeSpec, 0, n)
	for i := 0; i < n; i++ {
		specs = append(specs, placeSpec{slug: fmt.Sprintf("pin-%03d", i), dLat: float64(i/12) * 0.002})
	}
	return specs
}

type mapReply struct {
	status int
	body   negocios.MapResponse
}

func getMap(t *testing.T, app *fiber.App, zoom int, extra url.Values) mapReply {
	t.Helper()
	q := url.Values{
		"min_lat": {"19.40"}, "max_lat": {"19.50"}, "min_lng": {"-99.20"}, "max_lng": {"-99.10"},
		"zoom": {fmt.Sprint(zoom)},
	}
	for k, v := range extra {
		q[k] = v
		if v[0] == "" {
			q.Del(k)
		}
	}
	resp := get(t, app, "/api/v1/lugares/mapa?"+q.Encode())
	out := mapReply{status: resp.StatusCode}
	if resp.StatusCode == http.StatusOK {
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&out.body))
	}
	return out
}

func TestMapa_FewPlacesArePins(t *testing.T) {
	env, app := lugaresEnv(t)
	insertPlaces(t, env, thirtyPlaces())
	reply := getMap(t, app, 14, nil)
	require.Equal(t, http.StatusOK, reply.status)
	assert.False(t, reply.body.Grouped)
	assert.Len(t, reply.body.Pins, 25, "solo aprobados")
	assert.Empty(t, reply.body.Cells)
}

func TestMapa_ManyPlacesGroupByZoom(t *testing.T) {
	env, app := lugaresEnv(t)
	insertPlaces(t, env, append(grid(negocios.MaxMapPins+20), placeSpec{slug: "borrador-mapa", state: negocios.StatusDraft}))

	count := func(cells []negocios.MapCell) int {
		total := 0
		for _, c := range cells {
			total += c.Count
		}
		return total
	}
	low := getMap(t, app, 10, nil)
	require.Equal(t, http.StatusOK, low.status)
	assert.True(t, low.body.Grouped)
	assert.Empty(t, low.body.Pins)
	assert.Equal(t, negocios.MaxMapPins+20, count(low.body.Cells), "el borrador no cuenta")

	high := getMap(t, app, 17, nil)
	require.True(t, high.body.Grouped)
	assert.Greater(t, len(high.body.Cells), len(low.body.Cells), "más zoom, celdas más chicas")
	assert.Equal(t, negocios.MaxMapPins+20, count(high.body.Cells))
}

func TestMapa_PinCapAndCategory(t *testing.T) {
	env, app := lugaresEnv(t)
	specs := grid(negocios.MaxMapPins + 1)
	specs = append(specs, placeSpec{slug: "unico-bar", category: "bar"})
	insertPlaces(t, env, specs)

	all := getMap(t, app, 15, nil)
	assert.True(t, all.body.Grouped, "más de MaxMapPins se agrupa")
	bars := getMap(t, app, 15, url.Values{"categoria": {"bar"}})
	require.False(t, bars.body.Grouped)
	require.Len(t, bars.body.Pins, 1)
	assert.Equal(t, "Lugar unico-bar", bars.body.Pins[0].Name)
}

func TestMapa_InvalidArea(t *testing.T) {
	_, app := lugaresEnv(t)
	assert.Equal(t, http.StatusUnprocessableEntity, getMap(t, app, 14, url.Values{"min_lat": {"19.6"}}).status)
	assert.Equal(t, http.StatusUnprocessableEntity, getMap(t, app, 2, nil).status)
	assert.Equal(t, http.StatusUnprocessableEntity, getMap(t, app, 14, url.Values{"max_lng": {""}}).status)
	assert.Equal(t, http.StatusUnprocessableEntity, getMap(t, app, 14, url.Values{"categoria": {"spa"}}).status)
}

// TestLugares_PerformanceP95 mide la lista con filtros y ubicación sobre 1,000 negocios.
func TestLugares_PerformanceP95(t *testing.T) {
	env, app := lugaresEnv(t)
	specs := make([]placeSpec, 0, 1000)
	categories := []string{"restaurante", "cafeteria", "bar", "entretenimiento"}
	for i := 0; i < 1000; i++ {
		specs = append(specs, placeSpec{
			slug: fmt.Sprintf("rendimiento-%04d", i), category: categories[i%4], price: i%4 + 1,
			avgPrice: 100 + i%300, rating: float64(i%50) / 10, dLat: float64(i%100)*0.0015 - 0.07,
		})
	}
	insertPlaces(t, env, specs)
	q := near(url.Values{"categoria": {"cafeteria"}, "precio_max": {"3"}, "abierto_ahora": {"true"}, "radio_m": {"20000"}})
	_, _ = listPage(t, app, q) // calienta conexiones y planes

	const runs = 40
	durations := make([]time.Duration, 0, runs)
	for range runs {
		start := time.Now()
		status, page := listPage(t, app, q)
		durations = append(durations, time.Since(start))
		require.Equal(t, http.StatusOK, status)
		require.NotEmpty(t, page.Items)
	}
	slices.Sort(durations)
	p95 := durations[runs*95/100-1]
	t.Logf("p95 GET /lugares con 1,000 negocios: %s", p95)
	assert.Less(t, p95, 150*time.Millisecond)
}

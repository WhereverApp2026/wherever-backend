package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/cupones"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/resenas"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
)

// explorar85 son cuatro lugares: a (Sello y cupón), b (bar, precio 1, cupón), c (cafetería,
// precio 3, Sello) y d (sin nada).
type explorar85 struct {
	env        *authtest.Env
	app        *fiber.App
	a, b, c, d negocios.Business
}

func nuevoExplorar85(t *testing.T) explorar85 {
	t.Helper()
	env, app := amistadesEnv(t)
	ps := insertPlaces(t, env, []placeSpec{
		{slug: "exp85-a", rating: 4.8, avgPrice: 200},
		{slug: "exp85-b", rating: 4.5, category: "bar", price: 1, dLat: 0.001},
		{slug: "exp85-c", rating: 4.2, category: "cafeteria", price: 3, dLat: 0.002},
		{slug: "exp85-d", rating: 4.0, dLat: 0.003},
	})
	col := env.Mongo.DB().Collection(negocios.Collection)
	set := func(id bson.ObjectID, fields bson.D) {
		_, err := col.UpdateByID(context.Background(), id, bson.D{{Key: "$set", Value: fields}})
		require.NoError(t, err)
	}
	set(ps[0].ID, bson.D{{Key: "sello_excelencia", Value: true}, {Key: "cupones_activos", Value: 2}, {Key: "afluencia", Value: matriz85(80)}})
	set(ps[1].ID, bson.D{{Key: "cupones_activos", Value: 1}})
	set(ps[2].ID, bson.D{{Key: "sello_excelencia", Value: true}})
	return explorar85{env: env, app: app, a: ps[0], b: ps[1], c: ps[2], d: ps[3]}
}

// matriz85 es una curva 7 × 24 con el mismo valor a toda hora.
func matriz85(v int) [][]int {
	m := make([][]int, 7)
	for d := range m {
		m[d] = make([]int, 24)
		for h := range m[d] {
			m[d][h] = v
		}
	}
	return m
}

func (e explorar85) lista(t *testing.T, query, token string) []map[string]any {
	t.Helper()
	resp := sendJSON(t, e.app, http.MethodGet, "/api/v1/lugares?limite=50&"+query, nil, token)
	raw := readAll(t, resp)
	require.Equal(t, http.StatusOK, resp.StatusCode, raw)
	var out struct {
		Items []map[string]any `json:"items"`
	}
	require.NoError(t, json.Unmarshal([]byte(raw), &out), raw)
	return out.Items
}

func ids85(t *testing.T, items []map[string]any) []string {
	t.Helper()
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, str(t, it["id"]))
	}
	slices.Sort(out)
	return out
}

func hex85(bs ...negocios.Business) []string {
	out := make([]string, 0, len(bs))
	for _, b := range bs {
		out = append(out, b.ID.Hex())
	}
	slices.Sort(out)
	return out
}

func (e explorar85) conteo(t *testing.T, query, token string) int64 {
	t.Helper()
	resp := sendJSON(t, e.app, http.MethodGet, "/api/v1/lugares/conteo?"+query, nil, token)
	raw := readAll(t, resp)
	require.Equal(t, http.StatusOK, resp.StatusCode, raw)
	var out negocios.CountResponse
	require.NoError(t, json.Unmarshal([]byte(raw), &out), raw)
	return out.Total
}

func (e explorar85) mapa(t *testing.T, query, token string) []string {
	t.Helper()
	resp := sendJSON(t, e.app, http.MethodGet, "/api/v1/lugares/mapa?min_lat=19&max_lat=20&min_lng=-100&max_lng=-99&zoom=14&"+query, nil, token)
	raw := readAll(t, resp)
	require.Equal(t, http.StatusOK, resp.StatusCode, raw)
	var out negocios.MapResponse
	require.NoError(t, json.Unmarshal([]byte(raw), &out), raw)
	ids := make([]string, 0, len(out.Pins))
	for _, p := range out.Pins {
		ids = append(ids, p.ID)
	}
	slices.Sort(ids)
	return ids
}

// Cada filtro y sus combinaciones dan lo mismo en lista, conteo y mapa («Ver N lugares»).
func TestExplorar85_FiltrosConteoYMapa(t *testing.T) {
	e := nuevoExplorar85(t)
	casos := map[string][]negocios.Business{
		"con_sello=true":                  {e.a, e.c},
		"con_cupon=true":                  {e.a, e.b},
		"categorias=bar,cafeteria":        {e.b, e.c},
		"categoria=bar&categorias=bar":    {e.b},
		"precio=1,3":                      {e.b, e.c},
		"con_sello=true&con_cupon=true":   {e.a},
		"con_sello=true&precio=3":         {e.c},
		"categorias=restaurante&precio=2": {e.a, e.d},
	}
	for query, want := range casos {
		assert.Equal(t, hex85(want...), ids85(t, e.lista(t, query, "")), query)
		assert.Equal(t, int64(len(want)), e.conteo(t, query, ""), query)
		assert.Equal(t, hex85(want...), e.mapa(t, query, ""), query)
	}
}

// La tarjeta trae Sello, afluencia de la hora y el mejor cupón vigente.
func TestExplorar85_TarjetaSelloAfluenciaYCupon(t *testing.T) {
	e := nuevoExplorar85(t)
	now := time.Now().UTC()
	cupon := func(negocio bson.ObjectID, titulo, tipo string, valor int64, inicio, fin time.Time, estado string) bson.ObjectID {
		c := cupones.Cupon{ID: bson.NewObjectID(), NegocioID: negocio, Titulo: titulo, Tipo: tipo, Valor: valor,
			VigenciaInicio: inicio, VigenciaFin: fin, Estado: estado, CreadoEn: now, ActualizadoEn: now,
			Ubicacion: cupones.Location{Type: "Point", Coordinates: []float64{-99.1, 19.4}}}
		_, err := e.env.Mongo.DB().Collection(cupones.Collection).InsertOne(context.Background(), c)
		require.NoError(t, err)
		return c.ID
	}
	dia := 24 * time.Hour
	mejor := cupon(e.a.ID, "Cupón $50", cupones.TipoMonto, 5000, now.Add(-dia), now.Add(dia), cupones.EstadoActivo)
	cupon(e.a.ID, "10 %", cupones.TipoPorcentaje, 10, now.Add(-dia), now.Add(dia), cupones.EstadoActivo) // 200 × 10 = $20
	cupon(e.a.ID, "Vencido", cupones.TipoMonto, 90000, now.Add(-2*dia), now.Add(-dia), cupones.EstadoActivo)
	cupon(e.a.ID, "Pausado", cupones.TipoMonto, 90000, now.Add(-dia), now.Add(dia), cupones.EstadoPausado)
	cupon(e.b.ID, "Mañana", cupones.TipoMonto, 90000, now.Add(dia), now.Add(2*dia), cupones.EstadoProgramado)

	items := map[string]map[string]any{}
	for _, it := range e.lista(t, "", "") {
		items[str(t, it["id"])] = it
	}
	a, b, d := items[e.a.ID.Hex()], items[e.b.ID.Hex()], items[e.d.ID.Hex()]
	assert.Equal(t, true, a["sello_excelencia"])
	assert.Equal(t, false, d["sello_excelencia"])
	assert.Equal(t, negocios.CrowdPeak, a["afluencia_ahora"])
	assert.Nil(t, d["afluencia_ahora"], "sin curva")
	assert.Equal(t, map[string]any{"id": mejor.Hex(), "titulo": "Cupón $50"}, a["cupon"])
	assert.Nil(t, b["cupon"], "el de b aún no empieza")
	assert.Nil(t, d["cupon"])
	assert.NotContains(t, a, "amigos_fueron", "sin sesión no hay amigos")
}

// «De amigos» y «amigos que fueron» solo cuentan amistades aceptadas que comparten su
// actividad, con check-in válido en los últimos 180 días.
func TestExplorar85_DeAmigos(t *testing.T) {
	e := nuevoExplorar85(t)
	db := e.env.Mongo.DB()
	ctx := context.Background()
	yo := seedAmigo(t, e.env, "yo85")
	ana := seedAmigo(t, e.env, "ana85")
	beto := seedAmigo(t, e.env, "beto85")   // amigo que oculta su actividad
	carla := seedAmigo(t, e.env, "carla85") // no es amiga
	dani := seedAmigo(t, e.env, "dani85")   // amigo con check-in viejo o no válido
	for _, f := range []amigo{ana, beto, dani} {
		befriend(t, e.env, yo, f)
	}
	_, err := e.env.Users.Update(ctx, ana.user.ID, bson.D{{Key: "nombre", Value: "Ana María López"}})
	require.NoError(t, err)
	_, err = e.env.Users.Update(ctx, beto.user.ID, bson.D{{Key: "privacidad.social.actividad", Value: "nadie"}})
	require.NoError(t, err)
	now := time.Now().UTC()
	checkin := func(who amigo, place bson.ObjectID, ago time.Duration, valid bool) {
		at := now.Add(-ago)
		_, err := db.Collection(resenas.CheckinsCollection).InsertOne(ctx, resenas.Checkin{ID: bson.NewObjectID(), UsuarioID: who.user.ID,
			NegocioID: place, Metodo: resenas.MetodoQR, Valido: valid, Dia: at.Format(time.DateOnly), CreadoEn: at})
		require.NoError(t, err)
	}
	checkin(ana, e.a.ID, time.Hour, true)
	checkin(ana, e.a.ID, 48*time.Hour, true)
	checkin(beto, e.b.ID, time.Hour, true)
	checkin(carla, e.c.ID, time.Hour, true)
	checkin(dani, e.d.ID, 200*24*time.Hour, true)
	checkin(dani, e.c.ID, time.Hour, false)

	assert.Equal(t, hex85(e.a), ids85(t, e.lista(t, "de_amigos=true", yo.token)))
	assert.Equal(t, int64(1), e.conteo(t, "de_amigos=true", yo.token))
	assert.Equal(t, hex85(e.a), e.mapa(t, "de_amigos=true", yo.token))
	assert.Empty(t, e.lista(t, "de_amigos=true&con_sello=true&categorias=bar", yo.token))
	assert.Empty(t, e.lista(t, "de_amigos=true", carla.token), "carla no tiene amigos")

	resp := sendJSON(t, e.app, http.MethodGet, "/api/v1/lugares?limite=50", nil, yo.token)
	raw := readAll(t, resp)
	require.Equal(t, http.StatusOK, resp.StatusCode, raw)
	assert.NotContains(t, raw, "Ana María", "nunca el nombre completo")
	for _, who := range []amigo{beto, carla, dani} {
		assert.NotContains(t, raw, who.id())
	}
	items := map[string]map[string]any{}
	for _, it := range e.lista(t, "", yo.token) {
		items[str(t, it["id"])] = it
	}
	assert.Equal(t, map[string]any{"total": float64(1), "iniciales": []any{map[string]any{"id": ana.id(), "iniciales": "AL"}}},
		items[e.a.ID.Hex()]["amigos_fueron"])
	for _, otro := range []negocios.Business{e.b, e.c, e.d} {
		assert.Equal(t, map[string]any{"total": float64(0), "iniciales": []any{}}, items[otro.ID.Hex()]["amigos_fueron"], otro.Slug)
	}
}

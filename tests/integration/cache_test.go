package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/cache"
	platformredis "github.com/WhereverApp2026/wherever-backend/internal/platform/redis"
)

// Caché 6 (nv-18): acierto y fallo, avalancha, invalidación al editar y Redis caído.

func fichaNombre(t *testing.T, app *fiber.App, id string) string {
	t.Helper()
	nombre, ok := getFicha(t, app, id)["nombre"].(string)
	require.True(t, ok)
	return nombre
}

func getFicha(t *testing.T, app *fiber.App, id string) map[string]any {
	t.Helper()
	resp := get(t, app, "/api/v1/lugares/"+id)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	return decode(t, resp)
}

func TestCache_FichaAciertoYFallo(t *testing.T) {
	env, app := lugaresEnv(t)
	placed := insertPlaces(t, env, []placeSpec{{slug: "cacheada"}})
	id := placed[0].ID.Hex()
	ctx := context.Background()

	require.Equal(t, "Lugar cacheada", fichaNombre(t, app, id))
	ttl := redisClient.TTL(ctx, "cache:negocio:"+id).Val()
	assert.Greater(t, ttl, time.Duration(0), "la ficha se guarda con TTL")
	assert.LessOrEqual(t, ttl, negocios.VidaFicha)

	// Un cambio directo en MongoDB no se ve mientras la ficha siga en la caché.
	_, err := env.Mongo.DB().Collection(negocios.Collection).UpdateByID(ctx, placed[0].ID,
		bson.D{{Key: "$set", Value: bson.D{{Key: "nombre", Value: "Cambiado"}}}})
	require.NoError(t, err)
	assert.Equal(t, "Lugar cacheada", fichaNombre(t, app, id), "acierto: responde la caché")

	require.NoError(t, redisClient.Del(ctx, "cache:negocio:"+id).Err())
	assert.Equal(t, "Cambiado", fichaNombre(t, app, id), "fallo: responde MongoDB")
}

func TestCache_AbiertoAhoraNoSeCongela(t *testing.T) {
	env, app := lugaresEnv(t)
	placed := insertPlaces(t, env, []placeSpec{{slug: "horario"}})
	id := placed[0].ID.Hex()

	assert.Equal(t, true, getFicha(t, app, id)["abierto_ahora"])
	env.Negocios.WithClock(func() time.Time { return lunesMediodia.Add(10 * time.Hour) })
	assert.Equal(t, false, getFicha(t, app, id)["abierto_ahora"], "se recalcula con la ficha guardada")
}

func TestCache_AvalanchaUnaConsultaAlOrigen(t *testing.T) {
	negociosEnv(t) // deja Redis vacío
	c := cache.New(redisClient, nil)
	var consultas atomic.Int32
	soltar := make(chan struct{})
	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			v, err := cache.Obtener(context.Background(), c, cache.Clave("prueba", "avalancha"), time.Minute,
				func(context.Context) (string, error) {
					consultas.Add(1)
					<-soltar
					return "origen", nil
				})
			assert.NoError(t, err)
			assert.Equal(t, "origen", v)
		})
	}
	time.Sleep(200 * time.Millisecond)
	close(soltar)
	wg.Wait()
	assert.Equal(t, int32(1), consultas.Load())
	assert.Equal(t, `"origen"`, redisClient.Get(context.Background(), "cache:prueba:avalancha").Val())
}

func TestCache_ImportarInvalidaFichaYCategorias(t *testing.T) {
	env, app := lugaresEnv(t)
	admin := env.CreateAdmin(t, "admin-cache@wherever.test")
	ctx := context.Background()

	_, err := env.Negocios.Import(ctx, admin.ID.Hex(), strings.NewReader(csvWithRows(csvValueRow("editable", "Antes", "bar", nil))), "csv")
	require.NoError(t, err)
	id := findBusiness(t, env, "editable").ID.Hex()
	require.Equal(t, "Antes", fichaNombre(t, app, id))
	require.Equal(t, 1, categoriaConteo(t, app, "bar"))

	report, err := env.Negocios.Import(ctx, admin.ID.Hex(), strings.NewReader(csvWithRows(
		csvValueRow("editable", "Despues", "bar", nil),
		csvValueRow("nuevo-bar", "Nuevo", "bar", nil))), "csv")
	require.NoError(t, err)
	require.Equal(t, 1, report.Updated)
	assert.Equal(t, "Despues", fichaNombre(t, app, id), "la reimportación borra la ficha guardada")
	assert.Equal(t, 2, categoriaConteo(t, app, "bar"), "y las categorías")
}

func TestCache_RedisCaidoRespondeMongo(t *testing.T) {
	env, _ := lugaresEnv(t)
	placed := insertPlaces(t, env, []placeSpec{{slug: "sin-redis"}})
	caido, err := platformredis.New("127.0.0.1:1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = caido.Close() })
	env.Negocios.WithCache(cache.New(caido, env.Log))

	for range 2 {
		detalle, err := env.Negocios.Detail(context.Background(), placed[0].ID.Hex())
		require.NoError(t, err)
		assert.Equal(t, "Lugar sin-redis", detalle.Name)
	}
	facets, err := env.Negocios.Facets(context.Background())
	require.NoError(t, err)
	assert.Len(t, facets.Categories, 6)
	assert.Contains(t, env.Logs.String(), "caché de Redis apagada")
	assert.Equal(t, 1, strings.Count(env.Logs.String(), "caché de Redis apagada"), "un aviso por pausa")
}

func categoriaConteo(t *testing.T, app *fiber.App, id string) int {
	t.Helper()
	resp := get(t, app, "/api/v1/categorias")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var facets negocios.FacetsResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&facets))
	for _, item := range facets.Categories {
		if item.ID == id {
			return item.Count
		}
	}
	return 0
}

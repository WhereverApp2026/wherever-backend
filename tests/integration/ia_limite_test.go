package integration

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/ia"
)

// Límites de la IA (nv-28) con Redis y MongoDB reales. Cada prueba usa su propia fecha para
// que las ventanas del minuto y del día no se compartan.
func guardiaLimite(t *testing.T, ahora time.Time, lim ia.Limitador) (*ia.Guardia, *modeloIA) {
	t.Helper()
	m := &modeloIA{salida: `{"ids":["n1","n2"]}`}
	g, err := ia.NewGuardia(m, ia.NewContadorRedis(redisClient), ia.NewRegistroMongo(mongoClient.DB()), ia.NewCacheRedis(redisClient),
		ia.GuardiaConfig{TopeUSDMes: 5, Limites: ia.Limites{PorHora: 5, PorMinuto: 12, PorDia: 400}, Limitador: lim,
			Ahora: func() time.Time { return ahora }},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)
	return g, m
}

// generarIA pide una generación con una frase distinta cada vez, para que no la conteste la caché.
func generarIA(ctx context.Context, t *testing.T, g *ia.Guardia, n int) error {
	t.Helper()
	var out idsIA
	_, err := g.GenerarValidado(ctx, peticionIA(t, fmt.Sprintf("tacos %d %s", n, t.Name())), &out)
	return err
}

func TestIALimite_SextaGeneracionDa429ConRegreso(t *testing.T) {
	ahora := time.Date(2032, 1, 5, 10, 0, 0, 0, time.UTC)
	g, m := guardiaLimite(t, ahora, ia.NewLimitadorRedis(redisClient))
	usuario := bson.NewObjectID()
	ctx := ia.ConUsuario(context.Background(), usuario)
	for i := range 5 {
		require.NoError(t, generarIA(ctx, t, g, i))
	}

	err := generarIA(ctx, t, g, 5)

	var lim *ia.LimiteUsuarioError
	require.ErrorAs(t, err, &lim)
	assert.Equal(t, 5, m.llamadas, "la sexta no llama al proveedor")
	assert.Greater(t, lim.RetryAfter(), 3500)
	assert.LessOrEqual(t, lim.RetryAfter(), 3600)
	assert.WithinDuration(t, ahora.Add(time.Hour), lim.Regreso, 2*time.Minute)
}

func TestIALimite_RegresoSaleDeLaVidaDeLaLlave(t *testing.T) {
	ahora := time.Date(2032, 1, 6, 10, 0, 0, 0, time.UTC)
	g, _ := guardiaLimite(t, ahora, ia.NewLimitadorRedis(redisClient))
	usuario := bson.NewObjectID()
	ctx := context.Background()
	llave := "rate:ia_usuario:" + usuario.Hex()
	// La hora del usuario empezó hace 40 minutos y ya lleva 5.
	require.NoError(t, redisClient.Set(ctx, llave, 5, 20*time.Minute).Err())

	err := generarIA(ia.ConUsuario(ctx, usuario), t, g, 0)

	var lim *ia.LimiteUsuarioError
	require.ErrorAs(t, err, &lim)
	assert.InDelta(t, 1200, lim.RetryAfter(), 5, "regresa cuando vence la llave, no una hora después")
}

func TestIALimite_OtroUsuarioNoSeAfecta(t *testing.T) {
	g, _ := guardiaLimite(t, time.Date(2032, 1, 7, 10, 0, 0, 0, time.UTC), ia.NewLimitadorRedis(redisClient))
	lleno := ia.ConUsuario(context.Background(), bson.NewObjectID())
	for i := range 5 {
		require.NoError(t, generarIA(lleno, t, g, i))
	}
	require.ErrorIs(t, generarIA(lleno, t, g, 5), ia.ErrLimiteUsuario)

	require.NoError(t, generarIA(ia.ConUsuario(context.Background(), bson.NewObjectID()), t, g, 6))
}

func TestIALimite_GlobalPorMinuto(t *testing.T) {
	ahora := time.Date(2032, 1, 8, 10, 0, 0, 0, time.UTC)
	g, m := guardiaLimite(t, ahora, ia.NewLimitadorRedis(redisClient))
	ctx := context.Background()
	for range 12 {
		_, err := g.Generar(ctx, peticionIA(t, "café"))
		require.NoError(t, err)
	}

	_, err := g.Generar(ctx, peticionIA(t, "café"))

	require.ErrorIs(t, err, ia.ErrLimiteGlobal)
	assert.Equal(t, 12, m.llamadas)
	vida := redisClient.TTL(ctx, "rate:ia_min:203201081000").Val()
	assert.Positive(t, vida)
	assert.LessOrEqual(t, vida, time.Minute)
}

func TestIALimite_GlobalPorDiaDelPacifico(t *testing.T) {
	// 2032-01-09 06:00 UTC son las 22:00 del 8 en Los Ángeles.
	ahora := time.Date(2032, 1, 9, 6, 0, 0, 0, time.UTC)
	g, m := guardiaLimite(t, ahora, ia.NewLimitadorRedis(redisClient))
	ctx := context.Background()
	require.NoError(t, redisClient.Set(ctx, "rate:ia_dia:2032-01-08", 400, 25*time.Hour).Err())

	_, err := g.Generar(ctx, peticionIA(t, "café"))

	require.ErrorIs(t, err, ia.ErrLimiteGlobal)
	assert.Zero(t, m.llamadas)
	assert.Zero(t, redisClient.Exists(ctx, "rate:ia_dia:2032-01-09").Val(), "el día es el del proveedor, no el de UTC")
}

func TestIALimite_RedisCaidoCierra(t *testing.T) {
	caido := goredis.NewClient(&goredis.Options{Addr: "127.0.0.1:1", DialTimeout: 200 * time.Millisecond, MaxRetries: -1})
	t.Cleanup(func() { _ = caido.Close() })
	g, m := guardiaLimite(t, time.Date(2032, 1, 10, 10, 0, 0, 0, time.UTC), ia.NewLimitadorRedis(caido))

	_, err := g.Generar(context.Background(), peticionIA(t, "café"))
	require.ErrorIs(t, err, ia.ErrLimiteDesconocido)
	err = generarIA(ia.ConUsuario(context.Background(), bson.NewObjectID()), t, g, 0)
	require.ErrorIs(t, err, ia.ErrLimiteDesconocido)

	assert.Zero(t, m.llamadas, "sin Redis no se llama al proveedor")
}

func TestIALimite_RechazoQuedaRegistrado(t *testing.T) {
	ahora := time.Date(2032, 1, 11, 10, 0, 0, 0, time.UTC)
	g, _ := guardiaLimite(t, ahora, ia.NewLimitadorRedis(redisClient))
	usuario := bson.NewObjectID()
	ctx := ia.ConUsuario(context.Background(), usuario)
	for i := range 6 {
		_ = generarIA(ctx, t, g, i)
	}

	var uso ia.Uso
	err := mongoClient.DB().Collection(ia.ColeccionUsos).FindOne(context.Background(),
		bson.D{{Key: "usuario_id", Value: usuario}, {Key: "resultado", Value: ia.ResultadoLimiteUsuario}}).Decode(&uso)
	require.NoError(t, err)
	assert.Equal(t, ia.Mes(ahora), uso.Mes)
	assert.Zero(t, uso.TokensEntrada)
}

func TestIALimite_LlavesConVida(t *testing.T) {
	ahora := time.Date(2032, 1, 12, 10, 0, 0, 0, time.UTC)
	g, _ := guardiaLimite(t, ahora, ia.NewLimitadorRedis(redisClient))
	usuario := bson.NewObjectID()
	require.NoError(t, generarIA(ia.ConUsuario(context.Background(), usuario), t, g, 0))

	ctx := context.Background()
	for llave, maxima := range map[string]time.Duration{
		"rate:ia_usuario:" + usuario.Hex(): time.Hour,
		"rate:ia_min:203201121000":         time.Minute,
		"rate:ia_dia:2032-01-12":           25 * time.Hour,
	} {
		assert.Equal(t, "1", redisClient.Get(ctx, llave).Val(), llave)
		vida := redisClient.TTL(ctx, llave).Val()
		assert.Positive(t, vida, "constitución §6: toda llave con TTL (%s)", llave)
		assert.LessOrEqual(t, vida, maxima, llave)
	}
}

// TestPresupuestoRedis_LimitesIA fija el costo de los límites (nv-28): tres comandos por
// ventana en un Pipeline. Si sube, rehaz la estimación de specs/nv-28.
func TestPresupuestoRedis_LimitesIA(t *testing.T) {
	lim := ia.NewLimitadorRedis(redisClient)
	ctx := context.Background()

	usuario := commandsOf(t, func() {
		_, err := lim.Tomar(ctx, ia.Ventana{Llave: "rate:ia_usuario:presupuesto", Vida: time.Hour})
		require.NoError(t, err)
	})
	global := commandsOf(t, func() {
		_, err := lim.Tomar(ctx, ia.Ventana{Llave: "rate:ia_min:presupuesto", Vida: time.Minute},
			ia.Ventana{Llave: "rate:ia_dia:presupuesto", Vida: 25 * time.Hour})
		require.NoError(t, err)
	})

	assert.Equal(t, 3, usuario, "límite por usuario: SET NX, INCR y PTTL")
	assert.Equal(t, 6, global, "límite global: tres por ventana")
}

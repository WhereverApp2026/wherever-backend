// Package cache guarda lecturas de catálogo en Redis (nv-18) bajo `cache:{entidad}:{id}`
// (constitución §6), con TTL siempre.
//
// Presupuesto: un acierto es un GET; un fallo, GET + SET. Una invalidación es un solo DEL
// con todas sus llaves. Si Redis falla (caído o cuota de Upstash agotada), la caché se
// apaga por Pausa y responde el origen: la API nunca falla por la caché.
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync/atomic"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

const (
	// Prefijo de todas las llaves de esta caché.
	Prefijo = "cache:"
	// Pausa es cuánto se apaga la caché tras un error de Redis: no gasta comandos ni espera
	// tiempos de espera mientras Redis no responde.
	Pausa = 30 * time.Second
	// plazoCarga acota la consulta al origen compartida por varias peticiones: no depende de
	// que la primera petición siga viva.
	plazoCarga = 10 * time.Second
)

// Cache es segura para uso concurrente. Un *Cache nil no cachea: llama siempre al origen.
type Cache struct {
	rdb   *goredis.Client
	log   *slog.Logger
	grupo singleflight.Group
	now   func() time.Time
	// apagadaHasta es un instante en nanosegundos Unix; 0 = encendida.
	apagadaHasta atomic.Int64
}

// New recibe el cliente de Redis y el logger por constructor.
func New(rdb *goredis.Client, log *slog.Logger) *Cache {
	if log == nil {
		log = slog.Default()
	}
	return &Cache{rdb: rdb, log: log, now: time.Now}
}

// Clave arma `cache:{entidad}:{id}`.
func Clave(entidad, id string) string { return Prefijo + entidad + ":" + id }

// Obtener devuelve el valor de `clave` o lo calcula con `fn` y lo guarda por `ttl`. Las
// peticiones simultáneas de la misma clave en una instancia comparten una sola llamada a
// `fn` (evita avalanchas); comparten también el valor, que no debe modificarse. Un error de
// `fn` no se guarda.
func Obtener[T any](ctx context.Context, c *Cache, clave string, ttl time.Duration, fn func(context.Context) (T, error)) (T, error) {
	if c == nil || c.rdb == nil {
		return fn(ctx)
	}
	v, err, _ := c.grupo.Do(clave, func() (any, error) {
		carga, cancel := context.WithTimeout(context.WithoutCancel(ctx), plazoCarga)
		defer cancel()
		return cargar(carga, c, clave, ttl, fn)
	})
	valor, _ := v.(T) // siempre T: lo devuelve cargar
	return valor, err
}

func cargar[T any](ctx context.Context, c *Cache, clave string, ttl time.Duration, fn func(context.Context) (T, error)) (T, error) {
	if guardado, ok := c.leer(ctx, clave); ok {
		var v T
		if err := json.Unmarshal(guardado, &v); err == nil {
			return v, nil
		}
	}
	v, err := fn(ctx)
	if err != nil {
		return v, err
	}
	if datos, err := json.Marshal(v); err == nil {
		c.guardar(ctx, clave, datos, ttl)
	}
	return v, nil
}

// Invalidar borra las llaves con un solo DEL. Si Redis falla, lo guardado vence por su TTL.
func (c *Cache) Invalidar(ctx context.Context, claves ...string) {
	if c == nil || c.rdb == nil || len(claves) == 0 || c.apagada() {
		return
	}
	if err := c.rdb.Del(ctx, claves...).Err(); err != nil {
		c.apagar(err)
	}
}

func (c *Cache) leer(ctx context.Context, clave string) ([]byte, bool) {
	if c.apagada() {
		return nil, false
	}
	v, err := c.rdb.Get(ctx, clave).Bytes()
	if errors.Is(err, goredis.Nil) {
		return nil, false
	}
	if err != nil {
		c.apagar(err)
		return nil, false
	}
	return v, true
}

func (c *Cache) guardar(ctx context.Context, clave string, datos []byte, ttl time.Duration) {
	if c.apagada() {
		return
	}
	if err := c.rdb.Set(ctx, clave, datos, ttl).Err(); err != nil {
		c.apagar(err)
	}
}

func (c *Cache) apagada() bool {
	hasta := c.apagadaHasta.Load()
	return hasta != 0 && c.now().UnixNano() < hasta
}

// apagar registra el error una vez por pausa, sin la llave (puede llevar un id).
func (c *Cache) apagar(err error) {
	antes := c.apagadaHasta.Load()
	if c.apagadaHasta.CompareAndSwap(antes, c.now().Add(Pausa).UnixNano()) && (antes == 0 || c.now().UnixNano() >= antes) {
		c.log.Warn("caché de Redis apagada; responde el origen", "dependencia", "redis", "error", err.Error(), "pausa_s", int(Pausa.Seconds()))
	}
}

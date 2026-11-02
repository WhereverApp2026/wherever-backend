package middleware

import (
	"context"
	"log/slog"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v2"
	goredis "github.com/redis/go-redis/v9"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

// Políticas con llave `rate:{politica}:{sujeto}` (constitución §6, docs/rate-limit-matrix.md).
const (
	PolicyGlobal = "global"
	PolicyWrites = "writes"

	// writesTTL evita que un contador de escrituras quede vivo si una instancia muere a medias.
	writesTTL       = 2 * time.Minute
	releaseTimeout  = 300 * time.Millisecond
	memorySweepSize = 10000
	// redisCooldown: tras un fallo, el limitador no vuelve a intentar Redis durante este
	// tiempo, para no sumar la espera de conexión a cada petición.
	redisCooldown = 5 * time.Second
)

// Limiter cuenta en Redis. Si Redis falla, cuenta en memoria de esta instancia y lo
// avisa en el log: es el único estado que se permite en memoria (constitución §2.1).
type Limiter struct {
	redis    *goredis.Client
	log      *slog.Logger
	memory   *memoryStore
	degraded atomic.Bool
	// downUntil (UnixNano) marca hasta cuándo se usa la memoria sin probar Redis.
	downUntil atomic.Int64
}

// NewLimiter crea el limitador sobre el cliente de Redis dado.
func NewLimiter(rdb *goredis.Client, log *slog.Logger) *Limiter {
	return &Limiter{redis: rdb, log: log, memory: newMemoryStore()}
}

// Reset limpia el respaldo en memoria (para pruebas).
func (l *Limiter) Reset() {
	l.memory.reset()
	l.degraded.Store(false)
	l.downUntil.Store(0)
}

func (l *Limiter) redisAvailable() bool {
	return time.Now().UnixNano() >= l.downUntil.Load()
}

// Window limita a `limit` peticiones por ventana fija y por IP.
func (l *Limiter) Window(policy string, limit int, window time.Duration) fiber.Handler {
	return func(c *fiber.Ctx) error {
		key := "rate:" + policy + ":" + ClientIP(c)
		count, resetIn := l.hit(c.UserContext(), key, window)
		if count > int64(limit) {
			return apperr.RateLimited(retryAfterSeconds(resetIn))
		}
		return c.Next()
	}
}

// Concurrent limita las escrituras simultáneas (POST/PUT/PATCH/DELETE) por sujeto.
func (l *Limiter) Concurrent(policy string, limit int) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if !isWrite(c.Method()) {
			return c.Next()
		}
		key := "rate:" + policy + ":" + ClientIP(c)
		count, release := l.acquire(c.UserContext(), key)
		defer release()
		if count > int64(limit) {
			return apperr.RateLimited(1)
		}
		return c.Next()
	}
}

func (l *Limiter) hit(ctx context.Context, key string, window time.Duration) (int64, time.Duration) {
	if !l.redisAvailable() {
		return l.memory.hit(key, window)
	}
	pipe := l.redis.TxPipeline()
	incr := pipe.Incr(ctx, key)
	pipe.ExpireNX(ctx, key, window)
	ttl := pipe.PTTL(ctx, key)
	if _, err := pipe.Exec(ctx); err != nil {
		l.markDegraded(err)
		return l.memory.hit(key, window)
	}
	l.markHealthy()
	return incr.Val(), ttl.Val()
}

func (l *Limiter) acquire(ctx context.Context, key string) (int64, func()) {
	if !l.redisAvailable() {
		return l.memory.acquire(key), func() { l.memory.release(key) }
	}
	pipe := l.redis.TxPipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, writesTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		l.markDegraded(err)
		return l.memory.acquire(key), func() { l.memory.release(key) }
	}
	l.markHealthy()
	return incr.Val(), func() {
		// El contexto de la petición puede estar vencido; se libera con uno propio y corto.
		releaseCtx, cancel := context.WithTimeout(context.Background(), releaseTimeout)
		defer cancel()
		if err := l.redis.Decr(releaseCtx, key).Err(); err != nil {
			l.log.Warn("no se pudo liberar el contador de escrituras; vence por TTL", "llave", key, "error", err.Error())
		}
	}
}

func (l *Limiter) markDegraded(err error) {
	l.downUntil.Store(time.Now().Add(redisCooldown).UnixNano())
	if !l.degraded.Swap(true) {
		l.log.Warn("Redis no responde: el rate limit pasa a memoria por instancia (límite efectivo N × límite)",
			"error", err.Error())
	}
}

func (l *Limiter) markHealthy() {
	if l.degraded.Swap(false) {
		l.log.Info("Redis respondió de nuevo: el rate limit vuelve a Redis")
	}
}

func isWrite(method string) bool {
	switch method {
	case fiber.MethodPost, fiber.MethodPut, fiber.MethodPatch, fiber.MethodDelete:
		return true
	}
	return false
}

func retryAfterSeconds(d time.Duration) int {
	if d <= 0 {
		return 1
	}
	return int(math.Ceil(d.Seconds()))
}

type memoryEntry struct {
	count   int64
	resetAt time.Time
}

// memoryStore es el respaldo por instancia; con mutex y Reset (AGENTS.md §3).
type memoryStore struct {
	mu      sync.Mutex
	windows map[string]memoryEntry
	active  map[string]int64
}

func newMemoryStore() *memoryStore {
	return &memoryStore{windows: map[string]memoryEntry{}, active: map[string]int64{}}
}

func (m *memoryStore) reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.windows = map[string]memoryEntry{}
	m.active = map[string]int64{}
}

func (m *memoryStore) hit(key string, window time.Duration) (int64, time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	if len(m.windows) > memorySweepSize {
		m.sweep(now)
	}
	entry, ok := m.windows[key]
	if !ok || !now.Before(entry.resetAt) {
		entry = memoryEntry{resetAt: now.Add(window)}
	}
	entry.count++
	m.windows[key] = entry
	return entry.count, entry.resetAt.Sub(now)
}

func (m *memoryStore) sweep(now time.Time) {
	for key, entry := range m.windows {
		if !now.Before(entry.resetAt) {
			delete(m.windows, key)
		}
	}
}

func (m *memoryStore) acquire(key string) int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.active[key]++
	return m.active[key]
}

func (m *memoryStore) release(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.active[key]--; m.active[key] <= 0 {
		delete(m.active, key)
	}
}

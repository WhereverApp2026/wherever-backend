package vaca

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// Candado del recálculo (constitución §6: `lock:{recurso}`, con TTL). Sin Redis no se escribe
// (fail-closed). Costo en Upstash: SET NX + liberar = 2 comandos por escritura sin espera.
const (
	lockPrefix = "lock:vaca:"
	lockTTL    = 10 * time.Second
	lockWait   = 3 * time.Second
	lockRetry  = 50 * time.Millisecond
)

// ErrLockBusy indica que otra petición tuvo el candado durante toda la espera.
var ErrLockBusy = errors.New("candado ocupado")

// Locker toma el candado de un grupo y devuelve con qué soltarlo.
type Locker interface {
	Lock(ctx context.Context, groupID string) (release func(), err error)
}

// releaseScript borra el candado solo si sigue siendo nuestro (un comando: EVALSHA).
var releaseScript = goredis.NewScript(`if redis.call("get", KEYS[1]) == ARGV[1] then return redis.call("del", KEYS[1]) else return 0 end`)

// RedisLocker es el candado real en Redis.
type RedisLocker struct {
	rdb *goredis.Client
}

// NewRedisLocker recibe el cliente por constructor.
func NewRedisLocker(rdb *goredis.Client) *RedisLocker { return &RedisLocker{rdb: rdb} }

// Lock reintenta cada 50 ms hasta 3 s. Error de Redis = error (el service responde 503).
func (l *RedisLocker) Lock(ctx context.Context, groupID string) (func(), error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return nil, err
	}
	key, token := lockPrefix+groupID, hex.EncodeToString(buf)
	deadline := time.Now().Add(lockWait)
	for {
		ok, err := l.rdb.SetNX(ctx, key, token, lockTTL).Result()
		if err != nil {
			return nil, err
		}
		if ok {
			return func() {
				// Si falla, el TTL lo suelta; el contexto puede venir cancelado.
				_ = releaseScript.Run(context.WithoutCancel(ctx), l.rdb, []string{key}, token).Err()
			}, nil
		}
		if time.Now().After(deadline) {
			return nil, ErrLockBusy
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(lockRetry):
		}
	}
}

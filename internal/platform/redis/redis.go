// Package redis crea el cliente de Redis con tiempos cortos (spec nv-01: ≤ 500 ms).
package redis

import (
	"context"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const (
	dialTimeout = 300 * time.Millisecond
	ioTimeout   = 300 * time.Millisecond
	poolTimeout = 500 * time.Millisecond
	maxRetries  = 0
	// go-redis reintenta conectar 5 veces por defecto; con Redis caído eso frena cada petición.
	dialRetries  = 1
	urlSchemeSep = "://"
)

// New crea el cliente desde `host:puerto` o una URL `redis://` / `rediss://`.
func New(addr string) (*goredis.Client, error) {
	opts := &goredis.Options{Addr: addr}
	if strings.Contains(addr, urlSchemeSep) {
		parsed, err := goredis.ParseURL(addr)
		if err != nil {
			return nil, err
		}
		opts = parsed
	}
	opts.DialTimeout = dialTimeout
	opts.ReadTimeout = ioTimeout
	opts.WriteTimeout = ioTimeout
	opts.PoolTimeout = poolTimeout
	opts.MaxRetries = maxRetries
	opts.DialerRetries = dialRetries
	return goredis.NewClient(opts), nil
}

// Pinger adapta el cliente a la verificación de /ready.
type Pinger struct {
	Client *goredis.Client
}

// Ping confirma que Redis responde.
func (p Pinger) Ping(ctx context.Context) error {
	return p.Client.Ping(ctx).Err()
}

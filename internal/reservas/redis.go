package reservas

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// Llaves de Redis (constitución §6), todas con TTL. Redis caído = 503 (fail-closed).
// Costo en Upstash por POST /reservas nuevo: candado SET NX + liberar EVALSHA (2) e
// idempotencia SET NX + SET XX (2); el limitador `reservas` suma su ventana.
const (
	lockPrefix = "lock:reserva:"
	lockTTL    = 5 * time.Second
	lockWait   = 3 * time.Second
	lockRetry  = 50 * time.Millisecond

	idemPrefix  = "idem:reserva:"
	idemTTL     = 24 * time.Hour
	idemEnCurso = "en_curso"
)

// ErrLockBusy indica que otra petición tuvo el candado durante toda la espera.
var ErrLockBusy = errors.New("candado ocupado")

// Locker toma el candado de un horario y devuelve con qué soltarlo.
type Locker interface {
	Lock(ctx context.Context, recurso string) (release func(), err error)
}

// Idempotencia recuerda qué reserva creó cada `Idempotency-Key`.
type Idempotencia interface {
	// Tomar aparta la llave; si ya existía devuelve su valor (id de reserva o "en_curso").
	Tomar(ctx context.Context, llave string) (previo string, err error)
	// Guardar deja el id de la reserva creada.
	Guardar(ctx context.Context, llave, reservaID string) error
	// Soltar libera la llave si la creación falló (se puede reintentar).
	Soltar(ctx context.Context, llave string)
}

var releaseScript = goredis.NewScript(`if redis.call("get", KEYS[1]) == ARGV[1] then return redis.call("del", KEYS[1]) else return 0 end`)

// Redis es el candado y la idempotencia reales.
type Redis struct{ rdb *goredis.Client }

// NewRedis recibe el cliente por constructor.
func NewRedis(rdb *goredis.Client) *Redis { return &Redis{rdb: rdb} }

// Lock reintenta cada 50 ms hasta 3 s. Error de Redis = error (el service responde 503).
func (l *Redis) Lock(ctx context.Context, recurso string) (func(), error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return nil, err
	}
	key, token := lockPrefix+recurso, hex.EncodeToString(buf)
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

// Tomar aparta la llave con SET NX; solo si ya estaba hace un GET (la repetición). No usa
// SET NX GET: Upstash y Redis 6 no lo aceptan.
func (l *Redis) Tomar(ctx context.Context, llave string) (string, error) {
	ok, err := l.rdb.SetNX(ctx, idemPrefix+llave, idemEnCurso, idemTTL).Result()
	if err != nil || ok {
		return "", err
	}
	prev, err := l.rdb.Get(ctx, idemPrefix+llave).Result()
	if errors.Is(err, goredis.Nil) {
		// Venció entre los dos comandos: se trata como en curso y la app reintenta.
		return idemEnCurso, nil
	}
	return prev, err
}

// Guardar reemplaza "en_curso" por el id, sin tocar el TTL.
func (l *Redis) Guardar(ctx context.Context, llave, reservaID string) error {
	return l.rdb.SetArgs(context.WithoutCancel(ctx), idemPrefix+llave, reservaID, goredis.SetArgs{Mode: "XX", KeepTTL: true}).Err()
}

// Soltar borra la llave; si falla, vence en 24 h.
func (l *Redis) Soltar(ctx context.Context, llave string) {
	_ = l.rdb.Del(context.WithoutCancel(ctx), idemPrefix+llave).Err()
}

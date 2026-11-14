package pagos

import (
	"context"
	"errors"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// Llaves de Redis (constitución §6), todas con TTL. Redis caído = 503 (fail-closed).
// Costo en Upstash: 2 comandos por intento de cobro (SET NX + SET XX) y 1 por evento del
// webhook (SET NX); el corte semanal corre dentro del candado del trabajo nocturno.
const (
	idemPagoPrefix   = "idem:pago:"
	idemPagoTTL      = 24 * time.Hour
	idemEventoPrefix = "idem:stripe_evento:"
	// Stripe reintenta un evento hasta 3 días.
	idemEventoTTL = 7 * 24 * time.Hour
	idemEnCurso   = "en_curso"
)

// Idempotencia guarda las llaves de la app y los eventos de Stripe ya recibidos.
type Idempotencia interface {
	// TomarPago aparta `Idempotency-Key`; si ya existía devuelve su valor (pago_id o "en_curso").
	TomarPago(ctx context.Context, llave string) (string, error)
	GuardarPago(ctx context.Context, llave, pagoID string) error
	SoltarPago(ctx context.Context, llave string)
	// TomarEvento dice si el evento es nuevo (SET NX).
	TomarEvento(ctx context.Context, id string) (bool, error)
	SoltarEvento(ctx context.Context, id string)
}

// Redis es la idempotencia real.
type Redis struct{ rdb *goredis.Client }

// NewRedis recibe el cliente por constructor.
func NewRedis(rdb *goredis.Client) *Redis { return &Redis{rdb: rdb} }

// TomarPago usa SET NX y solo al repetir un GET (Upstash no acepta SET NX GET).
func (r *Redis) TomarPago(ctx context.Context, llave string) (string, error) {
	ok, err := r.rdb.SetNX(ctx, idemPagoPrefix+llave, idemEnCurso, idemPagoTTL).Result()
	if err != nil || ok {
		return "", err
	}
	prev, err := r.rdb.Get(ctx, idemPagoPrefix+llave).Result()
	if errors.Is(err, goredis.Nil) {
		return idemEnCurso, nil
	}
	return prev, err
}

// GuardarPago reemplaza "en_curso" por el id del pago sin tocar el TTL.
func (r *Redis) GuardarPago(ctx context.Context, llave, pagoID string) error {
	return r.rdb.SetArgs(context.WithoutCancel(ctx), idemPagoPrefix+llave, pagoID, goredis.SetArgs{Mode: "XX", KeepTTL: true}).Err()
}

// SoltarPago libera la llave si el cobro falló.
func (r *Redis) SoltarPago(ctx context.Context, llave string) {
	_ = r.rdb.Del(context.WithoutCancel(ctx), idemPagoPrefix+llave).Err()
}

// TomarEvento aparta el evento por 7 días.
func (r *Redis) TomarEvento(ctx context.Context, id string) (bool, error) {
	return r.rdb.SetNX(ctx, idemEventoPrefix+id, "1", idemEventoTTL).Result()
}

// SoltarEvento libera el evento si aplicarlo falló, para que el reintento de Stripe entre.
func (r *Redis) SoltarEvento(ctx context.Context, id string) {
	_ = r.rdb.Del(context.WithoutCancel(ctx), idemEventoPrefix+id).Err()
}

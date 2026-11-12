package ia

import (
	"context"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// Errores de los límites (nv-28). Con ErrLimiteGlobal y ErrLimiteDesconocido responde el
// motor de reglas; con ErrLimiteUsuario el endpoint responde 429 con Retry-After.
var (
	// ErrLimiteUsuario: el usuario llegó a sus generaciones de la hora (IA_LIMITE_HORA).
	ErrLimiteUsuario = errors.New("ia: límite de generaciones del usuario")
	// ErrLimiteGlobal: la app llegó a sus llamadas del minuto o del día (IA_LIMITE_MIN/DIA).
	ErrLimiteGlobal = errors.New("ia: límite global de llamadas")
	// ErrLimiteDesconocido: Redis no respondió; se cierra y no se llama al proveedor.
	ErrLimiteDesconocido = errors.New("ia: límites no disponibles")
)

// LimiteUsuarioError dice cuándo vuelve a poder generar el usuario.
type LimiteUsuarioError struct {
	Regreso time.Time
	Espera  time.Duration
}

func (e *LimiteUsuarioError) Error() string {
	return fmt.Sprintf("%s; regresa a las %s", ErrLimiteUsuario, e.Regreso.UTC().Format(time.RFC3339))
}

// Is hace que errors.Is(err, ErrLimiteUsuario) funcione.
func (e *LimiteUsuarioError) Is(target error) bool { return target == ErrLimiteUsuario }

// RetryAfter son los segundos enteros de espera, redondeados hacia arriba, para la cabecera.
func (e *LimiteUsuarioError) RetryAfter() int {
	s := int((e.Espera + time.Second - 1) / time.Second)
	if s < 1 {
		return 1
	}
	return s
}

// Llaves de los límites (constitución §6, rate:{politica}:{sujeto}), siempre con TTL.
const (
	prefijoLimiteUsuario = "rate:ia_usuario:"
	prefijoLimiteMin     = "rate:ia_min:"
	prefijoLimiteDia     = "rate:ia_dia:"
	// sinUsuario es la cubeta común de las generaciones sin usuario en el contexto: así un
	// llamador que olvida ia.ConUsuario no queda sin límite.
	sinUsuario = "sin_usuario"
	// zonaProveedor es la del día de cuota de Gemini.
	zonaProveedor = "America/Los_Angeles"
	vidaUsuario   = time.Hour
	vidaMinuto    = time.Minute
	vidaDia       = 25 * time.Hour
)

// Ventana es una llave de conteo con la vida que toma al crearse.
type Ventana struct {
	Llave string
	Vida  time.Duration
}

// Cuenta es el total de una ventana tras sumar uno y lo que le queda de vida.
type Cuenta struct {
	Total int64
	Resta time.Duration
}

// Limitador suma uno a cada ventana de una vez y devuelve sus cuentas en el mismo orden.
type Limitador interface {
	Tomar(ctx context.Context, ventanas ...Ventana) ([]Cuenta, error)
}

// Limites son los valores de IA_LIMITE_HORA, IA_LIMITE_MIN e IA_LIMITE_DIA.
type Limites struct {
	PorHora   int
	PorMinuto int
	PorDia    int
}

func (l Limites) validar() error {
	if l.PorHora < 1 || l.PorMinuto < 1 || l.PorDia < 1 {
		return errors.New("ia: los límites deben ser positivos")
	}
	return nil
}

// llaveMinuto y llaveDia nombran la ventana fija del minuto (UTC) y del día del proveedor.
func llaveMinuto(t time.Time) string { return prefijoLimiteMin + t.UTC().Format("200601021504") }

func llaveDia(t time.Time, zona *time.Location) string {
	return prefijoLimiteDia + t.In(zona).Format("2006-01-02")
}

func llaveUsuario(ctx context.Context) string {
	u := usuarioDe(ctx)
	if u.IsZero() {
		return prefijoLimiteUsuario + sinUsuario
	}
	return prefijoLimiteUsuario + u.Hex()
}

// LimitadorRedis cuenta en Redis con un solo Pipeline: por ventana, SET NX con vida, INCR y
// PTTL (3 comandos, sin MULTI/EXEC). Cumple Limitador.
type LimitadorRedis struct {
	rdb *goredis.Client
}

// NewLimitadorRedis recibe el cliente.
func NewLimitadorRedis(rdb *goredis.Client) *LimitadorRedis { return &LimitadorRedis{rdb: rdb} }

// Tomar suma uno a cada ventana; la vida solo se pone cuando la llave nace.
func (l *LimitadorRedis) Tomar(ctx context.Context, ventanas ...Ventana) ([]Cuenta, error) {
	return l.Sumar(ctx, 1, ventanas...)
}

// Sumar suma n a cada ventana con los mismos 3 comandos por ventana (INCRBY en lugar de INCR).
func (l *LimitadorRedis) Sumar(ctx context.Context, n int64, ventanas ...Ventana) ([]Cuenta, error) {
	pipe := l.rdb.Pipeline()
	incr := make([]*goredis.IntCmd, len(ventanas))
	vida := make([]*goredis.DurationCmd, len(ventanas))
	for i, v := range ventanas {
		pipe.SetNX(ctx, v.Llave, 0, v.Vida)
		incr[i] = pipe.IncrBy(ctx, v.Llave, n)
		vida[i] = pipe.PTTL(ctx, v.Llave)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, err
	}
	out := make([]Cuenta, len(ventanas))
	for i := range ventanas {
		out[i] = Cuenta{Total: incr[i].Val(), Resta: vida[i].Val()}
	}
	return out, nil
}

// tomarGlobal cuenta una llamada al proveedor en el minuto y en el día. Una llamada rechazada
// también cuenta: el límite queda del lado seguro.
func (g *Guardia) tomarGlobal(ctx context.Context, ahora time.Time) error {
	cuentas, err := g.limitador.Tomar(ctx,
		Ventana{Llave: llaveMinuto(ahora), Vida: vidaMinuto},
		Ventana{Llave: llaveDia(ahora, g.zona), Vida: vidaDia})
	if err != nil || len(cuentas) != 2 {
		return ErrLimiteDesconocido
	}
	if cuentas[0].Total > int64(g.limites.PorMinuto) || cuentas[1].Total > int64(g.limites.PorDia) {
		return ErrLimiteGlobal
	}
	return nil
}

// tomarUsuario cuenta una generación del usuario en su hora, que empieza con la primera.
func (g *Guardia) tomarUsuario(ctx context.Context, ahora time.Time) error {
	cuentas, err := g.limitador.Tomar(ctx, Ventana{Llave: llaveUsuario(ctx), Vida: vidaUsuario})
	if err != nil || len(cuentas) != 1 {
		return ErrLimiteDesconocido
	}
	if cuentas[0].Total <= int64(g.limites.PorHora) {
		return nil
	}
	resta := cuentas[0].Resta
	if resta <= 0 {
		resta = vidaUsuario
	}
	return &LimiteUsuarioError{Regreso: ahora.Add(resta).UTC(), Espera: resta}
}

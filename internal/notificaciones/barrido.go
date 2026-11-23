package notificaciones

import (
	"context"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Unico avisa una sola vez por persona, tipo y objeto; lo cumple *Agrupador (nv-86). Los
// módulos de reseñas, cupones y planes lo reciben por constructor para no importar el centro.
type Unico interface {
	Once(ctx context.Context, to bson.ObjectID, kind, objeto string, data map[string]string) bool
}

// Tarea es un barrido de avisos programados (nv-86). Devuelve cuántos avisos nuevos salieron.
type Tarea interface {
	Name() string
	Avisar(ctx context.Context, now time.Time) (int, error)
}

// barridoCada es cada cuánto revisa cada instancia. Como el barrido de reservas (nv-73), sin
// candado de Redis: cada aviso se reserva una vez en MongoDB (Once), así que correr en las
// dos instancias no repite avisos y no gasta Upstash.
const barridoCada = 5 * time.Minute

// Barrido corre las tareas de avisos cada 5 minutos hasta que ctx termina.
type Barrido struct {
	tareas []Tarea
	log    *slog.Logger
	now    func() time.Time
}

// NewBarrido recibe las tareas por constructor.
func NewBarrido(log *slog.Logger, tareas ...Tarea) *Barrido {
	return &Barrido{tareas: tareas, log: log, now: time.Now}
}

// RunOnce corre cada tarea una vez; el fallo de una no frena a las demás.
func (b *Barrido) RunOnce(ctx context.Context) {
	now := b.now().UTC()
	for _, t := range b.tareas {
		n, err := t.Avisar(ctx, now)
		if err != nil && ctx.Err() == nil {
			b.log.WarnContext(ctx, "avisos: la tarea se reintentará", slog.String("evento", "avisos_reintento"), slog.String("tarea", t.Name()))
			continue
		}
		if n > 0 {
			b.log.InfoContext(ctx, "avisos programados enviados", slog.String("evento", "avisos_programados"), slog.String("tarea", t.Name()), slog.Int("avisos", n))
		}
	}
}

// Run revisa cada 5 minutos hasta que ctx termina.
func (b *Barrido) Run(ctx context.Context) {
	ticker := time.NewTicker(barridoCada)
	defer ticker.Stop()
	for {
		b.RunOnce(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

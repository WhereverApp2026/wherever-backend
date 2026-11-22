package notificaciones

import (
	"context"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Agrupador avisa una sola vez por persona, tipo y objeto cada GroupEvery (nv-83): el primer
// me gusta o comentario avisa y los siguientes dentro del lapso no. La reserva vive en MongoDB
// (`notificaciones_agrupadas`), así que vale entre instancias y sin Redis.
type Agrupador struct {
	repo  *Repository
	n     Notifier
	every time.Duration
	log   *slog.Logger
	now   func() time.Time
}

// NewAgrupador recibe el repositorio y el Notifier por constructor; con n nulo no avisa.
func NewAgrupador(repo *Repository, n Notifier, log *slog.Logger) *Agrupador {
	if n == nil {
		n = Noop{}
	}
	if log == nil {
		log = slog.Default()
	}
	return &Agrupador{repo: repo, n: n, every: GroupEvery, log: log, now: time.Now}
}

// WithReloj fija el reloj (pruebas del lapso).
func (a *Agrupador) WithReloj(now func() time.Time) *Agrupador {
	a.now = now
	return a
}

// Emit avisa `kind` a `to` sobre `objeto` (p. ej. `momento:<id>`) si no se avisó en el lapso.
// Como Notifier.Emit, nunca falla: un error queda en el log sin datos.
func (a *Agrupador) Emit(ctx context.Context, to bson.ObjectID, kind, objeto string, data map[string]string) {
	if a == nil {
		return
	}
	ok, err := a.repo.ClaimGroup(ctx, to, kind+":"+objeto, a.now().UTC(), a.every)
	if err != nil {
		a.log.WarnContext(ctx, "notificaciones: no se reservó el aviso agrupado", slog.String("evento", "aviso_fallido"), slog.String("tipo", kind))
		return
	}
	if ok {
		a.n.Emit(ctx, to, kind, data)
	}
}

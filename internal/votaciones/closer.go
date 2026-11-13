package votaciones

import (
	"context"
	"log/slog"
	"time"
)

// closeEvery es cada cuánto revisa cada instancia las votaciones vencidas. Solo consulta
// MongoDB (ix_estado_cierra): no gasta comandos de Redis. Con dos instancias, cerrar es una
// operación condicionada y cada votación se cierra una sola vez.
const closeEvery = time.Minute

// Closer cierra las votaciones vencidas y termina de aplicar las pendientes.
type Closer struct {
	svc *Service
	log *slog.Logger
}

// NewCloser recibe el service por constructor.
func NewCloser(svc *Service, log *slog.Logger) *Closer {
	return &Closer{svc: svc, log: log}
}

// Run revisa cada minuto hasta que ctx termina.
func (c *Closer) Run(ctx context.Context) {
	ticker := time.NewTicker(closeEvery)
	defer ticker.Stop()
	for {
		if _, err := c.svc.FinishDue(ctx); err != nil && ctx.Err() == nil {
			c.log.Warn("votaciones: se reintentará", "evento", "votacion_reintento", "error", err.Error())
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

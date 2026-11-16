package estadisticas

import (
	"context"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// trabajoCada es cada cuánto recalcula cada instancia. Solo MongoDB y sin candado de Redis:
// las escrituras son `$set` idempotentes y dos instancias escriben lo mismo (como el barrido de
// nv-59).
const trabajoCada = time.Hour

// Trabajo recalcula hoy y ayer y rellena 50 negocios cada hora hasta que ctx termina.
type Trabajo struct {
	svc *Service
	log *slog.Logger
}

// NewTrabajo recibe el service por constructor.
func NewTrabajo(svc *Service, log *slog.Logger) *Trabajo { return &Trabajo{svc: svc, log: log} }

// Run corre al arrancar y luego cada hora. Los logs no llevan ids de clientes.
func (t *Trabajo) Run(ctx context.Context) {
	ticker := time.NewTicker(trabajoCada)
	defer ticker.Stop()
	for {
		if _, err := t.svc.Recalcular(ctx); err != nil && ctx.Err() == nil {
			t.log.Warn("estadisticas: el recálculo se reintentará", "evento", "estadisticas_reintento", "error", err.Error())
		}
		if _, err := t.svc.Rellenar(ctx); err != nil && ctx.Err() == nil {
			t.log.Warn("estadisticas: el relleno se reintentará", "evento", "estadisticas_relleno_reintento", "error", err.Error())
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Tarea es la parte nocturna: promedios por categoría y borrado de días de más de 400 días.
// Cumple privacidad.Tarea (corre con su candado).
type Tarea struct{ svc *Service }

// NewTarea recibe el service por constructor.
func NewTarea(svc *Service) *Tarea { return &Tarea{svc: svc} }

// Name identifica la tarea en el log.
func (t *Tarea) Name() string { return "estadisticas_negocio" }

// Run devuelve cuántos días viejos borró.
func (t *Tarea) Run(ctx context.Context) (int, error) { return t.svc.Nocturno(ctx) }

// Observador recibe la pausa, la reanudación y la baja del negocio (nv-56): solo la baja
// borra sus estadísticas.
type Observador struct{ svc *Service }

// NewObservador recibe el service por constructor.
func NewObservador(svc *Service) Observador { return Observador{svc: svc} }

// Pausado no hace nada: las cifras siguen siendo del dueño.
func (o Observador) Pausado(context.Context, bson.ObjectID) error { return nil }

// Reanudado no hace nada.
func (o Observador) Reanudado(context.Context, bson.ObjectID) error { return nil }

// DadoDeBaja borra los días del negocio.
func (o Observador) DadoDeBaja(ctx context.Context, negocio bson.ObjectID) error {
	return o.svc.BorrarNegocio(ctx, negocio)
}

package reservas

import (
	"context"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
)

// barridoCada es cada cuánto revisa cada instancia. Solo MongoDB y sin candado de Redis: las
// actualizaciones son condicionales e idempotentes, correr en dos instancias no hace daño y no
// gasta Upstash. El cupo no depende del barrido: el conteo ya ignora apartados vencidos.
const barridoCada = 5 * time.Minute

// Barrer pasa los apartados vencidos a `expirada` y las confirmadas de días anteriores a
// `no_asistio`. Devuelve cuántas cambió de cada una.
func (s *Service) Barrer(ctx context.Context) (expiradas, ausentes int64, err error) {
	now := s.now().UTC()
	expiradas, err = s.repo.CambiarVarias(ctx, bson.D{
		{Key: "estado", Value: EstadoBloqueada},
		{Key: "bloqueo_expira_en", Value: bson.D{{Key: "$lte", Value: now}}},
	}, EstadoExpirada, PorSistema, now)
	if err != nil {
		return 0, 0, err
	}
	hoy, _ := s.diaLocal(now)
	ausentes, err = s.repo.CambiarVarias(ctx, bson.D{
		{Key: "estado", Value: EstadoConfirmada},
		{Key: "fecha_hora", Value: bson.D{{Key: "$lt", Value: hoy}}},
	}, EstadoNoAsistio, PorSistema, now)
	return expiradas, ausentes, err
}

// Barrido corre Barrer cada 5 minutos hasta que ctx termina.
type Barrido struct {
	svc *Service
	log *slog.Logger
}

// NewBarrido recibe el service por constructor.
func NewBarrido(svc *Service, log *slog.Logger) *Barrido { return &Barrido{svc: svc, log: log} }

// Run revisa cada 5 minutos hasta que ctx termina.
func (b *Barrido) Run(ctx context.Context) {
	ticker := time.NewTicker(barridoCada)
	defer ticker.Stop()
	for {
		if _, _, err := b.svc.Barrer(ctx); err != nil && ctx.Err() == nil {
			b.log.Warn("reservas: el barrido se reintentará", "evento", "reserva_barrido_reintento", "error", err.Error())
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// --- Panel del negocio (nv-56) ---

// ResumenReservas es la tarjeta del panel: reservas activas del día y la siguiente.
func (s *Service) ResumenReservas(ctx context.Context, negocio bson.ObjectID, dia negocios.Periodo) (negocios.ResumenReservas, error) {
	rs, err := s.repo.DelPeriodo(ctx, negocio, dia.Desde, dia.Hasta, []string{EstadoBloqueada, EstadoPendiente, EstadoConfirmada, EstadoAsistio})
	if err != nil {
		return negocios.ResumenReservas{}, err
	}
	now := s.now()
	out := negocios.ResumenReservas{}
	for _, r := range rs {
		if r.Estado != EstadoAsistio && !r.Ocupa(now) {
			continue
		}
		out.Hoy++
		if out.Proxima == nil && r.Estado != EstadoAsistio && !r.FechaHora.Before(now) {
			out.Proxima = &negocios.ProximaReserva{Hora: r.FechaHora.UTC(), Personas: r.Personas}
		}
	}
	return out, nil
}

// HayReservasFuturas frena la baja del negocio si quedan reservas activas (nv-56).
func (s *Service) HayReservasFuturas(ctx context.Context, negocio bson.ObjectID, desde time.Time) (bool, error) {
	return s.repo.HayFuturas(ctx, negocio, desde)
}

// Observador recibe la pausa y la baja del negocio (nv-56). Pausado no hace nada: un negocio
// pausado no es visible y la disponibilidad responde 404; las reservas ya hechas siguen.
type Observador struct{ svc *Service }

// NewObservador recibe el service por constructor.
func NewObservador(svc *Service) Observador { return Observador{svc: svc} }

// Pausado no cambia las reservas.
func (o Observador) Pausado(context.Context, bson.ObjectID) error { return nil }

// Reanudado no cambia las reservas.
func (o Observador) Reanudado(context.Context, bson.ObjectID) error { return nil }

// DadoDeBaja cancela por el negocio las reservas activas que queden (la baja ya las frena).
func (o Observador) DadoDeBaja(ctx context.Context, negocio bson.ObjectID) error {
	now := o.svc.now().UTC()
	_, err := o.svc.repo.CambiarVarias(ctx, bson.D{
		{Key: "negocio_id", Value: negocio}, {Key: "fecha_hora", Value: bson.D{{Key: "$gte", Value: now}}},
		{Key: "estado", Value: bson.D{{Key: "$in", Value: activos}}},
	}, EstadoCancelada, PorNegocio, now)
	return err
}

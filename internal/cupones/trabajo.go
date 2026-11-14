package cupones

import (
	"context"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
)

// trabajoCada es cada cuánto revisa cada instancia. Solo MongoDB y sin candado de Redis: las
// actualizaciones son condicionales e idempotentes y `cupones_activos` se escribe con un
// conteo (`$set`), así que dos instancias a la vez no rompen nada ni gastan Upstash. Las
// lecturas filtran por fechas: lo que ve la persona no depende de este trabajo.
const trabajoCada = 5 * time.Minute

// Mantener activa los programados, vence los terminados (y las carteras), olvida los códigos
// usados de más de 5 minutos y recalcula `cupones_activos` de los negocios tocados.
func (s *Service) Mantener(ctx context.Context) error {
	now := s.now().UTC()
	inicio, err := s.repo.NegociosConInicio(ctx, now)
	if err != nil {
		return err
	}
	if err := s.repo.Activar(ctx, now); err != nil {
		return err
	}
	vencidos, err := s.repo.Vencer(ctx, now)
	if err != nil {
		return err
	}
	if err := s.repo.VencerCarteras(ctx, now); err != nil {
		return err
	}
	tocados := map[bson.ObjectID]bool{}
	for _, id := range append(inicio, vencidos...) {
		if !tocados[id] {
			tocados[id] = true
			s.recalcular(ctx, id)
		}
	}
	return nil
}

// Trabajo corre Mantener cada 5 minutos hasta que ctx termina.
type Trabajo struct {
	svc *Service
	log *slog.Logger
}

// NewTrabajo recibe el service por constructor.
func NewTrabajo(svc *Service, log *slog.Logger) *Trabajo { return &Trabajo{svc: svc, log: log} }

// Run revisa cada 5 minutos hasta que ctx termina.
func (t *Trabajo) Run(ctx context.Context) {
	ticker := time.NewTicker(trabajoCada)
	defer ticker.Stop()
	for {
		if err := t.svc.Mantener(ctx); err != nil && ctx.Err() == nil {
			t.log.Warn("cupones: el trabajo se reintentará", "evento", "cupones_trabajo_reintento", "error", err.Error())
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// --- Panel del negocio (nv-56) ---

// ResumenCupones es la tarjeta de P-49: canjes del día y cupones vigentes.
func (s *Service) ResumenCupones(ctx context.Context, negocio bson.ObjectID, dia negocios.Periodo) (negocios.ResumenCupones, error) {
	canjes, err := s.repo.CanjeadosEntre(ctx, negocio, dia.Desde, dia.Hasta)
	if err != nil {
		return negocios.ResumenCupones{}, err
	}
	activos, err := s.repo.ContarActivos(ctx, negocio, s.now().UTC())
	if err != nil {
		return negocios.ResumenCupones{}, err
	}
	return negocios.ResumenCupones{CanjeadosHoy: int(canjes), Activos: int(activos)}, nil
}

// Observador recibe la pausa, la reanudación y la baja del negocio (nv-56).
type Observador struct{ svc *Service }

// NewObservador recibe el service por constructor.
func NewObservador(svc *Service) Observador { return Observador{svc: svc} }

// Pausado pausa los cupones activos o programados; vuelven al reanudar.
func (o Observador) Pausado(ctx context.Context, negocio bson.ObjectID) error {
	if err := o.svc.repo.PausarDelNegocio(ctx, negocio, true, o.svc.now().UTC()); err != nil {
		return err
	}
	o.svc.recalcular(ctx, negocio)
	return nil
}

// Reanudado devuelve los cupones que pausó el negocio.
func (o Observador) Reanudado(ctx context.Context, negocio bson.ObjectID) error {
	if err := o.svc.repo.ReanudarDelNegocio(ctx, negocio, o.svc.now().UTC()); err != nil {
		return err
	}
	o.svc.recalcular(ctx, negocio)
	return nil
}

// DadoDeBaja pausa los cupones sin marca: no vuelven.
func (o Observador) DadoDeBaja(ctx context.Context, negocio bson.ObjectID) error {
	if err := o.svc.repo.PausarDelNegocio(ctx, negocio, false, o.svc.now().UTC()); err != nil {
		return err
	}
	o.svc.recalcular(ctx, negocio)
	return nil
}

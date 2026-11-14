package panel

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
)

// loteTarea acota cuántos negocios toca cada corrida.
const loteTarea = 500

// Tarea es el trabajo nocturno del panel: reanuda las pausas cuyo `hasta` venció y reintenta
// los avisos a observadores que fallaron. Cumple privacidad.Tarea (corre con su candado).
type Tarea struct {
	svc *Service
}

// NewTarea recibe el service del panel.
func NewTarea(svc *Service) *Tarea { return &Tarea{svc: svc} }

// Name identifica la tarea en el log.
func (t *Tarea) Name() string { return "negocios_pausas" }

// Run devuelve cuántos negocios reanudó o avisó.
func (t *Tarea) Run(ctx context.Context) (int, error) {
	s := t.svc
	now := s.now().UTC()
	vencidas, err := s.repo.PausasVencidas(ctx, now, loteTarea)
	if err != nil {
		return 0, err
	}
	n := 0
	var errs []error
	for _, b := range vencidas {
		nb, err := s.reanudar(ctx, bson.D{{Key: "_id", Value: b.ID}, {Key: "estado", Value: negocios.StatusPaused},
			{Key: "pausado.hasta", Value: bson.D{{Key: "$lte", Value: now}}}})
		if errors.Is(err, negocios.ErrNoAplica) {
			continue
		}
		if err != nil {
			errs = append(errs, err)
			continue
		}
		s.despues(ctx, nb.ID, negocios.AvisoReanudado)
		n++
	}
	m, err := t.reintentar(ctx)
	return n + m, errors.Join(append(errs, err)...)
}

// reintentar vuelve a mandar los avisos pendientes; el que llega se quita.
func (t *Tarea) reintentar(ctx context.Context) (int, error) {
	s := t.svc
	if len(s.observadores) == 0 {
		return 0, nil
	}
	rows, err := s.repo.ConAvisos(ctx, loteTarea)
	if err != nil {
		return 0, err
	}
	n := 0
	var errs []error
	for _, b := range rows {
		for _, evento := range b.AvisosPendientes {
			if err := s.observadores.Avisar(ctx, evento, b.ID); err != nil {
				continue
			}
			errs = append(errs, s.repo.QuitarAviso(ctx, b.ID, evento))
			n++
		}
	}
	return n, errors.Join(errs...)
}

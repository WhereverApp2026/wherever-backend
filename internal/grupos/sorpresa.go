package grupos

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/planes"
)

// SurpriseFor cumple planes.Surprises: si el grupo del plan le oculta las paradas a userID,
// devuelve lo que sí puede ver (revelar_en y el punto de encuentro). Se decide al leer, igual
// en las dos instancias.
func (r *Repository) SurpriseFor(ctx context.Context, planID, userID bson.ObjectID, now time.Time) (*planes.Sorpresa, error) {
	g, err := r.FindByPlan(ctx, planID)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !g.Surprise.HidesFrom(userID, now) {
		return nil, nil
	}
	out := &planes.Sorpresa{RevelarEn: g.Surprise.RevealAt}
	if mp := g.MeetingPoint; mp != nil {
		lat, lng, at := mp.Location.Lat, mp.Location.Lng, mp.At
		out.Punto = &planes.PuntoSorpresa{Nombre: mp.Name, Lat: &lat, Lng: &lng, Hora: &at}
		if mp.BusinessID != nil {
			out.Punto.NegocioID = mp.BusinessID.Hex()
		}
	}
	return out, nil
}

// LocationsTask es la tarea nocturna que borra las ubicaciones compartidas vencidas (nv-69);
// cumple privacidad.Tarea.
type LocationsTask struct {
	repo *Repository
	now  func() time.Time
}

// NewLocationsTask recibe el repositorio por constructor.
func NewLocationsTask(repo *Repository) *LocationsTask {
	return &LocationsTask{repo: repo, now: time.Now}
}

// Name identifica la tarea en los logs.
func (t *LocationsTask) Name() string { return "grupos_ubicaciones" }

// Run borra las ubicaciones vencidas; devuelve cuántos grupos tocó.
func (t *LocationsTask) Run(ctx context.Context) (int, error) {
	n, err := t.repo.PurgeExpiredLocations(ctx, t.now().UTC())
	return int(n), err
}

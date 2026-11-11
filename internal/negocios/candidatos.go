package negocios

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// CandidateQuery es la búsqueda de candidatos del recomendador (NV-23): negocios visibles
// dentro del radio que cumplen `Conditions`, del más cercano al más lejano.
type CandidateQuery struct {
	Lat, Lng   float64
	RadiusM    float64
	Conditions bson.A // filtros duros; van en `$geoNear.query` junto con VisibleFilter
	Limit      int
}

// Candidate es un negocio con su distancia en metros al punto de la consulta.
type Candidate struct {
	Business
	DistanceM float64
}

// Candidates reutiliza la consulta de GET /lugares ordenada por distancia: los filtros
// duros se aplican en MongoDB, antes del límite, y el orden es estable (distancia y `_id`).
func (r *Repository) Candidates(ctx context.Context, q CandidateQuery) ([]Candidate, error) {
	rows, err := r.List(ctx, listParams{
		Match: q.Conditions, Near: &geoPoint{Lat: q.Lat, Lng: q.Lng},
		RadiusM: q.RadiusM, Order: OrderDistance, Limit: q.Limit,
	})
	if err != nil {
		return nil, err
	}
	if len(rows) > q.Limit {
		rows = rows[:q.Limit]
	}
	out := make([]Candidate, 0, len(rows))
	for _, row := range rows {
		c := Candidate{Business: row.Business}
		if row.DistanceM != nil {
			c.DistanceM = *row.DistanceM
		}
		out = append(out, c)
	}
	return out, nil
}

// OpenAtCondition es la regla de OpenAt lista para un `$and` de MongoDB.
func OpenAtCondition(now time.Time) bson.D { return bson.D{openAtFilter(now)} }

// ItemOf arma la tarjeta de GET /lugares de un candidato; la reutiliza GET /recomendaciones
// (nv-24) para que las dos listas tengan la misma forma.
func ItemOf(c Candidate, now time.Time) PlaceItem {
	distance := c.DistanceM
	return toItem(placeRow{Business: c.Business, DistanceM: &distance}, now)
}

// MexicoCity es la zona de los horarios (la misma de OpenAt).
func MexicoCity() *time.Location { return mexicoCity }

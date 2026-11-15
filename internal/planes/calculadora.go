package planes

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
)

// Negocios es lo que la calculadora lee del catálogo.
type Negocios interface {
	FindVisibleMany(ctx context.Context, ids []bson.ObjectID) ([]negocios.Business, error)
}

// Calculadora carga los negocios de las paradas y aplica Recalcular.
type Calculadora struct {
	negocios    Negocios
	velocidades Velocidades
}

// NewCalculadora recibe el catálogo y las velocidades por constructor.
func NewCalculadora(n Negocios, v Velocidades) *Calculadora {
	return &Calculadora{negocios: n, velocidades: v}
}

// Aplicar recalcula el plan con los negocios aprobados de sus paradas.
func (c *Calculadora) Aplicar(ctx context.Context, p *Plan) error {
	ids := make([]bson.ObjectID, 0, len(p.Paradas))
	for _, s := range p.Paradas {
		ids = append(ids, s.NegocioID)
	}
	found, err := c.negocios.FindVisibleMany(ctx, ids)
	if err != nil {
		return err
	}
	lugares := make(map[bson.ObjectID]Lugar, len(found))
	for _, b := range found {
		if len(b.Location.Coordinates) != 2 {
			continue
		}
		lugares[b.ID] = Lugar{Lng: b.Location.Coordinates[0], Lat: b.Location.Coordinates[1], Horario: b.ScheduleMin}
	}
	if err := Recalcular(p, lugares, c.velocidades); err != nil {
		return err
	}
	// La zona es la primera parada: la usa GET /planes/populares (nv-68).
	if l, ok := lugares[p.Paradas[0].NegocioID]; ok {
		p.Zona = &GeoPoint{Type: "Point", Coordinates: []float64{l.Lng, l.Lat}}
	}
	return nil
}

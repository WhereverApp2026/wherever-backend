package clima

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/ciudades"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

// Ciudades busca el centro de una ciudad; la implementa *ciudades.Service.
type Ciudades interface {
	Find(ctx context.Context, id bson.ObjectID) (ciudades.City, error)
}

// coordenadas toma lat y lng o el centro de la ciudad.
func coordenadas(ctx context.Context, cities Ciudades, q Query) (float64, float64, error) {
	if q.Lat != nil && q.Lng != nil {
		return *q.Lat, *q.Lng, nil
	}
	if q.CiudadID == "" {
		return 0, 0, apperr.Validation("indica lat y lng o ciudad_id")
	}
	id, err := bson.ObjectIDFromHex(q.CiudadID)
	if err != nil {
		return 0, 0, apperr.Validation("ciudad inválida")
	}
	city, err := cities.Find(ctx, id)
	if errors.Is(err, ciudades.ErrNotFound) {
		return 0, 0, apperr.Validation("ciudad inválida")
	}
	if err != nil {
		return 0, 0, apperr.Unavailable("servicio no disponible")
	}
	if len(city.Center.Coordinates) != 2 {
		return 0, 0, apperr.Validation("ciudad sin ubicación")
	}
	return city.Center.Coordinates[1], city.Center.Coordinates[0], nil
}

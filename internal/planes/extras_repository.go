package planes

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Radio y ventana de los planes populares (nv-68).
const (
	popularRadiusM = 15000
	popularDays    = 7
	popularTop     = 3
)

// IncClones suma 1 a `veces_clonado` del plan original en una sola operación.
func (r *Repository) IncClones(ctx context.Context, id bson.ObjectID) error {
	_, err := r.col.UpdateOne(ctx, bson.D{{Key: "_id", Value: id}}, bson.D{{Key: "$inc", Value: bson.D{{Key: "veces_clonado", Value: 1}}}})
	return err
}

// Popular trae los planes públicos activos de la última semana cerca del punto (radio de
// 15 km sobre `zona`), los más copiados primero. Usa ix_populares.
func (r *Repository) Popular(ctx context.Context, lat, lng float64, now time.Time) ([]Plan, error) {
	pipeline := bson.A{
		bson.D{{Key: "$geoNear", Value: bson.D{
			{Key: "near", Value: bson.D{{Key: "type", Value: "Point"}, {Key: "coordinates", Value: bson.A{lng, lat}}}},
			{Key: "distanceField", Value: "distancia_m"},
			{Key: "maxDistance", Value: popularRadiusM},
			{Key: "key", Value: "zona"},
			{Key: "query", Value: bson.D{
				{Key: "visibilidad", Value: VisibilityPublic}, {Key: "estado", Value: StatusActive},
				{Key: "actualizado_en", Value: bson.D{{Key: "$gte", Value: now.Add(-popularDays * 24 * time.Hour)}}},
			}},
		}}},
		bson.D{{Key: "$sort", Value: bson.D{{Key: "veces_clonado", Value: -1}, {Key: "_id", Value: 1}}}},
		bson.D{{Key: "$limit", Value: popularTop}},
	}
	cur, err := r.col.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	out := []Plan{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

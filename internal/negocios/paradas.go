package negocios

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// FindVisibleMany trae de los ids solo los negocios aprobados y no eliminados, con la
// ubicación y el horario (lo que nv-29 usa para validar paradas). Un id que no vuelve no
// está disponible.
func (r *Repository) FindVisibleMany(ctx context.Context, ids []bson.ObjectID) ([]Business, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	filter := append(VisibleFilter(), bson.E{Key: "_id", Value: bson.D{{Key: "$in", Value: ids}}})
	proj := options.Find().SetProjection(bson.D{{Key: "ubicacion", Value: 1}, {Key: "horario_min", Value: 1}})
	cur, err := r.col.Find(ctx, filter, proj)
	if err != nil {
		return nil, err
	}
	var out []Business
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

package negocios

import (
	"context"
	"time"

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

// FindCards devuelve la tarjeta pública (la de GET /lugares) de los negocios visibles de
// ids, en cualquier orden; un id que no vuelve no está disponible. La usa la vista pública
// de un plan compartido (nv-38).
func (r *Repository) FindCards(ctx context.Context, ids []bson.ObjectID, now time.Time) ([]PlaceItem, error) {
	out := []PlaceItem{}
	if len(ids) == 0 {
		return out, nil
	}
	filter := append(VisibleFilter(), bson.E{Key: "_id", Value: bson.D{{Key: "$in", Value: ids}}})
	cur, err := r.col.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	var rows []Business
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	for _, b := range rows {
		out = append(out, toItem(placeRow{Business: b}, now))
	}
	return out, nil
}

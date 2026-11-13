package negocios

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// FindFavoriteCards trae de los ids lo que favoritos (nv-54) muestra, sin filtrar por estado: quien
// llama decide qué ocultar (no aprobado) y qué limpiar (`eliminado_en` o id ausente).
func (r *Repository) FindFavoriteCards(ctx context.Context, ids []bson.ObjectID) ([]Business, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	proj := options.Find().SetProjection(bson.D{
		{Key: "nombre", Value: 1}, {Key: "fotos", Value: 1}, {Key: "categoria", Value: 1},
		{Key: "estado", Value: 1}, {Key: "eliminado_en", Value: 1}, {Key: "cupones_activos", Value: 1},
		{Key: "horario_min", Value: 1},
	})
	cur, err := r.col.Find(ctx, bson.D{{Key: "_id", Value: bson.D{{Key: "$in", Value: ids}}}}, proj)
	if err != nil {
		return nil, err
	}
	var out []Business
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

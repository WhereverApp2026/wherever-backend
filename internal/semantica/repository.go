package semantica

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/WhereverApp2026/wherever-backend/internal/embeddings"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
)

// MaxNegocios es el tope de negocios que se comparan en la API (plan B). El piloto tiene 30;
// con muchos más conviene un índice vectorial.
const MaxNegocios = 1000

// Repository lee los negocios aprobados y sus vectores. Sin colecciones propias.
type Repository struct {
	negocios *mongo.Collection
	vectores *mongo.Collection
}

// NewRepository usa `negocios` y `vectores_negocios` (nv-50).
func NewRepository(db *mongo.Database) *Repository {
	return &Repository{negocios: db.Collection(negocios.Collection), vectores: db.Collection(embeddings.Collection)}
}

// Visibles trae los negocios aprobados y no eliminados, en orden de id.
func (r *Repository) Visibles(ctx context.Context) ([]negocios.Business, error) {
	opts := options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}).SetLimit(MaxNegocios)
	cur, err := r.negocios.Find(ctx, negocios.VisibleFilter(), opts)
	if err != nil {
		return nil, err
	}
	var out []negocios.Business
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Vectores trae los vectores guardados de los ids pedidos; un negocio sin vector no vuelve.
func (r *Repository) Vectores(ctx context.Context, ids []bson.ObjectID) (map[bson.ObjectID][]float32, error) {
	out := map[bson.ObjectID][]float32{}
	if len(ids) == 0 {
		return out, nil
	}
	filter := bson.D{{Key: "_id", Value: bson.D{{Key: "$in", Value: ids}}}}
	cur, err := r.vectores.Find(ctx, filter, options.Find().SetProjection(bson.D{{Key: "vector", Value: 1}}))
	if err != nil {
		return nil, err
	}
	var docs []embeddings.Vector
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	for _, d := range docs {
		out[d.NegocioID] = d.Vector
	}
	return out, nil
}

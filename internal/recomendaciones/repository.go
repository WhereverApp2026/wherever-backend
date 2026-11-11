// Package recomendaciones expone «Para ti ahora» y «No me interesa» (spec nv-24) sobre el
// motor de nv-23 (internal/recomendador).
package recomendaciones

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	platformmongo "github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
)

// Collection guarda los negocios marcados con «No me interesa».
const Collection = "descartes_usuario"

// MaxDescartes es cuántos descartes, los más recientes, se excluyen en cada consulta.
const MaxDescartes = 1000

// Descarte es un documento de `descartes_usuario`.
type Descarte struct {
	ID         bson.ObjectID `bson:"_id"`
	UserID     bson.ObjectID `bson:"usuario_id"`
	BusinessID bson.ObjectID `bson:"negocio_id"`
	CreatedAt  time.Time     `bson:"creado_en"`
}

// Indexes es el índice único (usuario, negocio). El único creador es scripts/indexes.
func Indexes() []platformmongo.IndexSet {
	return []platformmongo.IndexSet{{
		Collection: Collection,
		Models: []mongo.IndexModel{{
			Keys:    bson.D{{Key: "usuario_id", Value: 1}, {Key: "negocio_id", Value: 1}},
			Options: options.Index().SetName("ux_usuario_negocio").SetUnique(true),
		}},
	}}
}

// Repository accede a `descartes_usuario`. Toda operación lleva el usuario en el filtro.
type Repository struct {
	col *mongo.Collection
}

// NewRepository recibe la base por constructor.
func NewRepository(db *mongo.Database) *Repository {
	return &Repository{col: db.Collection(Collection)}
}

func pair(userID, businessID bson.ObjectID) bson.D {
	return bson.D{{Key: "usuario_id", Value: userID}, {Key: "negocio_id", Value: businessID}}
}

// Mark guarda el descarte; repetirlo no cambia nada (upsert por el índice único).
func (r *Repository) Mark(ctx context.Context, userID, businessID bson.ObjectID, now time.Time) error {
	update := bson.D{{Key: "$setOnInsert", Value: bson.D{
		{Key: "_id", Value: bson.NewObjectID()}, {Key: "creado_en", Value: now},
	}}}
	_, err := r.col.UpdateOne(ctx, pair(userID, businessID), update, options.UpdateOne().SetUpsert(true))
	if mongo.IsDuplicateKeyError(err) {
		// Dos marcas al mismo tiempo: la otra ya lo guardó.
		return nil
	}
	return err
}

// Unmark deshace el descarte; false si el usuario no lo tenía.
func (r *Repository) Unmark(ctx context.Context, userID, businessID bson.ObjectID) (bool, error) {
	res, err := r.col.DeleteOne(ctx, pair(userID, businessID))
	if err != nil {
		return false, err
	}
	return res.DeletedCount > 0, nil
}

// ListByUser devuelve los descartes del usuario, del más reciente al más antiguo.
func (r *Repository) ListByUser(ctx context.Context, userID bson.ObjectID, limit int64) ([]Descarte, error) {
	opts := options.Find().SetSort(bson.D{{Key: "creado_en", Value: -1}, {Key: "_id", Value: -1}})
	if limit > 0 {
		opts.SetLimit(limit)
	}
	cur, err := r.col.Find(ctx, bson.D{{Key: "usuario_id", Value: userID}}, opts)
	if err != nil {
		return nil, err
	}
	var out []Descarte
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Excluded son los ids de negocio que el recomendador no debe mostrar.
func (r *Repository) Excluded(ctx context.Context, userID bson.ObjectID) ([]bson.ObjectID, error) {
	rows, err := r.ListByUser(ctx, userID, MaxDescartes)
	if err != nil {
		return nil, err
	}
	out := make([]bson.ObjectID, 0, len(rows))
	for _, d := range rows {
		out = append(out, d.BusinessID)
	}
	return out, nil
}

// DeleteByUser borra todos los descartes del usuario (ARCO).
func (r *Repository) DeleteByUser(ctx context.Context, userID bson.ObjectID) error {
	_, err := r.col.DeleteMany(ctx, bson.D{{Key: "usuario_id", Value: userID}})
	return err
}

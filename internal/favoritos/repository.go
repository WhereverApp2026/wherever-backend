package favoritos

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	platformmongo "github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
)

// Indexes son los índices de `favoritos`. El único creador es scripts/indexes.
func Indexes() []platformmongo.IndexSet {
	return []platformmongo.IndexSet{{
		Collection: Collection,
		Models: []mongo.IndexModel{
			{
				Keys:    bson.D{{Key: "usuario_id", Value: 1}, {Key: "tipo", Value: 1}, {Key: "objetivo_id", Value: 1}},
				Options: options.Index().SetName("ux_favorito").SetUnique(true),
			},
			{
				Keys:    bson.D{{Key: "usuario_id", Value: 1}, {Key: "tipo", Value: 1}, {Key: "creado_en", Value: -1}, {Key: "_id", Value: -1}},
				Options: options.Index().SetName("ix_lista"),
			},
			{Keys: bson.D{{Key: "objetivo_id", Value: 1}}, Options: options.Index().SetName("ix_objetivo")},
		},
	}}
}

// Repository lee y escribe `favoritos`.
type Repository struct {
	col *mongo.Collection
}

// NewRepository recibe la base por constructor.
func NewRepository(db *mongo.Database) *Repository {
	return &Repository{col: db.Collection(Collection)}
}

func key(userID bson.ObjectID, kind string, target bson.ObjectID) bson.D {
	return bson.D{{Key: "usuario_id", Value: userID}, {Key: "tipo", Value: kind}, {Key: "objetivo_id", Value: target}}
}

func byType(userID bson.ObjectID, kind string) bson.D {
	return bson.D{{Key: "usuario_id", Value: userID}, {Key: "tipo", Value: kind}}
}

func newest() bson.D {
	return bson.D{{Key: "creado_en", Value: -1}, {Key: "_id", Value: -1}}
}

// Find trae el favorito; ok=false si no está.
func (r *Repository) Find(ctx context.Context, userID bson.ObjectID, kind string, target bson.ObjectID) (Favorite, bool, error) {
	var f Favorite
	err := r.col.FindOne(ctx, key(userID, kind, target)).Decode(&f)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Favorite{}, false, nil
	}
	if err != nil {
		return Favorite{}, false, err
	}
	return f, true, nil
}

// Count cuenta los favoritos de un tipo.
func (r *Repository) Count(ctx context.Context, userID bson.ObjectID, kind string) (int64, error) {
	return r.col.CountDocuments(ctx, byType(userID, kind))
}

// Upsert guarda f con `$setOnInsert` sobre ux_favorito: repetirlo devuelve el mismo
// documento sin duplicarlo ni cambiarlo.
func (r *Repository) Upsert(ctx context.Context, f Favorite) (Favorite, error) {
	update := bson.D{{Key: "$setOnInsert", Value: bson.D{
		{Key: "_id", Value: f.ID}, {Key: "resumen", Value: f.Summary}, {Key: "creado_en", Value: f.CreatedAt},
	}}}
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)
	var out Favorite
	err := r.col.FindOneAndUpdate(ctx, key(f.UserID, f.Type, f.TargetID), update, opts).Decode(&out)
	if !mongo.IsDuplicateKeyError(err) {
		return out, err
	}
	// Dos marcas simultáneas: la otra ganó el upsert; se devuelve ese documento.
	found, ok, err := r.Find(ctx, f.UserID, f.Type, f.TargetID)
	if err == nil && !ok {
		err = mongo.ErrNoDocuments
	}
	return found, err
}

// Delete quita el favorito; no falla si no estaba.
func (r *Repository) Delete(ctx context.Context, userID bson.ObjectID, kind string, target bson.ObjectID) error {
	_, err := r.col.DeleteOne(ctx, key(userID, kind, target))
	return err
}

// DeleteTargets quita los favoritos de userID cuyos objetivos ya no existen.
func (r *Repository) DeleteTargets(ctx context.Context, userID bson.ObjectID, kind string, targets []bson.ObjectID) error {
	if len(targets) == 0 {
		return nil
	}
	filter := append(byType(userID, kind), bson.E{Key: "objetivo_id", Value: bson.D{{Key: "$in", Value: targets}}})
	_, err := r.col.DeleteMany(ctx, filter)
	return err
}

// Position es dónde sigue la lista (cursor firmado).
type Position struct {
	CreatedAt time.Time
	ID        bson.ObjectID
}

// List trae hasta limit+1 favoritos de un tipo, del más reciente al más viejo (ix_lista).
func (r *Repository) List(ctx context.Context, userID bson.ObjectID, kind string, after *Position, limit int) ([]Favorite, error) {
	filter := byType(userID, kind)
	if after != nil {
		filter = append(filter, bson.E{Key: "$or", Value: bson.A{
			bson.D{{Key: "creado_en", Value: bson.D{{Key: "$lt", Value: after.CreatedAt}}}},
			bson.D{{Key: "creado_en", Value: after.CreatedAt}, {Key: "_id", Value: bson.D{{Key: "$lt", Value: after.ID}}}},
		}})
	}
	return r.find(ctx, filter, options.Find().SetSort(newest()).SetLimit(int64(limit+1)))
}

// All trae hasta MaxPerType favoritos de cada tipo pedido, del más reciente al más viejo.
func (r *Repository) All(ctx context.Context, userID bson.ObjectID, kinds ...string) ([]Favorite, error) {
	out := []Favorite{}
	for _, kind := range kinds {
		rows, err := r.find(ctx, byType(userID, kind), options.Find().SetSort(newest()).SetLimit(MaxPerType))
		if err != nil {
			return nil, err
		}
		out = append(out, rows...)
	}
	return out, nil
}

// IDs trae solo los objetivos de un tipo (hasta MaxPerType).
func (r *Repository) IDs(ctx context.Context, userID bson.ObjectID, kind string) ([]bson.ObjectID, error) {
	opts := options.Find().SetSort(newest()).SetLimit(MaxPerType).SetProjection(bson.D{{Key: "objetivo_id", Value: 1}})
	rows, err := r.find(ctx, byType(userID, kind), opts)
	if err != nil {
		return nil, err
	}
	out := make([]bson.ObjectID, 0, len(rows))
	for _, f := range rows {
		out = append(out, f.TargetID)
	}
	return out, nil
}

// ListByUser trae todos los favoritos de userID (exportación ARCO).
func (r *Repository) ListByUser(ctx context.Context, userID bson.ObjectID) ([]Favorite, error) {
	return r.find(ctx, bson.D{{Key: "usuario_id", Value: userID}}, options.Find().SetSort(bson.D{{Key: "creado_en", Value: 1}}))
}

// DeleteByUser borra todos los favoritos de userID (borrado definitivo de la cuenta).
func (r *Repository) DeleteByUser(ctx context.Context, userID bson.ObjectID) error {
	_, err := r.col.DeleteMany(ctx, bson.D{{Key: "usuario_id", Value: userID}})
	return err
}

func (r *Repository) find(ctx context.Context, filter bson.D, opts *options.FindOptionsBuilder) ([]Favorite, error) {
	cur, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	out := []Favorite{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

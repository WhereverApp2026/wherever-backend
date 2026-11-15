// Package vistas guarda «Vistos recientemente» (nv-66, US-034): qué fichas abrió cada persona.
// Es historial: se borra a los 180 días, con «borrar historial» y con la cuenta.
package vistas

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	platformmongo "github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
)

// Collection guarda las vistas (A2 §3, solo vistas: búsquedas y descartes viven aparte).
const Collection = "historial_vistas"

// Límites de la spec.
const (
	Retention    = 180 * 24 * time.Hour
	MinGap       = 10 * time.Minute
	MaxPerHour   = 60
	DefaultLimit = 10
	MaxLimit     = 50
)

// Vista es un documento de `historial_vistas`: una por persona y lugar.
type Vista struct {
	ID        bson.ObjectID `bson:"_id"`
	UserID    bson.ObjectID `bson:"usuario_id"`
	NegocioID bson.ObjectID `bson:"negocio_id"`
	Origen    string        `bson:"origen,omitempty"`
	Fecha     time.Time     `bson:"fecha"`
	ExpiraEn  time.Time     `bson:"expira_en"`
}

// Indexes son los índices de `historial_vistas`. El único creador es scripts/indexes.
func Indexes() []platformmongo.IndexSet {
	return []platformmongo.IndexSet{{
		Collection: Collection,
		Models: []mongo.IndexModel{
			{Keys: bson.D{{Key: "usuario_id", Value: 1}, {Key: "negocio_id", Value: 1}}, Options: options.Index().SetName("ux_vista").SetUnique(true)},
			{Keys: bson.D{{Key: "usuario_id", Value: 1}, {Key: "fecha", Value: -1}}, Options: options.Index().SetName("ix_recientes")},
			{Keys: bson.D{{Key: "expira_en", Value: 1}}, Options: options.Index().SetName("ttl_vistas").SetExpireAfterSeconds(0)},
		},
	}}
}

// Repository lee y escribe `historial_vistas`.
type Repository struct {
	col *mongo.Collection
}

// NewRepository recibe la base por constructor.
func NewRepository(db *mongo.Database) *Repository {
	return &Repository{col: db.Collection(Collection)}
}

// Registrar guarda la vista con upsert si la anterior tiene más de 10 min. El filtro exige la
// vista vieja; si hay una reciente, el upsert choca con `ux_vista` y no se escribe nada (sin
// leer antes). Devuelve si escribió.
func (r *Repository) Registrar(ctx context.Context, user, negocio bson.ObjectID, origen string, now time.Time) (bool, error) {
	filter := bson.D{{Key: "usuario_id", Value: user}, {Key: "negocio_id", Value: negocio}, {Key: "fecha", Value: bson.D{{Key: "$lte", Value: now.Add(-MinGap)}}}}
	set := bson.D{{Key: "fecha", Value: now}, {Key: "expira_en", Value: now.Add(Retention)}}
	if origen != "" {
		set = append(set, bson.E{Key: "origen", Value: origen})
	}
	update := bson.D{{Key: "$set", Value: set}, {Key: "$setOnInsert", Value: bson.D{{Key: "_id", Value: bson.NewObjectID()}}}}
	_, err := r.col.UpdateOne(ctx, filter, update, options.UpdateOne().SetUpsert(true))
	if mongo.IsDuplicateKeyError(err) {
		return false, nil
	}
	return err == nil, err
}

// OtrosEnLaHora cuenta los lugares distintos de negocio que user vio en la última hora.
func (r *Repository) OtrosEnLaHora(ctx context.Context, user, negocio bson.ObjectID, now time.Time) (int64, error) {
	return r.col.CountDocuments(ctx, bson.D{
		{Key: "usuario_id", Value: user},
		{Key: "fecha", Value: bson.D{{Key: "$gt", Value: now.Add(-time.Hour)}}},
		{Key: "negocio_id", Value: bson.D{{Key: "$ne", Value: negocio}}},
	})
}

// Recientes trae las vistas de user de la más nueva a la más vieja.
func (r *Repository) Recientes(ctx context.Context, user bson.ObjectID, limit int64) ([]Vista, error) {
	opts := options.Find().SetSort(bson.D{{Key: "fecha", Value: -1}})
	if limit > 0 {
		opts.SetLimit(limit)
	}
	cur, err := r.col.Find(ctx, bson.D{{Key: "usuario_id", Value: user}}, opts)
	if err != nil {
		return nil, err
	}
	out := []Vista{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// BorrarTodo borra todas las vistas de user.
func (r *Repository) BorrarTodo(ctx context.Context, user bson.ObjectID) error {
	_, err := r.col.DeleteMany(ctx, bson.D{{Key: "usuario_id", Value: user}})
	return err
}

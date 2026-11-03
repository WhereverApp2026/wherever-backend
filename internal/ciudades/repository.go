// Package ciudades es el catálogo de ciudades para elegir la ciudad a mano sin permiso de
// ubicación (hoja A1 §2, spec nv-05): autocompletado por prefijo e importación por script.
// No trae datos en el código: se cargan con scripts/importar_ciudades.
package ciudades

import (
	"context"
	"errors"
	"regexp"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	platformmongo "github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/text"
)

// Collection es el nombre de la colección.
const Collection = "ciudades"

// ErrNotFound indica que no existe la ciudad.
var ErrNotFound = errors.New("ciudad no encontrada")

// Point es un GeoJSON Point `[longitud, latitud]`.
type Point struct {
	Type        string    `bson:"type"`
	Coordinates []float64 `bson:"coordinates"`
}

// City es un documento de `ciudades` (hoja A1 v3: `con_cobertura` en lugar de `activa_piloto`).
type City struct {
	ID             bson.ObjectID `bson:"_id"`
	Name           string        `bson:"nombre"`
	State          string        `bson:"estado"`
	NormalizedName string        `bson:"nombre_normalizado"`
	Center         Point         `bson:"centro"`
	Covered        bool          `bson:"con_cobertura"`
}

// Repository accede a `ciudades`.
type Repository struct {
	col *mongo.Collection
}

// NewRepository recibe la base por constructor.
func NewRepository(db *mongo.Database) *Repository {
	return &Repository{col: db.Collection(Collection)}
}

// Indexes son los índices de la hoja A1 §2.
func Indexes() []platformmongo.IndexSet {
	return []platformmongo.IndexSet{{
		Collection: Collection,
		Models: []mongo.IndexModel{
			{Keys: bson.D{{Key: "nombre_normalizado", Value: 1}, {Key: "estado", Value: 1}}, Options: options.Index().SetName("ux_ciudad").SetUnique(true)},
			{Keys: bson.D{{Key: "centro", Value: "2dsphere"}}, Options: options.Index().SetName("ix_centro")},
		},
	}}
}

// FindByID busca una ciudad por _id.
func (r *Repository) FindByID(ctx context.Context, id bson.ObjectID) (City, error) {
	var c City
	err := r.col.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&c)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return City{}, ErrNotFound
	}
	return c, err
}

// Autocomplete busca por prefijo de `nombre_normalizado` (usa ux_ciudad). El texto ya viene
// normalizado y se escapa: no se interpreta como expresión regular.
func (r *Repository) Autocomplete(ctx context.Context, normalizedPrefix string, limit int64) ([]City, error) {
	filter := bson.D{{Key: "nombre_normalizado", Value: bson.D{{Key: "$regex", Value: "^" + regexp.QuoteMeta(normalizedPrefix)}}}}
	opts := options.Find().
		SetSort(bson.D{{Key: "con_cobertura", Value: -1}, {Key: "nombre_normalizado", Value: 1}}).
		SetLimit(limit)
	cur, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	var out []City
	return out, cur.All(ctx, &out)
}

// Upsert crea o actualiza una ciudad por (nombre normalizado, estado); idempotente.
func (r *Repository) Upsert(ctx context.Context, c City) error {
	c.NormalizedName = text.Normalize(c.Name)
	_, err := r.col.UpdateOne(ctx,
		bson.D{{Key: "nombre_normalizado", Value: c.NormalizedName}, {Key: "estado", Value: c.State}},
		bson.D{
			{Key: "$set", Value: bson.D{{Key: "nombre", Value: c.Name}, {Key: "centro", Value: c.Center}, {Key: "con_cobertura", Value: c.Covered}}},
			{Key: "$setOnInsert", Value: bson.D{{Key: "_id", Value: bson.NewObjectID()}}},
		},
		options.UpdateOne().SetUpsert(true))
	return err
}

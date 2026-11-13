package errores

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	platformmongo "github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
)

// Collection guarda los errores de la app agrupados por huella (hoja J9 §1).
const Collection = "errores_app"

// OriginApp es el único `origen`: los errores del servidor siguen en CloudWatch (nv-13).
const OriginApp = "app"

// retention: un error se borra solo 30 días después de su última aparición.
const retention = 30 * 24 * time.Hour

// Report es un documento de `errores_app`. No lleva `usuario_id`: nada que purgar en ARCO.
type Report struct {
	ID          bson.ObjectID `bson:"_id"`
	Origin      string        `bson:"origen"`
	Version     string        `bson:"version"`
	Module      string        `bson:"modulo"`
	Message     string        `bson:"mensaje"`
	TraceID     string        `bson:"traza_id,omitempty"`
	Count       int64         `bson:"conteo"`
	Fingerprint string        `bson:"huella"`
	CreatedAt   time.Time     `bson:"creado_en"`
	LastAt      time.Time     `bson:"ultimo_en"`
}

// Position es dónde sigue la página del administrador: el último `ultimo_en` y su `_id`.
type Position struct {
	LastAt time.Time
	ID     bson.ObjectID
}

// Filter es la consulta del administrador; los campos vacíos no filtran.
type Filter struct {
	Module string
	Since  *time.Time
	After  *Position
}

// Indexes son los índices de `errores_app`. El único creador es scripts/indexes.
func Indexes() []platformmongo.IndexSet {
	return []platformmongo.IndexSet{{
		Collection: Collection,
		Models: []mongo.IndexModel{
			{Keys: bson.D{{Key: "huella", Value: 1}}, Options: options.Index().SetName("ux_huella").SetUnique(true)},
			{Keys: bson.D{{Key: "modulo", Value: 1}, {Key: "ultimo_en", Value: -1}}, Options: options.Index().SetName("ix_errores_modulo")},
			{Keys: bson.D{{Key: "ultimo_en", Value: 1}}, Options: options.Index().SetName("ttl_errores").SetExpireAfterSeconds(int32(retention / time.Second))},
		},
	}}
}

// Repository accede a `errores_app`.
type Repository struct {
	col *mongo.Collection
}

// NewRepository recibe la base por constructor.
func NewRepository(db *mongo.Database) *Repository {
	return &Repository{col: db.Collection(Collection)}
}

// Upsert suma una aparición al grupo de r.Fingerprint o lo crea. Si dos reportes nuevos
// chocan en ux_huella, el segundo reintenta y ya encuentra el documento.
func (r *Repository) Upsert(ctx context.Context, rep Report) error {
	set := bson.D{{Key: "ultimo_en", Value: rep.LastAt}, {Key: "version", Value: rep.Version}}
	if rep.TraceID != "" {
		set = append(set, bson.E{Key: "traza_id", Value: rep.TraceID})
	}
	update := bson.D{
		{Key: "$inc", Value: bson.D{{Key: "conteo", Value: 1}}},
		{Key: "$set", Value: set},
		{Key: "$setOnInsert", Value: bson.D{
			{Key: "origen", Value: rep.Origin}, {Key: "modulo", Value: rep.Module},
			{Key: "mensaje", Value: rep.Message}, {Key: "creado_en", Value: rep.LastAt},
		}},
	}
	filter := bson.D{{Key: "huella", Value: rep.Fingerprint}}
	opts := options.UpdateOne().SetUpsert(true)
	_, err := r.col.UpdateOne(ctx, filter, update, opts)
	if mongo.IsDuplicateKeyError(err) {
		_, err = r.col.UpdateOne(ctx, filter, update, opts)
	}
	return err
}

// List devuelve hasta limit+1 grupos del más reciente al más viejo; el de más dice que hay
// otra página.
func (r *Repository) List(ctx context.Context, f Filter, limit int) ([]Report, error) {
	filter := bson.D{}
	if f.Module != "" {
		filter = append(filter, bson.E{Key: "modulo", Value: f.Module})
	}
	if f.Since != nil {
		filter = append(filter, bson.E{Key: "ultimo_en", Value: bson.D{{Key: "$gte", Value: *f.Since}}})
	}
	if f.After != nil {
		filter = append(filter, bson.E{Key: "$or", Value: bson.A{
			bson.D{{Key: "ultimo_en", Value: bson.D{{Key: "$lt", Value: f.After.LastAt}}}},
			bson.D{{Key: "ultimo_en", Value: f.After.LastAt}, {Key: "_id", Value: bson.D{{Key: "$lt", Value: f.After.ID}}}},
		}})
	}
	opts := options.Find().SetSort(bson.D{{Key: "ultimo_en", Value: -1}, {Key: "_id", Value: -1}}).SetLimit(int64(limit) + 1)
	cur, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	out := []Report{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

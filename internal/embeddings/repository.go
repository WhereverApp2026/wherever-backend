package embeddings

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
)

// Collection guarda un vector por negocio, con el id del negocio como `_id`. Va aparte de
// `negocios` para que las listas y fichas no lean 768 números por documento.
const Collection = "vectores_negocios"

// Vector es un documento de `vectores_negocios`. No tiene datos de personas: fuera de ARCO.
type Vector struct {
	NegocioID     bson.ObjectID `bson:"_id"`
	Vector        []float32     `bson:"vector"`
	Version       string        `bson:"embedding_version"`
	Modelo        string        `bson:"modelo"`
	ActualizadoEn time.Time     `bson:"actualizado_en"`
}

// Repository lee los negocios visibles y escribe sus vectores.
type Repository struct {
	negocios *mongo.Collection
	vectores *mongo.Collection
}

// NewRepository recibe la base por constructor.
func NewRepository(db *mongo.Database) *Repository {
	return &Repository{negocios: db.Collection(negocios.Collection), vectores: db.Collection(Collection)}
}

// Fuentes devuelve los campos públicos de los negocios aprobados y no eliminados.
func (r *Repository) Fuentes(ctx context.Context) ([]Fuente, error) {
	proj := options.Find().SetProjection(bson.D{
		{Key: "nombre", Value: 1}, {Key: "categoria", Value: 1},
		{Key: "etiquetas", Value: 1}, {Key: "descripcion", Value: 1},
	}).SetSort(bson.D{{Key: "_id", Value: 1}})
	cur, err := r.negocios.Find(ctx, negocios.VisibleFilter(), proj)
	if err != nil {
		return nil, err
	}
	var out []Fuente
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Versiones devuelve la versión guardada de cada vector, sin leer los números.
func (r *Repository) Versiones(ctx context.Context) (map[bson.ObjectID]string, error) {
	cur, err := r.vectores.Find(ctx, bson.D{}, options.Find().SetProjection(bson.D{{Key: "embedding_version", Value: 1}}))
	if err != nil {
		return nil, err
	}
	var docs []Vector
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make(map[bson.ObjectID]string, len(docs))
	for _, d := range docs {
		out[d.NegocioID] = d.Version
	}
	return out, nil
}

// Guardar crea o reemplaza el vector de un negocio.
func (r *Repository) Guardar(ctx context.Context, v Vector) error {
	_, err := r.vectores.ReplaceOne(ctx, bson.D{{Key: "_id", Value: v.NegocioID}}, v, options.Replace().SetUpsert(true))
	return err
}

// BorrarExcepto quita los vectores de negocios que ya no son visibles.
func (r *Repository) BorrarExcepto(ctx context.Context, vigentes []bson.ObjectID) (int64, error) {
	res, err := r.vectores.DeleteMany(ctx, bson.D{{Key: "_id", Value: bson.D{{Key: "$nin", Value: vigentes}}}})
	if err != nil {
		return 0, err
	}
	return res.DeletedCount, nil
}

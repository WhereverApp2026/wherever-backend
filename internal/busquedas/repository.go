// Package busquedas expone el autocompletar, la búsqueda por texto con errores menores y las
// búsquedas recientes del usuario (spec nv-22).
package busquedas

import (
	"context"
	"regexp"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	platformmongo "github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
)

// Collection guarda las búsquedas recientes de cada usuario.
const Collection = "busquedas_recientes"

const (
	// MaxRecientes es cuántas búsquedas se guardan por usuario; la más vieja sale primero.
	MaxRecientes = 10
	// VidaReciente es cuánto vive una búsqueda reciente sin repetirse (índice TTL).
	VidaReciente = 90 * 24 * time.Hour
	// MaxNegocios es el tope de negocios aprobados que se comparan con errores menores, como
	// el plan B de nv-51. El piloto tiene 30; con muchos más hace falta otra técnica.
	MaxNegocios = 1000
	// maxPrefijo acota los negocios que trae el autocompletar antes de ordenarlos.
	maxPrefijo = 50
)

// Reciente es un documento de `busquedas_recientes`.
type Reciente struct {
	ID         bson.ObjectID `bson:"_id"`
	UserID     bson.ObjectID `bson:"usuario_id"`
	Text       string        `bson:"texto"`
	Normalized string        `bson:"texto_normalizado"`
	SearchedAt time.Time     `bson:"buscado_en"`
}

// Indexes: una búsqueda por usuario y texto, la lista por fecha y la caducidad. El único
// creador es scripts/indexes.
func Indexes() []platformmongo.IndexSet {
	return []platformmongo.IndexSet{{
		Collection: Collection,
		Models: []mongo.IndexModel{
			{
				Keys:    bson.D{{Key: "usuario_id", Value: 1}, {Key: "texto_normalizado", Value: 1}},
				Options: options.Index().SetName("ux_usuario_texto").SetUnique(true),
			},
			{
				Keys:    bson.D{{Key: "usuario_id", Value: 1}, {Key: "buscado_en", Value: -1}},
				Options: options.Index().SetName("ix_usuario_fecha"),
			},
			{
				Keys:    bson.D{{Key: "buscado_en", Value: 1}},
				Options: options.Index().SetName("ttl_buscado_en").SetExpireAfterSeconds(int32(VidaReciente / time.Second)),
			},
		},
	}}
}

// Repository lee `negocios` (solo aprobados) y escribe `busquedas_recientes`.
type Repository struct {
	negocios  *mongo.Collection
	recientes *mongo.Collection
}

// NewRepository recibe la base por constructor.
func NewRepository(db *mongo.Database) *Repository {
	return &Repository{negocios: db.Collection(negocios.Collection), recientes: db.Collection(Collection)}
}

func visibleWith(extra ...bson.E) bson.D {
	return append(negocios.VisibleFilter(), extra...)
}

func (r *Repository) find(ctx context.Context, filter bson.D, opts *options.FindOptionsBuilder) ([]negocios.Business, error) {
	cur, err := r.negocios.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	var out []negocios.Business
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Prefijo trae negocios aprobados cuyo `nombre_normalizado` tiene una palabra que empieza con
// `prefijo` (ya normalizado), mejor calificados primero. El texto va escapado.
func (r *Repository) Prefijo(ctx context.Context, prefijo string) ([]negocios.Business, error) {
	pattern := `(^| )` + regexp.QuoteMeta(prefijo)
	filter := visibleWith(bson.E{Key: "nombre_normalizado", Value: bson.D{{Key: "$regex", Value: pattern}}})
	opts := options.Find().
		SetSort(bson.D{{Key: "calificacion.promedio", Value: -1}, {Key: "_id", Value: 1}}).
		SetLimit(maxPrefijo).
		SetProjection(bson.D{
			{Key: "nombre", Value: 1}, {Key: "nombre_normalizado", Value: 1}, {Key: "slug", Value: 1},
			{Key: "categoria", Value: 1}, {Key: "calificacion", Value: 1},
		})
	return r.find(ctx, filter, opts)
}

// Texto busca con el índice `ix_texto` (español: sin acentos ni mayúsculas y con raíces,
// «tacos» encuentra «taco»). `consulta` son palabras sueltas, sin comillas ni guiones.
func (r *Repository) Texto(ctx context.Context, consulta string, limite int) ([]negocios.Business, error) {
	filter := visibleWith(bson.E{Key: "$text", Value: bson.D{{Key: "$search", Value: consulta}, {Key: "$language", Value: "spanish"}}})
	score := bson.D{{Key: "$meta", Value: "textScore"}}
	opts := options.Find().
		SetProjection(bson.D{{Key: "puntaje_texto", Value: score}}).
		SetSort(bson.D{{Key: "puntaje_texto", Value: score}, {Key: "calificacion.promedio", Value: -1}, {Key: "_id", Value: 1}}).
		SetLimit(int64(limite))
	return r.find(ctx, filter, opts)
}

// Visibles trae hasta MaxNegocios aprobados con lo que necesitan la tarjeta y la comparación.
func (r *Repository) Visibles(ctx context.Context) ([]negocios.Business, error) {
	opts := options.Find().
		SetSort(bson.D{{Key: "_id", Value: 1}}).
		SetLimit(MaxNegocios).
		SetProjection(bson.D{{Key: "descripcion", Value: 0}})
	return r.find(ctx, negocios.VisibleFilter(), opts)
}

// Guardar registra la búsqueda (repetirla solo actualiza la fecha) y deja las MaxRecientes
// más nuevas del usuario.
func (r *Repository) Guardar(ctx context.Context, userID bson.ObjectID, texto, normalizado string, now time.Time) error {
	filter := bson.D{{Key: "usuario_id", Value: userID}, {Key: "texto_normalizado", Value: normalizado}}
	update := bson.D{
		{Key: "$set", Value: bson.D{{Key: "texto", Value: texto}, {Key: "buscado_en", Value: now}}},
		{Key: "$setOnInsert", Value: bson.D{{Key: "_id", Value: bson.NewObjectID()}}},
	}
	_, err := r.recientes.UpdateOne(ctx, filter, update, options.UpdateOne().SetUpsert(true))
	if err != nil && !mongo.IsDuplicateKeyError(err) { // dos iguales a la vez: la otra ya quedó
		return err
	}
	return r.recortar(ctx, userID)
}

func (r *Repository) recortar(ctx context.Context, userID bson.ObjectID) error {
	opts := options.Find().
		SetSort(bson.D{{Key: "buscado_en", Value: -1}, {Key: "_id", Value: -1}}).
		SetSkip(MaxRecientes).
		SetProjection(bson.D{{Key: "_id", Value: 1}})
	cur, err := r.recientes.Find(ctx, bson.D{{Key: "usuario_id", Value: userID}}, opts)
	if err != nil {
		return err
	}
	var old []struct {
		ID bson.ObjectID `bson:"_id"`
	}
	if err := cur.All(ctx, &old); err != nil || len(old) == 0 {
		return err
	}
	ids := make(bson.A, 0, len(old))
	for _, o := range old {
		ids = append(ids, o.ID)
	}
	_, err = r.recientes.DeleteMany(ctx, bson.D{{Key: "usuario_id", Value: userID}, {Key: "_id", Value: bson.D{{Key: "$in", Value: ids}}}})
	return err
}

// Recientes devuelve las búsquedas del usuario, de la más nueva a la más vieja.
func (r *Repository) Recientes(ctx context.Context, userID bson.ObjectID) ([]Reciente, error) {
	opts := options.Find().SetSort(bson.D{{Key: "buscado_en", Value: -1}, {Key: "_id", Value: -1}}).SetLimit(MaxRecientes)
	cur, err := r.recientes.Find(ctx, bson.D{{Key: "usuario_id", Value: userID}}, opts)
	if err != nil {
		return nil, err
	}
	var out []Reciente
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Borrar elimina todas las búsquedas del usuario (botón «Borrar» y ARCO).
func (r *Repository) Borrar(ctx context.Context, userID bson.ObjectID) error {
	_, err := r.recientes.DeleteMany(ctx, bson.D{{Key: "usuario_id", Value: userID}})
	return err
}

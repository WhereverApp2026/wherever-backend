// Package menu es el menú del negocio por platillo (spec nv-56, hoja A5 §2): modelo,
// repositorio, service, handler y DTO. Usa `internal/negocios` para la propiedad del negocio;
// ninguno de los dos lo usa a él salvo el panel (baja y exportación).
package menu

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	platformmongo "github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
)

// Collection guarda un documento por platillo (A5 §2).
const Collection = "menu_items"

// MaxItems es el tope de platillos por negocio (409 `limite_menu`).
const MaxItems = 200

// Errores del repositorio.
var (
	ErrNotFound = errors.New("menu: platillo no encontrado")
	ErrVersion  = errors.New("menu: versión vieja")
)

// Item es un platillo.
type Item struct {
	ID            bson.ObjectID `bson:"_id"`
	NegocioID     bson.ObjectID `bson:"negocio_id"`
	Seccion       string        `bson:"seccion"`
	Nombre        string        `bson:"nombre"`
	Descripcion   string        `bson:"descripcion"`
	Precio        float64       `bson:"precio"`
	Etiquetas     []string      `bson:"etiquetas"`
	ImagenID      string        `bson:"imagen_id,omitempty"`
	FotoURL       string        `bson:"foto_url,omitempty"`
	Disponible    bool          `bson:"disponible"`
	Orden         int           `bson:"orden"`
	Version       int           `bson:"version"`
	CreadoEn      time.Time     `bson:"creado_en"`
	ActualizadoEn time.Time     `bson:"actualizado_en"`
}

// Indexes son `ix_menu` (lectura por sección y orden) e `ix_menu_texto` (búsqueda dentro del
// negocio, en español: sin acentos ni mayúsculas). El único creador es scripts/indexes.
func Indexes() []platformmongo.IndexSet {
	return []platformmongo.IndexSet{{
		Collection: Collection,
		Models: []mongo.IndexModel{
			{Keys: bson.D{{Key: "negocio_id", Value: 1}, {Key: "seccion", Value: 1}, {Key: "orden", Value: 1}}, Options: options.Index().SetName("ix_menu")},
			{Keys: bson.D{{Key: "negocio_id", Value: 1}, {Key: "nombre", Value: "text"}, {Key: "descripcion", Value: "text"}},
				Options: options.Index().SetName("ix_menu_texto").SetDefaultLanguage("spanish")},
		},
	}}
}

// Repository lee y escribe `menu_items`; toda consulta filtra por negocio.
type Repository struct {
	col *mongo.Collection
}

// NewRepository recibe la base por constructor.
func NewRepository(db *mongo.Database) *Repository {
	return &Repository{col: db.Collection(Collection)}
}

// Count cuenta los platillos del negocio.
func (r *Repository) Count(ctx context.Context, negocio bson.ObjectID) (int64, error) {
	return r.col.CountDocuments(ctx, bson.D{{Key: "negocio_id", Value: negocio}})
}

// Insert guarda un platillo nuevo.
func (r *Repository) Insert(ctx context.Context, it *Item) error {
	_, err := r.col.InsertOne(ctx, it)
	return err
}

// Find trae un platillo del negocio.
func (r *Repository) Find(ctx context.Context, negocio, id bson.ObjectID) (Item, error) {
	var it Item
	err := r.col.FindOne(ctx, bson.D{{Key: "_id", Value: id}, {Key: "negocio_id", Value: negocio}}).Decode(&it)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Item{}, ErrNotFound
	}
	return it, err
}

// UpdateIf aplica `$set` con la versión esperada; ErrVersion si cambió.
func (r *Repository) UpdateIf(ctx context.Context, negocio, id bson.ObjectID, version int, set bson.D) (Item, error) {
	set = append(set, bson.E{Key: "actualizado_en", Value: time.Now().UTC()})
	var it Item
	err := r.col.FindOneAndUpdate(ctx,
		bson.D{{Key: "_id", Value: id}, {Key: "negocio_id", Value: negocio}, {Key: "version", Value: version}},
		bson.D{{Key: "$set", Value: set}, {Key: "$inc", Value: bson.D{{Key: "version", Value: 1}}}},
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&it)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Item{}, ErrVersion
	}
	return it, err
}

// DeleteIf borra el platillo con la versión esperada; ErrVersion si cambió.
func (r *Repository) DeleteIf(ctx context.Context, negocio, id bson.ObjectID, version int) error {
	res, err := r.col.DeleteOne(ctx, bson.D{{Key: "_id", Value: id}, {Key: "negocio_id", Value: negocio}, {Key: "version", Value: version}})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return ErrVersion
	}
	return nil
}

// Publicos trae los platillos disponibles del negocio en orden; `consulta` (palabras sueltas)
// usa `ix_menu_texto`.
func (r *Repository) Publicos(ctx context.Context, negocio bson.ObjectID, seccion, consulta string) ([]Item, error) {
	filter := bson.D{{Key: "negocio_id", Value: negocio}, {Key: "disponible", Value: true}}
	if seccion != "" {
		filter = append(filter, bson.E{Key: "seccion", Value: seccion})
	}
	if consulta != "" {
		filter = append(filter, bson.E{Key: "$text", Value: bson.D{{Key: "$search", Value: consulta}, {Key: "$language", Value: "spanish"}}})
	}
	return r.find(ctx, filter)
}

// DeNegocio trae todos los platillos del negocio en orden (exportación y baja).
func (r *Repository) DeNegocio(ctx context.Context, negocio bson.ObjectID) ([]Item, error) {
	return r.find(ctx, bson.D{{Key: "negocio_id", Value: negocio}})
}

func (r *Repository) find(ctx context.Context, filter bson.D) ([]Item, error) {
	cur, err := r.col.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "orden", Value: 1}, {Key: "_id", Value: 1}}).SetLimit(MaxItems))
	if err != nil {
		return nil, err
	}
	out := []Item{}
	return out, cur.All(ctx, &out)
}

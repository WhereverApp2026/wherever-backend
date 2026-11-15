package eventos

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	platformmongo "github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
)

// Errores del repositorio; el service los traduce.
var (
	ErrNoEncontrado = errors.New("evento no encontrado")
	ErrVersion      = errors.New("el evento cambió")
)

// Indexes son los índices de `eventos`. El único creador es scripts/indexes.
func Indexes() []platformmongo.IndexSet {
	return []platformmongo.IndexSet{{
		Collection: Collection,
		Models: []mongo.IndexModel{
			{Keys: bson.D{{Key: "ubicacion", Value: "2dsphere"}, {Key: "estado", Value: 1}, {Key: "inicio", Value: 1}}, Options: options.Index().SetName("ix_eventos_cerca")},
			{Keys: bson.D{{Key: "fin", Value: 1}}, Options: options.Index().SetName("ix_eventos_fin")},
			{Keys: bson.D{{Key: "creado_por", Value: 1}}, Options: options.Index().SetName("ix_eventos_creador")},
			{
				Keys: bson.D{{Key: "negocio_id", Value: 1}},
				Options: options.Index().SetName("ix_eventos_negocio").
					SetPartialFilterExpression(bson.D{{Key: "negocio_id", Value: bson.D{{Key: "$exists", Value: true}}}}),
			},
		},
	}}
}

// Repository lee y escribe `eventos`.
type Repository struct {
	col *mongo.Collection
}

// NewRepository recibe la base por constructor.
func NewRepository(db *mongo.Database) *Repository {
	return &Repository{col: db.Collection(Collection)}
}

// Insertar guarda un evento nuevo.
func (r *Repository) Insertar(ctx context.Context, e *Evento) error {
	_, err := r.col.InsertOne(ctx, e)
	return err
}

// Buscar trae un evento por id.
func (r *Repository) Buscar(ctx context.Context, id bson.ObjectID) (Evento, error) {
	var e Evento
	err := r.col.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&e)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Evento{}, ErrNoEncontrado
	}
	return e, err
}

// Filtro es la consulta de GET /eventos ya resuelta.
type Filtro struct {
	Lat, Lng     float64
	RadioM       float64
	Desde, Hasta time.Time
	Ahora        time.Time
	Tipo         string
}

// Posicion es el último evento de la página anterior (inicio, distancia, id).
type Posicion struct {
	Inicio     time.Time
	DistanciaM float64
	ID         bson.ObjectID
}

// Cercanos lista los eventos visibles por `inicio`, distancia e id con `$geoNear`. Trae uno de
// más para saber si hay siguiente página.
func (r *Repository) Cercanos(ctx context.Context, f Filtro, after *Posicion, limite int) ([]Fila, error) {
	query := bson.D{
		{Key: "estado", Value: EstadoPublicado},
		{Key: "en_revision", Value: bson.D{{Key: "$ne", Value: true}}},
		{Key: "inicio", Value: bson.D{{Key: "$gte", Value: f.Desde}, {Key: "$lt", Value: f.Hasta}}},
		{Key: "fin", Value: bson.D{{Key: "$gt", Value: f.Ahora}}},
	}
	if f.Tipo != "" {
		query = append(query, bson.E{Key: "tipo", Value: f.Tipo})
	}
	pipeline := bson.A{
		bson.D{{Key: "$geoNear", Value: bson.D{
			{Key: "near", Value: bson.D{{Key: "type", Value: "Point"}, {Key: "coordinates", Value: bson.A{f.Lng, f.Lat}}}},
			{Key: "distanceField", Value: "distancia_m"},
			{Key: "maxDistance", Value: f.RadioM},
			{Key: "key", Value: "ubicacion"},
			{Key: "query", Value: query},
		}}},
	}
	if after != nil {
		pipeline = append(pipeline, bson.D{{Key: "$match", Value: bson.D{{Key: "$or", Value: bson.A{
			bson.D{{Key: "inicio", Value: bson.D{{Key: "$gt", Value: after.Inicio}}}},
			bson.D{{Key: "inicio", Value: after.Inicio}, {Key: "distancia_m", Value: bson.D{{Key: "$gt", Value: after.DistanciaM}}}},
			bson.D{{Key: "inicio", Value: after.Inicio}, {Key: "distancia_m", Value: after.DistanciaM}, {Key: "_id", Value: bson.D{{Key: "$gt", Value: after.ID}}}},
		}}}}})
	}
	pipeline = append(pipeline,
		bson.D{{Key: "$sort", Value: bson.D{{Key: "inicio", Value: 1}, {Key: "distancia_m", Value: 1}, {Key: "_id", Value: 1}}}},
		bson.D{{Key: "$limit", Value: limite + 1}},
	)
	cur, err := r.col.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	out := []Fila{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Reemplazar guarda el evento si sigue en `version`; sube la versión. ErrVersion si cambió,
// ErrNoEncontrado si ya no existe.
func (r *Repository) Reemplazar(ctx context.Context, e *Evento, version int) error {
	e.Version = version + 1
	res, err := r.col.ReplaceOne(ctx, bson.D{{Key: "_id", Value: e.ID}, {Key: "version", Value: version}}, e)
	if err != nil {
		return err
	}
	return r.resultado(ctx, e.ID, res.MatchedCount)
}

// Borrar borra el evento si sigue en `version` (borrado físico: no es historial de nadie).
func (r *Repository) Borrar(ctx context.Context, id bson.ObjectID, version int) error {
	res, err := r.col.DeleteOne(ctx, bson.D{{Key: "_id", Value: id}, {Key: "version", Value: version}})
	if err != nil {
		return err
	}
	return r.resultado(ctx, id, res.DeletedCount)
}

func (r *Repository) resultado(ctx context.Context, id bson.ObjectID, n int64) error {
	if n == 1 {
		return nil
	}
	if _, err := r.Buscar(ctx, id); err != nil {
		return err
	}
	return ErrVersion
}

// Decidir aplica la decisión de moderación: aprobado lo saca de revisión; rechazado lo deja
// en borrador fuera de revisión. Un evento que ya no está en revisión no cambia.
func (r *Repository) Decidir(ctx context.Context, id bson.ObjectID, aprobado bool, now time.Time) error {
	set := bson.D{{Key: "actualizado_en", Value: now}}
	if !aprobado {
		set = append(set, bson.E{Key: "estado", Value: EstadoBorrador})
	}
	_, err := r.col.UpdateOne(ctx, bson.D{{Key: "_id", Value: id}, {Key: "en_revision", Value: true}},
		bson.D{{Key: "$set", Value: set}, {Key: "$unset", Value: bson.D{{Key: "en_revision", Value: ""}}}, {Key: "$inc", Value: bson.D{{Key: "version", Value: 1}}}})
	return err
}

// DelCreador trae los eventos que creó userID (ARCO).
func (r *Repository) DelCreador(ctx context.Context, userID bson.ObjectID) ([]Evento, error) {
	cur, err := r.col.Find(ctx, bson.D{{Key: "creado_por", Value: userID}}, options.Find().SetSort(bson.D{{Key: "inicio", Value: 1}}))
	if err != nil {
		return nil, err
	}
	out := []Evento{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// PurgarCreador borra los eventos de userID sin negocio y le quita `creado_por` a los del
// negocio, que siguen siendo del negocio.
func (r *Repository) PurgarCreador(ctx context.Context, userID bson.ObjectID) error {
	if _, err := r.col.DeleteMany(ctx, bson.D{{Key: "creado_por", Value: userID}, {Key: "negocio_id", Value: bson.D{{Key: "$exists", Value: false}}}}); err != nil {
		return err
	}
	_, err := r.col.UpdateMany(ctx, bson.D{{Key: "creado_por", Value: userID}}, bson.D{{Key: "$unset", Value: bson.D{{Key: "creado_por", Value: ""}}}})
	return err
}

package moderacion

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
	ErrNoEncontrado = errors.New("moderacion: no encontrado")
	ErrYaDecidido   = errors.New("moderacion: ya decidido o cambió la versión")
	ErrYaExisten    = errors.New("moderacion: las reglas ya existen")
)

// Indexes son los índices de moderacion_cola; moderacion_reglas solo usa _id. El único creador
// es scripts/indexes.
func Indexes() []platformmongo.IndexSet {
	return []platformmongo.IndexSet{{
		Collection: ColeccionCola,
		Models: []mongo.IndexModel{
			{
				// Un recurso no entra dos veces mientras espera revisión.
				Keys: bson.D{{Key: "tipo", Value: 1}, {Key: "referencia", Value: 1}},
				Options: options.Index().SetName("ux_tipo_referencia_pendiente").SetUnique(true).
					SetPartialFilterExpression(bson.D{{Key: "estado", Value: EstadoPendiente}}),
			},
			{Keys: bson.D{{Key: "estado", Value: 1}, {Key: "creado_en", Value: 1}, {Key: "_id", Value: 1}}, Options: options.Index().SetName("ix_estado_creado")},
			{Keys: bson.D{{Key: "autor_id", Value: 1}}, Options: options.Index().SetName("ix_autor")},
		},
	}}
}

// Repository lee y escribe moderacion_cola y moderacion_reglas. Decidir es una sola operación
// condicionada a que siga pendiente y con la misma versión: dos administradores no deciden el
// mismo elemento dos veces.
type Repository struct {
	cola   *mongo.Collection
	reglas *mongo.Collection
}

// NewRepository recibe la base por constructor.
func NewRepository(db *mongo.Database) *Repository {
	return &Repository{cola: db.Collection(ColeccionCola), reglas: db.Collection(ColeccionReglas)}
}

// Insertar guarda un elemento nuevo; si el recurso ya espera revisión devuelve el existente.
func (r *Repository) Insertar(ctx context.Context, it Item) (Item, error) {
	_, err := r.cola.InsertOne(ctx, it)
	if mongo.IsDuplicateKeyError(err) {
		var prev Item
		err = r.cola.FindOne(ctx, bson.D{{Key: "tipo", Value: it.Tipo}, {Key: "referencia", Value: it.Referencia}, {Key: "estado", Value: EstadoPendiente}}).Decode(&prev)
		return prev, err
	}
	return it, err
}

// Buscar trae un elemento por id.
func (r *Repository) Buscar(ctx context.Context, id bson.ObjectID) (Item, error) {
	var it Item
	err := r.cola.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&it)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Item{}, ErrNoEncontrado
	}
	return it, err
}

// Posicion es dónde sigue la página: el último entregado por creado_en y _id.
type Posicion struct {
	CreadoEn time.Time
	ID       bson.ObjectID
}

// Listar trae hasta limit+1 elementos del estado, del más viejo al más nuevo (la cola se
// atiende en orden de llegada). tipo vacío = todos.
func (r *Repository) Listar(ctx context.Context, estado, tipo string, after *Posicion, limit int) ([]Item, error) {
	filter := bson.D{{Key: "estado", Value: estado}}
	if tipo != "" {
		filter = append(filter, bson.E{Key: "tipo", Value: tipo})
	}
	if after != nil {
		filter = append(filter, bson.E{Key: "$or", Value: bson.A{
			bson.D{{Key: "creado_en", Value: bson.D{{Key: "$gt", Value: after.CreadoEn}}}},
			bson.D{{Key: "creado_en", Value: after.CreadoEn}, {Key: "_id", Value: bson.D{{Key: "$gt", Value: after.ID}}}},
		}})
	}
	opts := options.Find().SetSort(bson.D{{Key: "creado_en", Value: 1}, {Key: "_id", Value: 1}}).SetLimit(int64(limit + 1))
	cur, err := r.cola.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	out := []Item{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Decidir aplica la decisión si el elemento sigue pendiente y en la versión dada.
// ErrYaDecidido si ya se decidió o cambió; ErrNoEncontrado si no existe.
func (r *Repository) Decidir(ctx context.Context, id bson.ObjectID, version int, estado, motivo string, revisor bson.ObjectID, at time.Time) (Item, error) {
	set := bson.D{{Key: "estado", Value: estado}, {Key: "revisor_id", Value: revisor}, {Key: "decidido_en", Value: at}}
	if motivo != "" {
		set = append(set, bson.E{Key: "motivo", Value: motivo})
	}
	var it Item
	err := r.cola.FindOneAndUpdate(ctx,
		bson.D{{Key: "_id", Value: id}, {Key: "estado", Value: EstadoPendiente}, {Key: "version", Value: version}},
		bson.D{{Key: "$set", Value: set}, {Key: "$inc", Value: bson.D{{Key: "version", Value: 1}}}},
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&it)
	if errors.Is(err, mongo.ErrNoDocuments) {
		if _, err := r.Buscar(ctx, id); err != nil {
			return Item{}, err
		}
		return Item{}, ErrYaDecidido
	}
	return it, err
}

// BorrarDeAutor borra los elementos del usuario (ARCO). Las decisiones de un administrador
// sobre contenido ajeno no son datos del administrador y se quedan.
func (r *Repository) BorrarDeAutor(ctx context.Context, autor bson.ObjectID) error {
	_, err := r.cola.DeleteMany(ctx, bson.D{{Key: "autor_id", Value: autor}})
	return err
}

// ListarDeAutor trae lo que el usuario tiene en la cola (exportación ARCO), hasta 500.
func (r *Repository) ListarDeAutor(ctx context.Context, autor bson.ObjectID) ([]Item, error) {
	cur, err := r.cola.Find(ctx, bson.D{{Key: "autor_id", Value: autor}}, options.Find().SetLimit(500))
	if err != nil {
		return nil, err
	}
	out := []Item{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// LeerReglas trae las reglas guardadas; ErrNoEncontrado si nunca se guardaron.
func (r *Repository) LeerReglas(ctx context.Context) (reglasDoc, error) {
	var d reglasDoc
	err := r.reglas.FindOne(ctx, bson.D{{Key: "_id", Value: idReglas}}).Decode(&d)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return reglasDoc{}, ErrNoEncontrado
	}
	return d, err
}

// CrearReglas guarda las primeras reglas; ErrYaExisten si ya había.
func (r *Repository) CrearReglas(ctx context.Context, d reglasDoc) error {
	d.ID = idReglas
	_, err := r.reglas.InsertOne(ctx, d)
	if mongo.IsDuplicateKeyError(err) {
		return ErrYaExisten
	}
	return err
}

// ActualizarReglas reemplaza las reglas si siguen en la versión dada. ErrYaDecidido si cambió
// la versión; ErrNoEncontrado si nunca se guardaron.
func (r *Repository) ActualizarReglas(ctx context.Context, version int, d reglasDoc) (reglasDoc, error) {
	d.ID = idReglas
	d.Version = version + 1
	res, err := r.reglas.ReplaceOne(ctx, bson.D{{Key: "_id", Value: idReglas}, {Key: "version", Value: version}}, d)
	if err != nil {
		return reglasDoc{}, err
	}
	if res.MatchedCount == 0 {
		if _, err := r.LeerReglas(ctx); err != nil {
			return reglasDoc{}, err
		}
		return reglasDoc{}, ErrYaDecidido
	}
	return d, nil
}

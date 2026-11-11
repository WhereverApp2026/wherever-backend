package planes

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	platformmongo "github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
)

// Errores del repositorio. El service los traduce a 404 y 409 (como el perfil de nv-05).
var (
	ErrNotFound        = errors.New("plan no encontrado")
	ErrVersionConflict = errors.New("la versión no coincide")
)

// Indexes son los 2 índices de la spec. El único creador es scripts/indexes.
func Indexes() []platformmongo.IndexSet {
	return []platformmongo.IndexSet{{
		Collection: Collection,
		Models: []mongo.IndexModel{
			{Keys: bson.D{{Key: "dueno_id", Value: 1}, {Key: "estado", Value: 1}, {Key: "fecha", Value: 1}}, Options: options.Index().SetName("ix_dueno_estado_fecha")},
			{Keys: bson.D{{Key: "estado", Value: 1}, {Key: "fecha", Value: 1}}, Options: options.Index().SetName("ix_estado_fecha")},
		},
	}}
}

// Repository lee y escribe `planes`. Toda consulta filtra por dueño y estado.
type Repository struct {
	col *mongo.Collection
	now func() time.Time
}

// NewRepository recibe la base por constructor.
func NewRepository(db *mongo.Database) *Repository {
	return &Repository{col: db.Collection(Collection), now: time.Now}
}

func ownedActive(owner, id bson.ObjectID) bson.D {
	return bson.D{{Key: "_id", Value: id}, {Key: "dueno_id", Value: owner}, {Key: "estado", Value: StatusActive}}
}

// Create guarda un plan nuevo y ya recalculado: versión 1 y estado activo.
func (r *Repository) Create(ctx context.Context, p *Plan) error {
	now := r.now().UTC()
	if p.ID.IsZero() {
		p.ID = bson.NewObjectID()
	}
	p.Estado, p.Version, p.CreadoEn, p.ActualizadoEn = StatusActive, 1, now, now
	_, err := r.col.InsertOne(ctx, p)
	return err
}

// FindOwned trae el plan activo del dueño.
func (r *Repository) FindOwned(ctx context.Context, owner, id bson.ObjectID) (Plan, error) {
	var p Plan
	err := r.col.FindOne(ctx, ownedActive(owner, id)).Decode(&p)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Plan{}, ErrNotFound
	}
	return p, err
}

// UpdateIfVersion reemplaza lo editable del plan solo si la versión sigue siendo `version`
// (If-Match) y sube la versión. El plan debe venir recalculado.
func (r *Repository) UpdateIfVersion(ctx context.Context, p *Plan, version int) error {
	set := bson.D{
		{Key: "nombre", Value: p.Nombre}, {Key: "fecha", Value: p.Fecha},
		{Key: "personas", Value: p.Personas}, {Key: "presupuesto", Value: p.Presupuesto},
		{Key: "movilidad", Value: p.Movilidad}, {Key: "paradas", Value: p.Paradas},
		{Key: "traslados", Value: p.Traslados}, {Key: "costo_total", Value: p.CostoTotal},
		{Key: "costo_por_persona", Value: p.CostoPorPersona}, {Key: "actualizado_en", Value: r.now().UTC()},
	}
	filter := append(ownedActive(p.OwnerID, p.ID), bson.E{Key: "version", Value: version})
	update := bson.D{{Key: "$set", Value: set}, {Key: "$inc", Value: bson.D{{Key: "version", Value: 1}}}}
	var updated Plan
	err := r.col.FindOneAndUpdate(ctx, filter, update, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&updated)
	if errors.Is(err, mongo.ErrNoDocuments) {
		if _, findErr := r.FindOwned(ctx, p.OwnerID, p.ID); findErr == nil {
			return ErrVersionConflict
		}
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	*p = updated
	return nil
}

// ListByOwner trae todos los planes del dueño (activos y borrados) para exportarlos.
func (r *Repository) ListByOwner(ctx context.Context, owner bson.ObjectID) ([]Plan, error) {
	cur, err := r.col.Find(ctx, bson.D{{Key: "dueno_id", Value: owner}}, options.Find().SetSort(bson.D{{Key: "fecha", Value: 1}}))
	if err != nil {
		return nil, err
	}
	out := []Plan{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteByOwner borra todos los planes del dueño (borrado definitivo de la cuenta).
func (r *Repository) DeleteByOwner(ctx context.Context, owner bson.ObjectID) error {
	_, err := r.col.DeleteMany(ctx, bson.D{{Key: "dueno_id", Value: owner}})
	return err
}

// DeleteIfVersion hace el borrado lógico del plan del dueño si la versión coincide.
func (r *Repository) DeleteIfVersion(ctx context.Context, owner, id bson.ObjectID, version int) error {
	filter := append(ownedActive(owner, id), bson.E{Key: "version", Value: version})
	update := bson.D{
		{Key: "$set", Value: bson.D{{Key: "estado", Value: StatusDeleted}, {Key: "actualizado_en", Value: r.now().UTC()}}},
		{Key: "$inc", Value: bson.D{{Key: "version", Value: 1}}},
	}
	res, err := r.col.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	if res.MatchedCount == 1 {
		return nil
	}
	if _, findErr := r.FindOwned(ctx, owner, id); findErr == nil {
		return ErrVersionConflict
	}
	return ErrNotFound
}

// Position es el último plan entregado de una página (fecha y id desempata).
type Position struct {
	Fecha time.Time
	ID    bson.ObjectID
}

// PageQuery pide una página de planes activos del dueño. Since y Before acotan `fecha`
// (cero = sin límite); Desc ordena de la más reciente a la más vieja.
type PageQuery struct {
	Owner  bson.ObjectID
	Since  time.Time
	Before time.Time
	Desc   bool
	After  *Position
	Limit  int
}

// List trae hasta Limit+1 planes activos del dueño (el extra dice si hay otra página).
// Usa ix_dueno_estado_fecha.
func (r *Repository) List(ctx context.Context, q PageQuery) ([]Plan, error) {
	dir := 1
	if q.Desc {
		dir = -1
	}
	opts := options.Find().SetSort(bson.D{{Key: "fecha", Value: dir}, {Key: "_id", Value: dir}}).SetLimit(int64(q.Limit + 1))
	cur, err := r.col.Find(ctx, pageFilter(q), opts)
	if err != nil {
		return nil, err
	}
	out := []Plan{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func pageFilter(q PageQuery) bson.D {
	filter := bson.D{{Key: "dueno_id", Value: q.Owner}, {Key: "estado", Value: StatusActive}}
	fecha := bson.D{}
	if !q.Since.IsZero() {
		fecha = append(fecha, bson.E{Key: "$gte", Value: q.Since})
	}
	if !q.Before.IsZero() {
		fecha = append(fecha, bson.E{Key: "$lt", Value: q.Before})
	}
	if len(fecha) > 0 {
		filter = append(filter, bson.E{Key: "fecha", Value: fecha})
	}
	if q.After != nil {
		op := "$gt"
		if q.Desc {
			op = "$lt"
		}
		filter = append(filter, bson.E{Key: "$or", Value: bson.A{
			bson.D{{Key: "fecha", Value: bson.D{{Key: op, Value: q.After.Fecha}}}},
			bson.D{{Key: "fecha", Value: q.After.Fecha}, {Key: "_id", Value: bson.D{{Key: op, Value: q.After.ID}}}},
		}})
	}
	return filter
}

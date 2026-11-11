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

// Errores del repositorio. nv-30 los traduce a 404 y 409 (como el perfil de nv-05).
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

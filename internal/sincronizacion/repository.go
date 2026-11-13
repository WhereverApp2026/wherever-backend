// Package sincronizacion deja a la app bajar solo lo que cambió desde su última vez (nv-46,
// TEC-33): un contador por usuario (`sync_seq`) que no depende de ningún reloj, y los
// borrados (`borrados`, 30 días).
package sincronizacion

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	platformmongo "github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
)

// Colecciones.
const (
	StateCollection      = "sync_estado"
	TombstonesCollection = "borrados"
)

// Retention es cuánto se guardan los borrados; quien lleva más sin sincronizar baja todo.
const Retention = 30 * 24 * time.Hour

// KindPlan es el tipo de un plan borrado.
const KindPlan = "plan"

// Tombstone es un borrado: la app quita la entidad de su copia local.
type Tombstone struct {
	ID        bson.ObjectID `bson:"_id"`
	UserID    bson.ObjectID `bson:"usuario_id"`
	Kind      string        `bson:"tipo"`
	EntityID  bson.ObjectID `bson:"entidad_id"`
	Seq       int64         `bson:"sync_seq"`
	ExpiresAt time.Time     `bson:"expira_en"`
}

// Indexes son los índices de `borrados` (`sync_estado` usa `_id`).
func Indexes() []platformmongo.IndexSet {
	return []platformmongo.IndexSet{{
		Collection: TombstonesCollection,
		Models: []mongo.IndexModel{
			{Keys: bson.D{{Key: "usuario_id", Value: 1}, {Key: "sync_seq", Value: 1}}, Options: options.Index().SetName("ix_usuario_seq")},
			{Keys: bson.D{{Key: "expira_en", Value: 1}}, Options: options.Index().SetName("ttl_borrado").SetExpireAfterSeconds(0)},
		},
	}}
}

// Repository guarda el contador de cada usuario y sus borrados.
type Repository struct {
	state      *mongo.Collection
	tombstones *mongo.Collection
	now        func() time.Time
}

// NewRepository recibe la base por constructor.
func NewRepository(db *mongo.Database) *Repository {
	return &Repository{state: db.Collection(StateCollection), tombstones: db.Collection(TombstonesCollection), now: time.Now}
}

// Next sube el contador del usuario y devuelve el valor nuevo: una sola operación atómica,
// así dos escrituras simultáneas nunca reciben el mismo número ni dejan huecos.
func (r *Repository) Next(ctx context.Context, userID bson.ObjectID) (int64, error) {
	var doc struct {
		Seq int64 `bson:"seq"`
	}
	err := r.state.FindOneAndUpdate(ctx, bson.D{{Key: "_id", Value: userID}},
		bson.D{{Key: "$inc", Value: bson.D{{Key: "seq", Value: 1}}}},
		options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)).Decode(&doc)
	return doc.Seq, err
}

// Deleted guarda el borrado de una entidad con su número.
func (r *Repository) Deleted(ctx context.Context, userID bson.ObjectID, kind string, entityID bson.ObjectID, seq int64) error {
	_, err := r.tombstones.InsertOne(ctx, Tombstone{
		ID: bson.NewObjectID(), UserID: userID, Kind: kind, EntityID: entityID, Seq: seq, ExpiresAt: r.now().UTC().Add(Retention),
	})
	return err
}

// TombstonesAfter trae hasta limit borrados del usuario con número mayor que after, en orden.
func (r *Repository) TombstonesAfter(ctx context.Context, userID bson.ObjectID, after int64, limit int) ([]Tombstone, error) {
	cur, err := r.tombstones.Find(ctx,
		bson.D{{Key: "usuario_id", Value: userID}, {Key: "sync_seq", Value: bson.D{{Key: "$gt", Value: after}}}},
		options.Find().SetSort(bson.D{{Key: "sync_seq", Value: 1}}).SetLimit(int64(limit)))
	if err != nil {
		return nil, err
	}
	out := []Tombstone{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteByUser borra el contador y los borrados del usuario (ARCO).
func (r *Repository) DeleteByUser(ctx context.Context, userID bson.ObjectID) error {
	if _, err := r.tombstones.DeleteMany(ctx, bson.D{{Key: "usuario_id", Value: userID}}); err != nil {
		return err
	}
	_, err := r.state.DeleteOne(ctx, bson.D{{Key: "_id", Value: userID}})
	return err
}

// Privacy borra lo de sincronización al borrar la cuenta. No exporta nada: son números
// técnicos sin datos de la persona.
type Privacy struct {
	repo    *Repository
	actions *Actions
}

// NewPrivacy recibe los repositorios por constructor.
func NewPrivacy(repo *Repository, actions *Actions) *Privacy {
	return &Privacy{repo: repo, actions: actions}
}

// Name identifica al módulo en el registro.
func (p *Privacy) Name() string { return TombstonesCollection }

// PurgeHistory no borra nada: sin contador, la app perdería cambios.
func (p *Privacy) PurgeHistory(context.Context, bson.ObjectID) error { return nil }

// PurgeAccount borra el contador, los borrados y el registro de acciones sin conexión.
func (p *Privacy) PurgeAccount(ctx context.Context, userID bson.ObjectID) error {
	if err := p.actions.DeleteByUser(ctx, userID); err != nil {
		return err
	}
	return p.repo.DeleteByUser(ctx, userID)
}

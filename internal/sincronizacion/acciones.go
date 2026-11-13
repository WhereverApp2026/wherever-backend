package sincronizacion

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	platformmongo "github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
)

// ActionsCollection guarda qué acciones sin conexión ya se procesaron (nv-47), 7 días.
const ActionsCollection = "sync_acciones"

// ActionsRetention es cuánto se recuerda una acción para no aplicarla dos veces.
const ActionsRetention = 7 * 24 * time.Hour

// Estado de una acción registrada; `en_curso` mientras se aplica.
const actionInProgress = "en_curso"

// ActionRecord es una acción ya vista: su resultado y el plan que tocó, si alguno.
type ActionRecord struct {
	ID        bson.ObjectID  `bson:"_id"`
	UserID    bson.ObjectID  `bson:"usuario_id"`
	ClientID  string         `bson:"id_cliente"`
	Result    string         `bson:"resultado"`
	PlanID    *bson.ObjectID `bson:"plan_id,omitempty"`
	CreatedAt time.Time      `bson:"creada_en"`
	ExpiresAt time.Time      `bson:"expira_en"`
}

// ActionIndexes son los índices de `sync_acciones`.
func ActionIndexes() []platformmongo.IndexSet {
	return []platformmongo.IndexSet{{
		Collection: ActionsCollection,
		Models: []mongo.IndexModel{
			{Keys: bson.D{{Key: "usuario_id", Value: 1}, {Key: "id_cliente", Value: 1}}, Options: options.Index().SetName("ux_usuario_cliente").SetUnique(true)},
			{Keys: bson.D{{Key: "expira_en", Value: 1}}, Options: options.Index().SetName("ttl_accion").SetExpireAfterSeconds(0)},
		},
	}}
}

// Actions registra las acciones: reclamar una es insertar con índice único, así dos lotes
// iguales a la vez nunca aplican la misma acción dos veces.
type Actions struct {
	col *mongo.Collection
	now func() time.Time
}

// NewActions recibe la base por constructor.
func NewActions(db *mongo.Database) *Actions {
	return &Actions{col: db.Collection(ActionsCollection), now: time.Now}
}

// Claim reserva la acción. Si ya existía, devuelve el registro anterior y claimed=false.
func (a *Actions) Claim(ctx context.Context, userID bson.ObjectID, clientID string) (ActionRecord, bool, error) {
	now := a.now().UTC()
	rec := ActionRecord{ID: bson.NewObjectID(), UserID: userID, ClientID: clientID, Result: actionInProgress, CreatedAt: now, ExpiresAt: now.Add(ActionsRetention)}
	_, err := a.col.InsertOne(ctx, rec)
	if err == nil {
		return rec, true, nil
	}
	if !mongo.IsDuplicateKeyError(err) {
		return ActionRecord{}, false, err
	}
	var prev ActionRecord
	err = a.col.FindOne(ctx, bson.D{{Key: "usuario_id", Value: userID}, {Key: "id_cliente", Value: clientID}}).Decode(&prev)
	if errors.Is(err, mongo.ErrNoDocuments) { // venció justo ahora: se puede volver a intentar
		return a.Claim(ctx, userID, clientID)
	}
	return prev, false, err
}

// Finish guarda el resultado de la acción.
func (a *Actions) Finish(ctx context.Context, id bson.ObjectID, result string, planID *bson.ObjectID) error {
	set := bson.D{{Key: "resultado", Value: result}}
	if planID != nil {
		set = append(set, bson.E{Key: "plan_id", Value: *planID})
	}
	_, err := a.col.UpdateOne(ctx, bson.D{{Key: "_id", Value: id}}, bson.D{{Key: "$set", Value: set}})
	return err
}

// Release suelta una acción que no se pudo aplicar por un fallo de la base, para que el
// siguiente intento la aplique.
func (a *Actions) Release(ctx context.Context, id bson.ObjectID) error {
	_, err := a.col.DeleteOne(ctx, bson.D{{Key: "_id", Value: id}})
	return err
}

// DeleteByUser borra el registro del usuario (ARCO).
func (a *Actions) DeleteByUser(ctx context.Context, userID bson.ObjectID) error {
	_, err := a.col.DeleteMany(ctx, bson.D{{Key: "usuario_id", Value: userID}})
	return err
}

// Package grafo proyecta a Neo4j las amistades aceptadas (AMIGO_DE) y los planes guardados
// (FUE_A hacia los negocios de sus paradas) (spec nv-52). Mongo manda: un cambio deja al
// usuario en la cola `grafo_pendiente` y el Worker lo recalcula desde Mongo. Si Neo4j falla,
// el usuario sigue en la cola y se reintenta; la acción original ya respondió.
package grafo

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	platformmongo "github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
)

// Collection guarda a los usuarios cuyo grafo falta recalcular, uno por documento.
const Collection = "grafo_pendiente"

const (
	// lease: un usuario tomado no se vuelve a tomar en este plazo (si la instancia cae, otra lo retoma).
	lease = time.Minute
	// Espera entre reintentos: 5 s, 10 s, 20 s... hasta 10 min (Aura Free se pausa).
	baseDelay    = 5 * time.Second
	maxDelay     = 10 * time.Minute
	touchTimeout = 2 * time.Second
)

// pending es un documento de la cola. `marca` cambia en cada Touch: si llega un cambio
// mientras se aplica el anterior, el usuario no sale de la cola.
type pending struct {
	UserID   bson.ObjectID `bson:"_id"`
	Mark     bson.ObjectID `bson:"marca"`
	Due      time.Time     `bson:"disponible_en"`
	Attempts int           `bson:"intentos"`
}

// Indexes: la cola se toma por `disponible_en`. El único creador es scripts/indexes.
func Indexes() []platformmongo.IndexSet {
	return []platformmongo.IndexSet{{
		Collection: Collection,
		Models: []mongo.IndexModel{
			{Keys: bson.D{{Key: "disponible_en", Value: 1}}, Options: options.Index().SetName("ix_disponible_en")},
		},
	}}
}

// Queue es la cola persistida en Mongo; la comparten todas las instancias.
type Queue struct {
	col  *mongo.Collection
	log  *slog.Logger
	now  func() time.Time
	kick chan struct{}
}

// NewQueue recibe la base por constructor.
func NewQueue(db *mongo.Database, log *slog.Logger) *Queue {
	return &Queue{col: db.Collection(Collection), log: log, now: func() time.Time { return time.Now().UTC() }, kick: make(chan struct{}, 1)}
}

// Touch pide recalcular a userID. No depende del contexto de la petición: la acción ya se
// guardó. Si Mongo falla, queda en el log y lo corrige `make reconstruir-grafo`.
func (q *Queue) Touch(ctx context.Context, userID bson.ObjectID) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), touchTimeout)
	defer cancel()
	set := bson.D{{Key: "marca", Value: bson.NewObjectID()}, {Key: "disponible_en", Value: q.now()}, {Key: "intentos", Value: 0}}
	_, err := q.col.UpdateOne(ctx, bson.D{{Key: "_id", Value: userID}}, bson.D{{Key: "$set", Value: set}}, options.UpdateOne().SetUpsert(true))
	if err != nil {
		q.log.Warn("grafo: no se encoló el cambio", "evento", "grafo_encolar", "error", err.Error())
		return err
	}
	select {
	case q.kick <- struct{}{}:
	default:
	}
	return nil
}

// claim toma al siguiente usuario vencido y lo aparta por `lease`.
func (q *Queue) claim(ctx context.Context) (pending, bool, error) {
	now := q.now()
	var p pending
	err := q.col.FindOneAndUpdate(ctx,
		bson.D{{Key: "disponible_en", Value: bson.D{{Key: "$lte", Value: now}}}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "disponible_en", Value: now.Add(lease)}}}, {Key: "$inc", Value: bson.D{{Key: "intentos", Value: 1}}}},
		options.FindOneAndUpdate().SetSort(bson.D{{Key: "disponible_en", Value: 1}}).SetReturnDocument(options.After),
	).Decode(&p)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return pending{}, false, nil
	}
	return p, err == nil, err
}

// done saca al usuario de la cola, salvo que haya llegado otro cambio mientras se aplicaba.
func (q *Queue) done(ctx context.Context, p pending) error {
	_, err := q.col.DeleteOne(ctx, bson.D{{Key: "_id", Value: p.UserID}, {Key: "marca", Value: p.Mark}})
	return err
}

// retry lo deja para más tarde, con espera creciente.
func (q *Queue) retry(ctx context.Context, p pending) error {
	_, err := q.col.UpdateOne(ctx, bson.D{{Key: "_id", Value: p.UserID}, {Key: "marca", Value: p.Mark}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "disponible_en", Value: q.now().Add(backoff(p.Attempts))}}}})
	return err
}

// Pending cuenta a los usuarios en la cola.
func (q *Queue) Pending(ctx context.Context) (int64, error) {
	return q.col.CountDocuments(ctx, bson.D{})
}

// backoff es la espera tras el intento número attempts (desde 1).
func backoff(attempts int) time.Duration {
	d := baseDelay
	for i := 1; i < attempts && d < maxDelay; i++ {
		d *= 2
	}
	return min(d, maxDelay)
}

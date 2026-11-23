package chat

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	platformmongo "github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
)

// ErrConversationNotFound: no hay conversación o quien pregunta no participa.
var ErrConversationNotFound = errors.New("conversación no encontrada")

// ConversationIndexes son los índices de `conversaciones`. El único creador es scripts/indexes.
func ConversationIndexes() []platformmongo.IndexSet {
	return []platformmongo.IndexSet{{
		Collection: ConversationCollection,
		Models: []mongo.IndexModel{
			{Keys: bson.D{{Key: "par", Value: 1}}, Options: options.Index().SetName("ux_conversacion_par").SetUnique(true)},
			// Lista de conversaciones de una persona, la más reciente primero; también ARCO.
			{Keys: bson.D{{Key: "participantes", Value: 1}, {Key: "actualizada_en", Value: -1}}, Options: options.Index().SetName("ix_conversacion_participante")},
		},
	}}
}

// ConversationRepository lee y escribe `conversaciones`.
type ConversationRepository struct {
	col *mongo.Collection
}

// NewConversationRepository recibe la base por constructor.
func NewConversationRepository(db *mongo.Database) *ConversationRepository {
	return &ConversationRepository{col: db.Collection(ConversationCollection)}
}

func (r *ConversationRepository) findOne(ctx context.Context, filter bson.D) (Conversation, error) {
	var c Conversation
	err := r.col.FindOne(ctx, filter).Decode(&c)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Conversation{}, ErrConversationNotFound
	}
	return c, err
}

// Open trae la conversación del par o la crea; true = la creó. Dos aperturas al mismo tiempo
// terminan en una sola (índice único `ux_conversacion_par`).
func (r *ConversationRepository) Open(ctx context.Context, a, b bson.ObjectID, now time.Time) (Conversation, bool, error) {
	pair := ConversationPair(a, b)
	c, err := r.findOne(ctx, bson.D{{Key: "par", Value: pair}})
	if err == nil || !errors.Is(err, ErrConversationNotFound) {
		return c, false, err
	}
	c = Conversation{ID: bson.NewObjectID(), Pair: pair, Participants: []bson.ObjectID{a, b}, UpdatedAt: now, CreatedAt: now}
	if _, err := r.col.InsertOne(ctx, c); mongo.IsDuplicateKeyError(err) {
		c, err = r.findOne(ctx, bson.D{{Key: "par", Value: pair}})
		return c, false, err
	} else if err != nil {
		return Conversation{}, false, err
	}
	return c, true, nil
}

// FindFor trae la conversación solo si me participa.
func (r *ConversationRepository) FindFor(ctx context.Context, id, me bson.ObjectID) (Conversation, error) {
	return r.findOne(ctx, bson.D{{Key: "_id", Value: id}, {Key: "participantes", Value: me}})
}

// ListFor trae hasta limit conversaciones de me, la más reciente primero.
func (r *ConversationRepository) ListFor(ctx context.Context, me bson.ObjectID, limit int64) ([]Conversation, error) {
	opts := options.Find().SetSort(bson.D{{Key: "actualizada_en", Value: -1}, {Key: "_id", Value: -1}})
	if limit > 0 {
		opts.SetLimit(limit)
	}
	cur, err := r.col.Find(ctx, bson.D{{Key: "participantes", Value: me}}, opts)
	if err != nil {
		return nil, err
	}
	out := []Conversation{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// SetLast guarda la vista previa y sube la conversación en la lista.
func (r *ConversationRepository) SetLast(ctx context.Context, id bson.ObjectID, last DirectLast) error {
	_, err := r.col.UpdateOne(ctx, bson.D{{Key: "_id", Value: id}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "ultimo_mensaje", Value: last}, {Key: "actualizada_en", Value: last.At}}}})
	return err
}

// ClearLast vacía la vista previa si era el mensaje borrado.
func (r *ConversationRepository) ClearLast(ctx context.Context, id, messageID bson.ObjectID) error {
	_, err := r.col.UpdateOne(ctx,
		bson.D{{Key: "_id", Value: id}, {Key: "ultimo_mensaje.mensaje_id", Value: messageID}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "ultimo_mensaje.texto", Value: ""}, {Key: "ultimo_mensaje.eliminado", Value: true}}}})
	return err
}

// MarkRead sube `leido_hasta` de me hasta upTo; nunca lo baja ($max).
func (r *ConversationRepository) MarkRead(ctx context.Context, id, me, upTo bson.ObjectID) error {
	_, err := r.col.UpdateOne(ctx, bson.D{{Key: "_id", Value: id}, {Key: "participantes", Value: me}},
		bson.D{{Key: "$max", Value: bson.D{{Key: "leido_hasta." + me.Hex(), Value: upTo}}}})
	return err
}

// Forget marca que me borró su cuenta en sus conversaciones: quita su lectura y vacía la
// vista previa si era suya. Las conversaciones donde la otra persona ya la había borrado
// se devuelven para borrarlas del todo.
func (r *ConversationRepository) Forget(ctx context.Context, me bson.ObjectID) ([]bson.ObjectID, error) {
	rows, err := r.ListFor(ctx, me, 0)
	if err != nil {
		return nil, err
	}
	gone := []bson.ObjectID{}
	for _, c := range rows {
		if contains(c.DeletedBy, c.Other(me)) {
			gone = append(gone, c.ID)
		}
	}
	if len(gone) > 0 {
		if _, err := r.col.DeleteMany(ctx, bson.D{{Key: "_id", Value: bson.D{{Key: "$in", Value: gone}}}}); err != nil {
			return nil, err
		}
	}
	mine := bson.D{{Key: "participantes", Value: me}}
	if _, err := r.col.UpdateMany(ctx, append(mine, bson.E{Key: "ultimo_mensaje.autor_id", Value: me}), bson.D{
		{Key: "$set", Value: bson.D{{Key: "ultimo_mensaje.texto", Value: ""}, {Key: "ultimo_mensaje.eliminado", Value: true}}},
		{Key: "$unset", Value: bson.D{{Key: "ultimo_mensaje.autor_id", Value: ""}}},
	}); err != nil {
		return nil, err
	}
	_, err = r.col.UpdateMany(ctx, mine, bson.D{
		{Key: "$addToSet", Value: bson.D{{Key: "eliminada_por", Value: me}}},
		{Key: "$unset", Value: bson.D{{Key: "leido_hasta." + me.Hex(), Value: ""}}},
	})
	return gone, err
}

func contains(ids []bson.ObjectID, id bson.ObjectID) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

// UnreadCounts cuenta, por conversación, los mensajes de la otra persona (no borrados)
// después de la lectura de me. Una sola agregación sobre `ix_mensajes_conversacion`.
func (r *Repository) UnreadCounts(ctx context.Context, me bson.ObjectID, convs []Conversation) (map[bson.ObjectID]int, error) {
	out := map[bson.ObjectID]int{}
	branches := bson.A{}
	for _, c := range convs {
		if c.Last == nil {
			continue
		}
		branch := bson.D{{Key: fieldDirect, Value: c.ID}}
		if read := c.ReadBy(me); !read.IsZero() {
			branch = append(branch, bson.E{Key: "_id", Value: bson.D{{Key: "$gt", Value: read}}})
		}
		branches = append(branches, branch)
	}
	if len(branches) == 0 {
		return out, nil
	}
	cur, err := r.col.Aggregate(ctx, mongo.Pipeline{
		{{Key: "$match", Value: bson.D{
			{Key: "$or", Value: branches},
			{Key: "autor_id", Value: bson.D{{Key: "$ne", Value: me}}},
			notDeleted(),
		}}},
		{{Key: "$group", Value: bson.D{{Key: "_id", Value: "$" + fieldDirect}, {Key: "n", Value: bson.D{{Key: "$sum", Value: 1}}}}}},
	})
	if err != nil {
		return nil, err
	}
	var rows []struct {
		ID bson.ObjectID `bson:"_id"`
		N  int           `bson:"n"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ID] = row.N
	}
	return out, nil
}

// Newest es el último mensaje de la conversación (para marcar leído); cero si no hay.
func (r *Repository) Newest(ctx context.Context, conversationID bson.ObjectID) (bson.ObjectID, error) {
	var m struct {
		ID bson.ObjectID `bson:"_id"`
	}
	err := r.col.FindOne(ctx, bson.D{{Key: fieldDirect, Value: conversationID}},
		options.FindOne().SetSort(bson.D{{Key: "_id", Value: -1}}).SetProjection(bson.D{{Key: "_id", Value: 1}})).Decode(&m)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return bson.ObjectID{}, nil
	}
	return m.ID, err
}

// DeleteConversations borra los mensajes de esas conversaciones (las dos personas borraron
// su cuenta).
func (r *Repository) DeleteConversations(ctx context.Context, ids []bson.ObjectID) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := r.col.DeleteMany(ctx, bson.D{{Key: fieldDirect, Value: bson.D{{Key: "$in", Value: ids}}}})
	return err
}

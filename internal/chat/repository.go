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

// Errores del repositorio; el service los traduce.
var (
	ErrNotFound  = errors.New("mensaje no encontrado")
	ErrDuplicate = errors.New("id_cliente repetido en el grupo")
)

// Indexes son los índices de `mensajes` y `mensajes_avisos`. El único creador es scripts/indexes.
func Indexes() []platformmongo.IndexSet {
	return []platformmongo.IndexSet{
		{
			Collection: Collection,
			Models: []mongo.IndexModel{
				{Keys: bson.D{{Key: "grupo_id", Value: 1}, {Key: "_id", Value: -1}}, Options: options.Index().SetName("ix_chat")},
				{
					Keys: bson.D{{Key: "grupo_id", Value: 1}, {Key: "id_cliente", Value: 1}},
					Options: options.Index().SetName("ux_id_cliente").SetUnique(true).
						SetPartialFilterExpression(bson.D{{Key: "id_cliente", Value: bson.D{{Key: "$type", Value: "string"}}}}),
				},
				{
					Keys:    bson.D{{Key: "grupo_id", Value: 1}, {Key: "texto", Value: "text"}},
					Options: options.Index().SetName("ix_chat_texto").SetDefaultLanguage("spanish"),
				},
				// Límite de envíos por persona y borrado ARCO.
				{Keys: bson.D{{Key: "autor_id", Value: 1}, {Key: "_id", Value: -1}}, Options: options.Index().SetName("ix_autor")},
				{Keys: bson.D{{Key: "reacciones.usuario_id", Value: 1}}, Options: options.Index().SetName("ix_reaccion_usuario")},
				// Mensajes directos (nv-87): historial por conversación y reenvío por `id_cliente`.
				{Keys: bson.D{{Key: "conversacion_id", Value: 1}, {Key: "_id", Value: -1}}, Options: options.Index().SetName("ix_mensajes_conversacion")},
				{
					Keys: bson.D{{Key: "conversacion_id", Value: 1}, {Key: "id_cliente", Value: 1}},
					Options: options.Index().SetName("ux_id_cliente_conversacion").SetUnique(true).
						SetPartialFilterExpression(bson.D{
							{Key: "id_cliente", Value: bson.D{{Key: "$type", Value: "string"}}},
							{Key: "conversacion_id", Value: bson.D{{Key: "$type", Value: "objectId"}}},
						}),
				},
			},
		},
		{
			Collection: NoticeCollection,
			Models: []mongo.IndexModel{
				{Keys: bson.D{{Key: "hasta", Value: 1}}, Options: options.Index().SetName("ttl_hasta").SetExpireAfterSeconds(0)},
				{Keys: bson.D{{Key: "usuario_id", Value: 1}}, Options: options.Index().SetName("ix_usuario")},
			},
		},
	}
}

// Repository lee y escribe `mensajes`. La idempotencia por `id_cliente` la da el índice
// único; reaccionar y borrar son una sola operación cada uno.
type Repository struct {
	col     *mongo.Collection
	notices *mongo.Collection
}

// NewRepository recibe la base por constructor.
func NewRepository(db *mongo.Database) *Repository {
	return &Repository{col: db.Collection(Collection), notices: db.Collection(NoticeCollection)}
}

// Insert guarda un mensaje; ErrDuplicate si el grupo ya tiene ese `id_cliente`.
func (r *Repository) Insert(ctx context.Context, m *Message) error {
	_, err := r.col.InsertOne(ctx, m)
	if mongo.IsDuplicateKeyError(err) {
		return ErrDuplicate
	}
	return err
}

func (r *Repository) findOne(ctx context.Context, filter bson.D) (Message, error) {
	var m Message
	err := r.col.FindOne(ctx, filter).Decode(&m)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Message{}, ErrNotFound
	}
	return m, err
}

// thread es la conversación a la que pertenece un mensaje: el chat de un grupo (nv-57) o una
// conversación directa (nv-87). Los dos viven en `mensajes`; cada uno con su campo.
type thread struct {
	field string
	id    bson.ObjectID
}

// Campos de cada tipo de conversación en `mensajes`.
const (
	fieldGroup  = "grupo_id"
	fieldDirect = "conversacion_id"
)

func groupThread(id bson.ObjectID) thread  { return thread{field: fieldGroup, id: id} }
func directThread(id bson.ObjectID) thread { return thread{field: fieldDirect, id: id} }

func (t thread) filter() bson.E { return bson.E{Key: t.field, Value: t.id} }

// FindByClientID trae el mensaje de la conversación con ese `id_cliente` (reenvío).
func (r *Repository) FindByClientID(ctx context.Context, t thread, clientID string) (Message, error) {
	return r.findOne(ctx, bson.D{t.filter(), {Key: "id_cliente", Value: clientID}})
}

// FindIn trae el mensaje solo si es de esa conversación.
func (r *Repository) FindIn(ctx context.Context, t thread, id bson.ObjectID) (Message, error) {
	return r.findOne(ctx, bson.D{{Key: "_id", Value: id}, t.filter()})
}

// Page pide una página del grupo: `Before` (más viejos, del más nuevo al más viejo) o
// `After` (más nuevos, del más viejo al más nuevo); `Query` busca con `ix_chat_texto`.
type Page struct {
	Before *bson.ObjectID
	After  *bson.ObjectID
	Query  string
	Limit  int
}

// List trae hasta Limit+1 mensajes (el extra dice si hay más).
func (r *Repository) List(ctx context.Context, t thread, p Page) ([]Message, error) {
	filter := bson.D{t.filter()}
	sort := -1
	switch {
	case p.After != nil:
		filter = append(filter, bson.E{Key: "_id", Value: bson.D{{Key: "$gt", Value: *p.After}}})
		sort = 1
	case p.Before != nil:
		filter = append(filter, bson.E{Key: "_id", Value: bson.D{{Key: "$lt", Value: *p.Before}}})
	}
	if p.Query != "" {
		filter = append(filter, bson.E{Key: "$text", Value: bson.D{{Key: "$search", Value: p.Query}}})
	}
	cur, err := r.col.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "_id", Value: sort}}).SetLimit(int64(p.Limit+1)))
	if err != nil {
		return nil, err
	}
	out := []Message{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CountSince cuenta los mensajes de authorID desde `since` en un tipo de conversación (el
// campo de `thread`: grupos o directos), para el límite de envíos sin Redis.
func (r *Repository) CountSince(ctx context.Context, field string, authorID bson.ObjectID, since time.Time) (int64, error) {
	return r.col.CountDocuments(ctx, bson.D{
		{Key: "autor_id", Value: authorID},
		{Key: "_id", Value: bson.D{{Key: "$gte", Value: bson.NewObjectIDFromTimestamp(since)}}},
		{Key: field, Value: bson.D{{Key: "$exists", Value: true}}},
	})
}

func notDeleted() bson.E {
	return bson.E{Key: "eliminado_en", Value: bson.D{{Key: "$exists", Value: false}}}
}

// SetReaction pone (o cambia) la reacción de userID en una sola operación; con emoji vacío
// la quita. false = el mensaje no existe en el grupo, es del sistema o está borrado.
func (r *Repository) SetReaction(ctx context.Context, t thread, id, userID bson.ObjectID, emoji string) (bool, error) {
	others := bson.D{{Key: "$filter", Value: bson.D{
		{Key: "input", Value: bson.D{{Key: "$ifNull", Value: bson.A{"$reacciones", bson.A{}}}}},
		{Key: "cond", Value: bson.D{{Key: "$ne", Value: bson.A{"$$this.usuario_id", userID}}}},
	}}}
	value := any(others)
	if emoji != "" {
		mine := bson.A{bson.D{{Key: "usuario_id", Value: userID}, {Key: "emoji", Value: emoji}}}
		value = bson.D{{Key: "$concatArrays", Value: bson.A{others, mine}}}
	}
	res, err := r.col.UpdateOne(ctx,
		bson.D{{Key: "_id", Value: id}, t.filter(), {Key: "tipo", Value: KindText}, notDeleted()},
		mongo.Pipeline{{{Key: "$set", Value: bson.D{{Key: "reacciones", Value: value}}}}})
	if err != nil {
		return false, err
	}
	return res.MatchedCount > 0, nil
}

func blanked(now time.Time) bson.D {
	return bson.D{{Key: "texto", Value: ""}, {Key: "reacciones", Value: bson.A{}}, {Key: "adjunto", Value: nil}, {Key: "eliminado_en", Value: now}}
}

// SoftDelete vacía el mensaje de authorID. false = no es suyo o no está en el grupo; borrar
// uno ya borrado cuenta como hecho.
func (r *Repository) SoftDelete(ctx context.Context, t thread, id, authorID bson.ObjectID, now time.Time) (bool, error) {
	filter := bson.D{{Key: "_id", Value: id}, t.filter(), {Key: "autor_id", Value: authorID}}
	res, err := r.col.UpdateOne(ctx, append(filter, notDeleted()), bson.D{{Key: "$set", Value: blanked(now)}})
	if err != nil {
		return false, err
	}
	if res.MatchedCount > 0 {
		return true, nil
	}
	n, err := r.col.CountDocuments(ctx, filter)
	return n > 0, err
}

// ClaimNotice reserva el push de chat de userID en el grupo por NoticeEvery. false = ya se
// avisó dentro del lapso (desde esta petición, otra o la otra instancia).
func (r *Repository) ClaimNotice(ctx context.Context, groupID, userID bson.ObjectID, now time.Time) (bool, error) {
	key := groupID.Hex() + ":" + userID.Hex()
	_, err := r.notices.UpdateOne(ctx,
		bson.D{{Key: "_id", Value: key}, {Key: "hasta", Value: bson.D{{Key: "$lte", Value: now}}}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "hasta", Value: now.Add(NoticeEvery)}, {Key: "usuario_id", Value: userID}}}},
		options.UpdateOne().SetUpsert(true))
	if mongo.IsDuplicateKeyError(err) {
		return false, nil
	}
	return err == nil, err
}

// ListByAuthor trae los mensajes no borrados de userID en un tipo de conversación
// (exportación ARCO; `field` es el de `thread`).
func (r *Repository) ListByAuthor(ctx context.Context, field string, userID bson.ObjectID) ([]Message, error) {
	return r.find(ctx, bson.D{{Key: "autor_id", Value: userID}, notDeleted(), exists(field)})
}

// ListReactedBy trae los mensajes donde userID reaccionó (exportación ARCO).
func (r *Repository) ListReactedBy(ctx context.Context, field string, userID bson.ObjectID) ([]Message, error) {
	return r.find(ctx, bson.D{{Key: "reacciones.usuario_id", Value: userID}, exists(field)})
}

func exists(field string) bson.E {
	return bson.E{Key: field, Value: bson.D{{Key: "$exists", Value: true}}}
}

func (r *Repository) find(ctx context.Context, filter bson.D) ([]Message, error) {
	cur, err := r.col.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}))
	if err != nil {
		return nil, err
	}
	out := []Message{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// PurgeAuthor vacía los mensajes de userID y les quita el autor, quita sus reacciones de los
// demás y borra sus candados de push. La conversación conserva su forma ("Mensaje eliminado").
func (r *Repository) PurgeAuthor(ctx context.Context, userID bson.ObjectID, now time.Time) error {
	if _, err := r.col.UpdateMany(ctx, bson.D{{Key: "autor_id", Value: userID}}, bson.D{
		{Key: "$set", Value: blanked(now)},
		{Key: "$unset", Value: bson.D{{Key: "autor_id", Value: ""}}},
	}); err != nil {
		return err
	}
	if _, err := r.col.UpdateMany(ctx, bson.D{{Key: "reacciones.usuario_id", Value: userID}},
		bson.D{{Key: "$pull", Value: bson.D{{Key: "reacciones", Value: bson.D{{Key: "usuario_id", Value: userID}}}}}}); err != nil {
		return err
	}
	_, err := r.notices.DeleteMany(ctx, bson.D{{Key: "usuario_id", Value: userID}})
	return err
}

// DeleteGroups borra los mensajes de esos grupos (el organizador borró su cuenta y el grupo
// desaparece con ella).
func (r *Repository) DeleteGroups(ctx context.Context, groupIDs []bson.ObjectID) error {
	if len(groupIDs) == 0 {
		return nil
	}
	_, err := r.col.DeleteMany(ctx, bson.D{{Key: "grupo_id", Value: bson.D{{Key: "$in", Value: groupIDs}}}})
	return err
}

// Package amistades envía, acepta, rechaza, elimina y bloquea amistades (spec nv-36, hoja
// J2). Cada par de personas tiene a lo sumo un documento (`par` único): dos solicitudes
// cruzadas al mismo tiempo terminan en una sola amistad.
package amistades

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	platformmongo "github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
)

// Collection guarda las relaciones entre personas.
const Collection = "amistades"

// Estados de una relación.
const (
	StatusPending  = "pendiente"
	StatusAccepted = "aceptada"
	StatusBlocked  = "bloqueada"
)

// MaxList es el tope de relaciones por lista (amigos, recibidas o enviadas).
const MaxList = 500

// ErrDuplicate: ya hay un documento para ese par (otra solicitud ganó la carrera).
var ErrDuplicate = errors.New("amistades: el par ya existe")

// Friendship es un documento de `amistades`. En `bloqueada`, `de` es quien bloqueó.
type Friendship struct {
	ID     bson.ObjectID `bson:"_id"`
	From   bson.ObjectID `bson:"de"`
	To     bson.ObjectID `bson:"para"`
	Status string        `bson:"estado"`
	Pair   string        `bson:"par"`
	// MutedBy son quienes silenciaron al otro (0 a 2, nv-70); cada quien solo ve su marca.
	MutedBy   []bson.ObjectID `bson:"silenciado_por,omitempty"`
	CreatedAt time.Time       `bson:"creada_en"`
	UpdatedAt time.Time       `bson:"actualizada_en"`
}

// Other es la otra persona de la relación vista por me.
func (f Friendship) Other(me bson.ObjectID) bson.ObjectID {
	if f.From == me {
		return f.To
	}
	return f.From
}

// PairKey son los dos ids ordenados: el mismo valor sin importar quién envió.
func PairKey(a, b bson.ObjectID) string {
	x, y := a.Hex(), b.Hex()
	if x > y {
		x, y = y, x
	}
	return x + ":" + y
}

// Indexes: `par` único y las listas por persona y estado. El único creador es scripts/indexes.
func Indexes() []platformmongo.IndexSet {
	return []platformmongo.IndexSet{{
		Collection: Collection,
		Models: []mongo.IndexModel{
			{Keys: bson.D{{Key: "par", Value: 1}}, Options: options.Index().SetName("ux_par").SetUnique(true)},
			{Keys: bson.D{{Key: "de", Value: 1}, {Key: "estado", Value: 1}, {Key: "actualizada_en", Value: -1}}, Options: options.Index().SetName("ix_de_estado")},
			{Keys: bson.D{{Key: "para", Value: 1}, {Key: "estado", Value: 1}, {Key: "actualizada_en", Value: -1}}, Options: options.Index().SetName("ix_para_estado")},
		},
	}}
}

// Repository accede a `amistades`. Toda operación sobre un documento lleva al participante
// en el filtro: nadie toca una relación ajena.
type Repository struct {
	col *mongo.Collection
}

// NewRepository recibe la base por constructor.
func NewRepository(db *mongo.Database) *Repository {
	return &Repository{col: db.Collection(Collection)}
}

func participant(me bson.ObjectID) bson.E {
	return bson.E{Key: "$or", Value: bson.A{bson.D{{Key: "de", Value: me}}, bson.D{{Key: "para", Value: me}}}}
}

func (r *Repository) findOne(ctx context.Context, filter bson.D) (Friendship, bool, error) {
	var f Friendship
	err := r.col.FindOne(ctx, filter).Decode(&f)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Friendship{}, false, nil
	}
	if err != nil {
		return Friendship{}, false, err
	}
	return f, true, nil
}

// FindByPair busca la relación entre a y b, en cualquier sentido.
func (r *Repository) FindByPair(ctx context.Context, a, b bson.ObjectID) (Friendship, bool, error) {
	return r.findOne(ctx, bson.D{{Key: "par", Value: PairKey(a, b)}})
}

// FindForParticipant busca la relación id solo si me participa en ella.
func (r *Repository) FindForParticipant(ctx context.Context, id, me bson.ObjectID) (Friendship, bool, error) {
	return r.findOne(ctx, bson.D{{Key: "_id", Value: id}, participant(me)})
}

// Insert guarda una solicitud nueva; ErrDuplicate si el par ya existe.
func (r *Repository) Insert(ctx context.Context, f *Friendship) error {
	_, err := r.col.InsertOne(ctx, f)
	if mongo.IsDuplicateKeyError(err) {
		return ErrDuplicate
	}
	return err
}

// update aplica set al documento que cumple filter y devuelve cómo quedó; false si ninguno
// cumplía el filtro.
func (r *Repository) update(ctx context.Context, filter, set bson.D) (Friendship, bool, error) {
	var f Friendship
	err := r.col.FindOneAndUpdate(ctx, filter, bson.D{{Key: "$set", Value: set}},
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&f)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Friendship{}, false, nil
	}
	if err != nil {
		return Friendship{}, false, err
	}
	return f, true, nil
}

// Accept pasa a `aceptada` la solicitud pendiente id que recibió receiver.
func (r *Repository) Accept(ctx context.Context, id, receiver bson.ObjectID, now time.Time) (Friendship, bool, error) {
	filter := bson.D{{Key: "_id", Value: id}, {Key: "para", Value: receiver}, {Key: "estado", Value: StatusPending}}
	return r.update(ctx, filter, bson.D{{Key: "estado", Value: StatusAccepted}, {Key: "actualizada_en", Value: now}})
}

// Block deja la relación como `bloqueada` por blocker, si sigue en el estado prev.
func (r *Repository) Block(ctx context.Context, f Friendship, blocker bson.ObjectID, now time.Time) (Friendship, bool, error) {
	filter := bson.D{{Key: "_id", Value: f.ID}, {Key: "estado", Value: f.Status}}
	set := bson.D{
		{Key: "de", Value: blocker}, {Key: "para", Value: f.Other(blocker)},
		{Key: "estado", Value: StatusBlocked}, {Key: "actualizada_en", Value: now},
		// Bloquear borra el silencio (nv-70): ya no hay amistad que silenciar.
		{Key: "silenciado_por", Value: bson.A{}},
	}
	return r.update(ctx, filter, set)
}

// SetMuted pone o quita la marca de me sobre una amistad aceptada en la que participa
// (`$addToSet`/`$pull`: repetirlo deja lo mismo). false si no participa, no existe o no está
// aceptada.
func (r *Repository) SetMuted(ctx context.Context, id, me bson.ObjectID, muted bool) (bool, error) {
	op := "$pull"
	if muted {
		op = "$addToSet"
	}
	res, err := r.col.UpdateOne(ctx, bson.D{{Key: "_id", Value: id}, participant(me), {Key: "estado", Value: StatusAccepted}},
		bson.D{{Key: op, Value: bson.D{{Key: "silenciado_por", Value: me}}}})
	if err != nil {
		return false, err
	}
	return res.MatchedCount == 1, nil
}

// Muted dice si me silenció a other (perfil de persona, nv-70).
func (r *Repository) Muted(ctx context.Context, me, other bson.ObjectID) (bool, error) {
	f, ok, err := r.FindByPair(ctx, me, other)
	if err != nil || !ok {
		return false, err
	}
	return f.MutedByUser(me), nil
}

// MutedByUser dice si me silenció a la otra persona.
func (f Friendship) MutedByUser(me bson.ObjectID) bool {
	for _, id := range f.MutedBy {
		if id == me {
			return true
		}
	}
	return false
}

// Reject borra la solicitud pendiente id que recibió receiver: quien envió no se entera.
func (r *Repository) Reject(ctx context.Context, id, receiver bson.ObjectID) (bool, error) {
	return r.deleteOne(ctx, bson.D{{Key: "_id", Value: id}, {Key: "para", Value: receiver}, {Key: "estado", Value: StatusPending}})
}

// Remove borra una amistad o una solicitud (enviada o recibida) de me; nunca un bloqueo.
func (r *Repository) Remove(ctx context.Context, id, me bson.ObjectID) (bool, error) {
	return r.deleteOne(ctx, bson.D{
		{Key: "_id", Value: id}, participant(me),
		{Key: "estado", Value: bson.D{{Key: "$in", Value: bson.A{StatusPending, StatusAccepted}}}},
	})
}

func (r *Repository) deleteOne(ctx context.Context, filter bson.D) (bool, error) {
	res, err := r.col.DeleteOne(ctx, filter)
	if err != nil {
		return false, err
	}
	return res.DeletedCount > 0, nil
}

// Kind es una de las listas de GET /amistades.
type Kind string

// Listas.
const (
	KindFriends  Kind = "amigos"
	KindReceived Kind = "recibidas"
	KindSent     Kind = "enviadas"
)

func listFilter(me bson.ObjectID, kind Kind) bson.D {
	switch kind {
	case KindReceived:
		return bson.D{{Key: "para", Value: me}, {Key: "estado", Value: StatusPending}}
	case KindSent:
		return bson.D{{Key: "de", Value: me}, {Key: "estado", Value: StatusPending}}
	default:
		return bson.D{participant(me), {Key: "estado", Value: StatusAccepted}}
	}
}

// List devuelve la lista kind de me, de la más reciente a la más antigua.
func (r *Repository) List(ctx context.Context, me bson.ObjectID, kind Kind) ([]Friendship, error) {
	return r.find(ctx, listFilter(me, kind), MaxList)
}

// CountReceived cuenta las solicitudes pendientes que recibió me.
func (r *Repository) CountReceived(ctx context.Context, me bson.ObjectID) (int64, error) {
	return r.col.CountDocuments(ctx, listFilter(me, KindReceived))
}

// BlockedWith son las personas con relación `bloqueada` con me, en cualquier sentido.
// Cumple personas.Blocks.
func (r *Repository) BlockedWith(ctx context.Context, me bson.ObjectID) ([]bson.ObjectID, error) {
	rows, err := r.find(ctx, bson.D{participant(me), {Key: "estado", Value: StatusBlocked}}, 0)
	if err != nil {
		return nil, err
	}
	out := make([]bson.ObjectID, 0, len(rows))
	for _, f := range rows {
		out = append(out, f.Other(me))
	}
	return out, nil
}

// Relation es el estado de la relación entre me y other ("" si no hay) y si la creó me (en
// `bloqueada`, si me bloqueó). Una sola consulta por `par` cubre los dos sentidos (nv-64).
func (r *Repository) Relation(ctx context.Context, me, other bson.ObjectID) (status string, mine bool, err error) {
	f, found, err := r.FindByPair(ctx, me, other)
	if err != nil || !found {
		return "", false, err
	}
	return f.Status, f.From == me, nil
}

// ListByUser devuelve todas las relaciones de me (exportación ARCO).
func (r *Repository) ListByUser(ctx context.Context, me bson.ObjectID) ([]Friendship, error) {
	return r.find(ctx, bson.D{participant(me)}, 0)
}

// DeleteByUser borra todas las relaciones de me (borrado de cuenta, ARCO).
func (r *Repository) DeleteByUser(ctx context.Context, me bson.ObjectID) error {
	_, err := r.col.DeleteMany(ctx, bson.D{participant(me)})
	return err
}

func (r *Repository) find(ctx context.Context, filter bson.D, limit int64) ([]Friendship, error) {
	opts := options.Find().SetSort(bson.D{{Key: "actualizada_en", Value: -1}, {Key: "_id", Value: -1}})
	if limit > 0 {
		opts.SetLimit(limit)
	}
	cur, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	out := []Friendship{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Blocked dice si a y b tienen un bloqueo, en cualquier sentido (nv-68: un plan público no
// se muestra ni se copia entre personas bloqueadas).
func (r *Repository) Blocked(ctx context.Context, a, b bson.ObjectID) (bool, error) {
	f, ok, err := r.FindByPair(ctx, a, b)
	if err != nil || !ok {
		return false, err
	}
	return f.Status == StatusBlocked, nil
}

// RelationsWith son las relaciones de me con cada persona de others en una consulta por `par`
// (nv-83); quien no tiene relación no aparece.
func (r *Repository) RelationsWith(ctx context.Context, me bson.ObjectID, others []bson.ObjectID) (map[bson.ObjectID]Friendship, error) {
	out := map[bson.ObjectID]Friendship{}
	if len(others) == 0 {
		return out, nil
	}
	pairs := make(bson.A, 0, len(others))
	for _, o := range others {
		pairs = append(pairs, PairKey(me, o))
	}
	rows, err := r.find(ctx, bson.D{{Key: "par", Value: bson.D{{Key: "$in", Value: pairs}}}}, 0)
	if err != nil {
		return nil, err
	}
	for _, f := range rows {
		out[f.Other(me)] = f
	}
	return out, nil
}

// CommonWith cuenta, para cada persona de others, cuántas amistades aceptadas tiene con las
// personas de mine (los amigos de quien pregunta, hasta MaxList), en una sola consulta (nv-83).
func (r *Repository) CommonWith(ctx context.Context, mine, others []bson.ObjectID) (map[bson.ObjectID]int, error) {
	if len(mine) == 0 || len(others) == 0 {
		return map[bson.ObjectID]int{}, nil
	}
	in := func(ids []bson.ObjectID) bson.D { return bson.D{{Key: "$in", Value: ids}} }
	filter := bson.D{{Key: "estado", Value: StatusAccepted}, {Key: "$or", Value: bson.A{
		bson.D{{Key: "de", Value: in(others)}, {Key: "para", Value: in(mine)}},
		bson.D{{Key: "para", Value: in(others)}, {Key: "de", Value: in(mine)}},
	}}}
	rows, err := r.find(ctx, filter, 0)
	if err != nil {
		return nil, err
	}
	return countCommon(rows, mine, others), nil
}

// countCommon suma a cada persona de others sus amistades con alguien de mine.
func countCommon(rows []Friendship, mine, others []bson.ObjectID) map[bson.ObjectID]int {
	out := map[bson.ObjectID]int{}
	set, friends := idSet(others), idSet(mine)
	for _, f := range rows {
		if set[f.From] && friends[f.To] {
			out[f.From]++
		}
		if set[f.To] && friends[f.From] {
			out[f.To]++
		}
	}
	return out
}

func idSet(ids []bson.ObjectID) map[bson.ObjectID]bool {
	out := make(map[bson.ObjectID]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out
}

// FriendIDs son las amistades aceptadas de me (hasta MaxList): el filtro «De amigos» y la
// tarjeta de Explorar (nv-85). Cumple negocios.FriendIDs.
func (r *Repository) FriendIDs(ctx context.Context, me bson.ObjectID) ([]bson.ObjectID, error) {
	rows, err := r.List(ctx, me, KindFriends)
	if err != nil {
		return nil, err
	}
	out := make([]bson.ObjectID, 0, len(rows))
	for _, f := range rows {
		out = append(out, f.Other(me))
	}
	return out, nil
}

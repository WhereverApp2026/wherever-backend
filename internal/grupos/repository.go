package grupos

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
	ErrNotFound  = errors.New("grupo no encontrado")
	ErrDuplicate = errors.New("el plan ya tiene grupo")
	ErrFull      = errors.New("el grupo está lleno")
)

// Indexes son los índices de `grupos`. El único creador es scripts/indexes.
func Indexes() []platformmongo.IndexSet {
	return []platformmongo.IndexSet{{
		Collection: Collection,
		Models: []mongo.IndexModel{
			{Keys: bson.D{{Key: "plan_id", Value: 1}}, Options: options.Index().SetName("ux_plan").SetUnique(true)},
			{
				Keys:    bson.D{{Key: "integrantes.usuario_id", Value: 1}, {Key: "actualizado_en", Value: -1}, {Key: "_id", Value: -1}},
				Options: options.Index().SetName("ix_integrante_actualizado"),
			},
			{
				Keys: bson.D{{Key: "codigo_hash", Value: 1}},
				Options: options.Index().SetName("ux_codigo_hash").SetUnique(true).
					SetPartialFilterExpression(bson.D{{Key: "codigo_hash", Value: bson.D{{Key: "$exists", Value: true}}}}),
			},
		},
	}}
}

// Repository lee y escribe `grupos`. Todo cambio de integrantes y de `ocupados` va en una
// sola operación con su condición, así el tope de 12 no depende de leer antes.
type Repository struct {
	col *mongo.Collection
}

// NewRepository recibe la base por constructor.
func NewRepository(db *mongo.Database) *Repository {
	return &Repository{col: db.Collection(Collection)}
}

func elem(match bson.D) bson.E {
	return bson.E{Key: "integrantes", Value: bson.D{{Key: "$elemMatch", Value: match}}}
}

func hasRoom() bson.E {
	return bson.E{Key: "ocupados", Value: bson.D{{Key: "$lt", Value: MaxMembers}}}
}

// Create guarda un grupo nuevo; ErrDuplicate si el plan ya tiene uno (ux_plan).
func (r *Repository) Create(ctx context.Context, g *Group) error {
	_, err := r.col.InsertOne(ctx, g)
	if mongo.IsDuplicateKeyError(err) {
		return ErrDuplicate
	}
	return err
}

func (r *Repository) findOne(ctx context.Context, filter bson.D) (Group, error) {
	var g Group
	err := r.col.FindOne(ctx, filter).Decode(&g)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Group{}, ErrNotFound
	}
	return g, err
}

// FindByID trae el grupo.
func (r *Repository) FindByID(ctx context.Context, id bson.ObjectID) (Group, error) {
	return r.findOne(ctx, bson.D{{Key: "_id", Value: id}})
}

// FindByPlan trae el grupo del plan.
func (r *Repository) FindByPlan(ctx context.Context, planID bson.ObjectID) (Group, error) {
	return r.findOne(ctx, bson.D{{Key: "plan_id", Value: planID}})
}

func (r *Repository) updated(ctx context.Context, filter, update bson.D) (bool, error) {
	res, err := r.col.UpdateOne(ctx, filter, update)
	if err != nil {
		return false, err
	}
	return res.MatchedCount == 1, nil
}

// Invite agrega a userID como invitado, o vuelve a invitar a quien había declinado, solo si
// queda cupo. Si ya estaba invitado o confirmado no cambia nada. ErrFull si no cupo.
func (r *Repository) Invite(ctx context.Context, groupID, organizer, userID bson.ObjectID, now time.Time) error {
	group := bson.E{Key: "_id", Value: groupID}
	byOrganizer := bson.E{Key: "organizador_id", Value: organizer}
	again := bson.D{group, byOrganizer, hasRoom(), elem(bson.D{{Key: "usuario_id", Value: userID}, {Key: "estado", Value: StatusDeclined}})}
	ok, err := r.updated(ctx, again, bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "integrantes.$.estado", Value: StatusInvited}, {Key: "integrantes.$.desde", Value: now},
			{Key: "integrantes.$.invitado_por", Value: organizer}, {Key: "actualizado_en", Value: now},
		}},
		{Key: "$inc", Value: bson.D{{Key: "ocupados", Value: 1}}},
	})
	if err != nil || ok {
		return err
	}
	member := Member{UserID: userID, Role: RoleMember, Status: StatusInvited, InvitedBy: &organizer, Since: now}
	fresh := bson.D{group, byOrganizer, hasRoom(), {Key: "integrantes.usuario_id", Value: bson.D{{Key: "$ne", Value: userID}}}}
	ok, err = r.updated(ctx, fresh, bson.D{
		{Key: "$push", Value: bson.D{{Key: "integrantes", Value: member}}},
		{Key: "$inc", Value: bson.D{{Key: "ocupados", Value: 1}}},
		{Key: "$set", Value: bson.D{{Key: "actualizado_en", Value: now}}},
	})
	if err != nil || ok {
		return err
	}
	// Ninguna aplicó: el grupo no existe, ya estaba dentro o no hay cupo.
	g, err := r.findOne(ctx, bson.D{group, byOrganizer})
	if err != nil {
		return err
	}
	if m, in := g.Member(userID); in && m.Status != StatusDeclined {
		return nil
	}
	return ErrFull
}

// SetStatus cambia el estado de un integrante (no del organizador) de `from` a `to`. Salir
// de `declinado` vuelve a ocupar cupo y exige que quede; entrar a `declinado` lo libera.
// Devuelve false si el integrante ya no estaba en `from` (otra petición ganó).
func (r *Repository) SetStatus(ctx context.Context, groupID, userID bson.ObjectID, from, to string, now time.Time) (bool, error) {
	filter := bson.D{{Key: "_id", Value: groupID}, elem(bson.D{{Key: "usuario_id", Value: userID}, {Key: "rol", Value: RoleMember}, {Key: "estado", Value: from}})}
	inc := 0
	switch {
	case from == StatusDeclined && to != StatusDeclined:
		inc = 1
		filter = append(filter, hasRoom())
	case from != StatusDeclined && to == StatusDeclined:
		inc = -1
	}
	update := bson.D{{Key: "$set", Value: bson.D{{Key: "integrantes.$.estado", Value: to}, {Key: "actualizado_en", Value: now}}}}
	if inc != 0 {
		update = append(update, bson.E{Key: "$inc", Value: bson.D{{Key: "ocupados", Value: inc}}})
	}
	return r.updated(ctx, filter, update)
}

// RemoveMember saca a un integrante (nunca al organizador) y libera su cupo si lo ocupaba.
// Devuelve false si no estaba.
func (r *Repository) RemoveMember(ctx context.Context, groupID, userID bson.ObjectID, now time.Time) (bool, error) {
	pull := bson.E{Key: "$pull", Value: bson.D{{Key: "integrantes", Value: bson.D{{Key: "usuario_id", Value: userID}}}}}
	stamp := bson.E{Key: "$set", Value: bson.D{{Key: "actualizado_en", Value: now}}}
	occupying := bson.D{{Key: "_id", Value: groupID}, elem(bson.D{{Key: "usuario_id", Value: userID}, {Key: "rol", Value: RoleMember}, {Key: "estado", Value: bson.D{{Key: "$ne", Value: StatusDeclined}}}})}
	ok, err := r.updated(ctx, occupying, bson.D{pull, stamp, {Key: "$inc", Value: bson.D{{Key: "ocupados", Value: -1}}}})
	if err != nil || ok {
		return ok, err
	}
	declined := bson.D{{Key: "_id", Value: groupID}, elem(bson.D{{Key: "usuario_id", Value: userID}, {Key: "rol", Value: RoleMember}, {Key: "estado", Value: StatusDeclined}})}
	return r.updated(ctx, declined, bson.D{pull, stamp})
}

// Position es el último grupo entregado de una página.
type Position struct {
	UpdatedAt time.Time
	ID        bson.ObjectID
}

// ListForMember trae hasta limit+1 grupos donde userID es integrante, del más reciente al más
// viejo. Usa ix_integrante_actualizado.
func (r *Repository) ListForMember(ctx context.Context, userID bson.ObjectID, after *Position, limit int) ([]Group, error) {
	filter := bson.D{{Key: "integrantes.usuario_id", Value: userID}}
	if after != nil {
		filter = append(filter, bson.E{Key: "$or", Value: bson.A{
			bson.D{{Key: "actualizado_en", Value: bson.D{{Key: "$lt", Value: after.UpdatedAt}}}},
			bson.D{{Key: "actualizado_en", Value: after.UpdatedAt}, {Key: "_id", Value: bson.D{{Key: "$lt", Value: after.ID}}}},
		}})
	}
	opts := options.Find().SetSort(bson.D{{Key: "actualizado_en", Value: -1}, {Key: "_id", Value: -1}}).SetLimit(int64(limit + 1))
	return r.find(ctx, filter, opts)
}

func (r *Repository) find(ctx context.Context, filter bson.D, opts *options.FindOptionsBuilder) ([]Group, error) {
	cur, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	out := []Group{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CanRead dice si userID está invitado o confirmado en el grupo del plan; lo usa
// planes.Service para que los integrantes lean el plan (nv-37).
func (r *Repository) CanRead(ctx context.Context, planID, userID bson.ObjectID) (bool, error) {
	n, err := r.col.CountDocuments(ctx, bson.D{
		{Key: "plan_id", Value: planID},
		elem(bson.D{{Key: "usuario_id", Value: userID}, {Key: "estado", Value: bson.D{{Key: "$in", Value: bson.A{StatusInvited, StatusConfirmed}}}}}),
	})
	return n > 0, err
}

// ListByUser trae todos los grupos del usuario, para exportarlos.
func (r *Repository) ListByUser(ctx context.Context, userID bson.ObjectID) ([]Group, error) {
	return r.find(ctx, bson.D{{Key: "integrantes.usuario_id", Value: userID}}, options.Find().SetSort(bson.D{{Key: "creado_en", Value: 1}}))
}

// DeleteByUser borra los grupos que organiza userID y lo saca de los demás, liberando su
// cupo donde lo ocupaba (borrado definitivo de la cuenta).
func (r *Repository) DeleteByUser(ctx context.Context, userID bson.ObjectID) error {
	if _, err := r.col.DeleteMany(ctx, bson.D{{Key: "organizador_id", Value: userID}}); err != nil {
		return err
	}
	pull := bson.E{Key: "$pull", Value: bson.D{{Key: "integrantes", Value: bson.D{{Key: "usuario_id", Value: userID}}}}}
	occupying := bson.D{elem(bson.D{{Key: "usuario_id", Value: userID}, {Key: "estado", Value: bson.D{{Key: "$ne", Value: StatusDeclined}}}})}
	if _, err := r.col.UpdateMany(ctx, occupying, bson.D{pull, {Key: "$inc", Value: bson.D{{Key: "ocupados", Value: -1}}}}); err != nil {
		return err
	}
	_, err := r.col.UpdateMany(ctx, bson.D{{Key: "integrantes.usuario_id", Value: userID}}, bson.D{pull})
	return err
}

// SetCode guarda el hash del código del enlace y su vencimiento; reemplaza al anterior, que
// deja de servir (nv-38).
func (r *Repository) SetCode(ctx context.Context, groupID, organizer bson.ObjectID, hash string, expires, now time.Time) error {
	ok, err := r.updated(ctx, bson.D{{Key: "_id", Value: groupID}, {Key: "organizador_id", Value: organizer}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "codigo_hash", Value: hash}, {Key: "codigo_vence", Value: expires}, {Key: "actualizado_en", Value: now}}}})
	if err == nil && !ok {
		return ErrNotFound
	}
	return err
}

// ClearCode revoca el enlace del grupo. Devuelve false si no tenía.
func (r *Repository) ClearCode(ctx context.Context, groupID, organizer bson.ObjectID, now time.Time) (bool, error) {
	return r.updated(ctx,
		bson.D{{Key: "_id", Value: groupID}, {Key: "organizador_id", Value: organizer}, {Key: "codigo_hash", Value: bson.D{{Key: "$exists", Value: true}}}},
		bson.D{{Key: "$unset", Value: bson.D{{Key: "codigo_hash", Value: ""}, {Key: "codigo_vence", Value: ""}}}, {Key: "$set", Value: bson.D{{Key: "actualizado_en", Value: now}}}})
}

// FindByCode trae el grupo con ese hash de código (ux_codigo_hash).
func (r *Repository) FindByCode(ctx context.Context, hash string) (Group, error) {
	return r.findOne(ctx, bson.D{{Key: "codigo_hash", Value: hash}})
}

// Join deja a userID confirmado por el enlace: entra nuevo, pasa de invitado a confirmado o
// vuelve después de declinar. Entrar nuevo o volver exige cupo en la misma operación
// (ErrFull). Si ya estaba confirmado no cambia nada.
func (r *Repository) Join(ctx context.Context, groupID, userID bson.ObjectID, now time.Time) error {
	g, err := r.FindByID(ctx, groupID)
	if err != nil {
		return err
	}
	if m, in := g.Member(userID); in {
		if m.Status == StatusConfirmed {
			return nil
		}
		ok, err := r.SetStatus(ctx, groupID, userID, m.Status, StatusConfirmed, now)
		return joinResult(ok, err)
	}
	member := Member{UserID: userID, Role: RoleMember, Status: StatusConfirmed, Since: now}
	ok, err := r.updated(ctx, bson.D{{Key: "_id", Value: groupID}, hasRoom(), {Key: "integrantes.usuario_id", Value: bson.D{{Key: "$ne", Value: userID}}}}, bson.D{
		{Key: "$push", Value: bson.D{{Key: "integrantes", Value: member}}},
		{Key: "$inc", Value: bson.D{{Key: "ocupados", Value: 1}}},
		{Key: "$set", Value: bson.D{{Key: "actualizado_en", Value: now}}},
	})
	return joinResult(ok, err)
}

// joinResult: sin coincidencia es falta de cupo (o una carrera con la misma persona, que el
// service resuelve releyendo).
func joinResult(ok bool, err error) error {
	if err != nil || ok {
		return err
	}
	return ErrFull
}

// SetLastMessage guarda el último mensaje del chat solo si es más nuevo que el guardado: dos
// envíos en carrera dejan el más reciente (nv-57). No cambia `actualizado_en`.
func (r *Repository) SetLastMessage(ctx context.Context, groupID bson.ObjectID, last LastMessage) error {
	_, err := r.col.UpdateOne(ctx, bson.D{
		{Key: "_id", Value: groupID},
		{Key: "$or", Value: bson.A{
			bson.D{{Key: "ultimo_mensaje", Value: bson.D{{Key: "$exists", Value: false}}}},
			bson.D{{Key: "ultimo_mensaje.mensaje_id", Value: bson.D{{Key: "$lt", Value: last.MessageID}}}},
		}},
	}, bson.D{{Key: "$set", Value: bson.D{{Key: "ultimo_mensaje", Value: last}}}})
	return err
}

// ClearLastMessage quita `ultimo_mensaje` si era ese mensaje (se borró).
func (r *Repository) ClearLastMessage(ctx context.Context, groupID, messageID bson.ObjectID) error {
	_, err := r.col.UpdateOne(ctx, bson.D{{Key: "_id", Value: groupID}, {Key: "ultimo_mensaje.mensaje_id", Value: messageID}},
		bson.D{{Key: "$unset", Value: bson.D{{Key: "ultimo_mensaje", Value: ""}}}})
	return err
}

// ClearLastMessagesBy quita `ultimo_mensaje` donde el autor es userID (borrado de la cuenta).
func (r *Repository) ClearLastMessagesBy(ctx context.Context, userID bson.ObjectID) error {
	_, err := r.col.UpdateMany(ctx, bson.D{{Key: "ultimo_mensaje.autor_id", Value: userID}},
		bson.D{{Key: "$unset", Value: bson.D{{Key: "ultimo_mensaje", Value: ""}}}})
	return err
}

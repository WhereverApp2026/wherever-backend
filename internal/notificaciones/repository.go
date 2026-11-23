package notificaciones

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	platformmongo "github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
)

// Errores del repositorio.
var (
	ErrNotFound    = errors.New("aviso no encontrado")
	ErrTooManyDevs = errors.New("máximo de dispositivos")
)

// Indexes son los índices de `notificaciones` y `dispositivos`. El único creador es
// scripts/indexes.
func Indexes() []platformmongo.IndexSet {
	return []platformmongo.IndexSet{
		{
			Collection: Collection,
			Models: []mongo.IndexModel{
				{Keys: bson.D{{Key: "usuario_id", Value: 1}, {Key: "creada_en", Value: -1}, {Key: "_id", Value: -1}}, Options: options.Index().SetName("ix_usuario_creada")},
				{Keys: bson.D{{Key: "expira_en", Value: 1}}, Options: options.Index().SetName("ttl_notificacion").SetExpireAfterSeconds(0)},
				// Pestañas de la bandeja (nv-86): la categoría se resuelve a tipos con $in.
				{Keys: bson.D{{Key: "usuario_id", Value: 1}, {Key: "tipo", Value: 1}, {Key: "creada_en", Value: -1}, {Key: "_id", Value: -1}}, Options: options.Index().SetName("ix_notif_categoria")},
			},
		},
		{
			Collection: DevicesCollection,
			Models: []mongo.IndexModel{
				{Keys: bson.D{{Key: "token_hash", Value: 1}}, Options: options.Index().SetName("ux_token_hash").SetUnique(true)},
				{Keys: bson.D{{Key: "usuario_id", Value: 1}, {Key: "ultimo_uso", Value: -1}}, Options: options.Index().SetName("ix_usuario_uso")},
			},
		},
		{
			Collection: GroupedCollection,
			Models: []mongo.IndexModel{
				{Keys: bson.D{{Key: "usuario_id", Value: 1}}, Options: options.Index().SetName("ix_agrupada_usuario")},
				{Keys: bson.D{{Key: "hasta", Value: 1}}, Options: options.Index().SetName("ttl_agrupada").SetExpireAfterSeconds(0)},
			},
		},
	}
}

// Repository lee y escribe avisos y dispositivos; toda consulta filtra por usuario.
type Repository struct {
	notes   *mongo.Collection
	devices *mongo.Collection
	grouped *mongo.Collection
}

// NewRepository recibe la base por constructor.
func NewRepository(db *mongo.Database) *Repository {
	return &Repository{notes: db.Collection(Collection), devices: db.Collection(DevicesCollection), grouped: db.Collection(GroupedCollection)}
}

// Insert guarda un aviso.
func (r *Repository) Insert(ctx context.Context, n *Notification) error {
	_, err := r.notes.InsertOne(ctx, n)
	return err
}

// Position es el último aviso entregado de una página.
type Position struct {
	CreatedAt time.Time
	ID        bson.ObjectID
}

// List trae hasta limit+1 avisos del usuario, del más nuevo al más viejo; con kinds, solo
// esos tipos (una pestaña, ix_notif_categoria).
func (r *Repository) List(ctx context.Context, userID bson.ObjectID, kinds []string, after *Position, limit int) ([]Notification, error) {
	filter := bson.D{{Key: "usuario_id", Value: userID}}
	if kinds != nil {
		filter = append(filter, bson.E{Key: "tipo", Value: bson.D{{Key: "$in", Value: kinds}}})
	}
	if after != nil {
		filter = append(filter, bson.E{Key: "$or", Value: bson.A{
			bson.D{{Key: "creada_en", Value: bson.D{{Key: "$lt", Value: after.CreatedAt}}}},
			bson.D{{Key: "creada_en", Value: after.CreatedAt}, {Key: "_id", Value: bson.D{{Key: "$lt", Value: after.ID}}}},
		}})
	}
	opts := options.Find().SetSort(bson.D{{Key: "creada_en", Value: -1}, {Key: "_id", Value: -1}}).SetLimit(int64(limit + 1))
	return findAll[Notification](ctx, r.notes, filter, opts)
}

func findAll[T any](ctx context.Context, col *mongo.Collection, filter bson.D, opts *options.FindOptionsBuilder) ([]T, error) {
	cur, err := col.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	out := []T{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// UnreadByKind cuenta los avisos sin leer del usuario por tipo (nv-86), en una consulta.
func (r *Repository) UnreadByKind(ctx context.Context, userID bson.ObjectID) (map[string]int64, error) {
	cur, err := r.notes.Aggregate(ctx, mongo.Pipeline{
		{{Key: "$match", Value: bson.D{{Key: "usuario_id", Value: userID}, {Key: "leida_en", Value: bson.D{{Key: "$exists", Value: false}}}}}},
		{{Key: "$group", Value: bson.D{{Key: "_id", Value: "$tipo"}, {Key: "n", Value: bson.D{{Key: "$sum", Value: 1}}}}}},
	})
	if err != nil {
		return nil, err
	}
	var rows []struct {
		Kind string `bson:"_id"`
		N    int64  `bson:"n"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(rows))
	for _, row := range rows {
		out[row.Kind] = row.N
	}
	return out, nil
}

// SetRead marca (o desmarca) un aviso del usuario y devuelve cómo quedó.
func (r *Repository) SetRead(ctx context.Context, userID, id bson.ObjectID, read bool, now time.Time) (Notification, error) {
	update := bson.D{{Key: "$unset", Value: bson.D{{Key: "leida_en", Value: ""}}}}
	if read {
		update = bson.D{{Key: "$set", Value: bson.D{{Key: "leida_en", Value: now}}}}
	}
	var n Notification
	err := r.notes.FindOneAndUpdate(ctx, bson.D{{Key: "_id", Value: id}, {Key: "usuario_id", Value: userID}}, update,
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&n)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Notification{}, ErrNotFound
	}
	return n, err
}

// ReadAll marca como leídos todos los avisos del usuario.
func (r *Repository) ReadAll(ctx context.Context, userID bson.ObjectID, now time.Time) error {
	_, err := r.notes.UpdateMany(ctx, bson.D{{Key: "usuario_id", Value: userID}, {Key: "leida_en", Value: bson.D{{Key: "$exists", Value: false}}}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "leida_en", Value: now}}}})
	return err
}

// Delete borra un aviso del usuario; ErrNotFound si no es suyo o no existe.
func (r *Repository) Delete(ctx context.Context, userID, id bson.ObjectID) error {
	res, err := r.notes.DeleteOne(ctx, bson.D{{Key: "_id", Value: id}, {Key: "usuario_id", Value: userID}})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return ErrNotFound
	}
	return nil
}

// ListByUser trae todos los avisos del usuario, para exportarlos.
func (r *Repository) ListByUser(ctx context.Context, userID bson.ObjectID) ([]Notification, error) {
	return findAll[Notification](ctx, r.notes, bson.D{{Key: "usuario_id", Value: userID}}, options.Find().SetSort(bson.D{{Key: "creada_en", Value: 1}}))
}

// DeleteByUser borra los avisos del usuario y sus reservas de avisos agrupados.
func (r *Repository) DeleteByUser(ctx context.Context, userID bson.ObjectID) error {
	if _, err := r.notes.DeleteMany(ctx, bson.D{{Key: "usuario_id", Value: userID}}); err != nil {
		return err
	}
	_, err := r.grouped.DeleteMany(ctx, bson.D{{Key: "usuario_id", Value: userID}})
	return err
}

// ClaimGroup reserva el aviso `key` de userID por `every`. false = ya se avisó dentro del
// lapso (desde esta petición, otra o la otra instancia). El índice TTL limpia lo vencido.
func (r *Repository) ClaimGroup(ctx context.Context, userID bson.ObjectID, key string, now time.Time, every time.Duration) (bool, error) {
	_, err := r.grouped.UpdateOne(ctx,
		bson.D{{Key: "_id", Value: userID.Hex() + ":" + key}, {Key: "hasta", Value: bson.D{{Key: "$lte", Value: now}}}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "hasta", Value: now.Add(every)}, {Key: "usuario_id", Value: userID}}}},
		options.UpdateOne().SetUpsert(true))
	if mongo.IsDuplicateKeyError(err) {
		return false, nil
	}
	return err == nil, err
}

// RegisterDevice guarda el token para userID o renueva su uso. Si el token era de otra
// cuenta (cambió de sesión en el mismo teléfono), pasa a esta. Una cuenta con 5 teléfonos no
// registra un sexto (ErrTooManyDevs).
// Devuelve el `_id` del documento: la app lo recibe en `Location` (nv-71).
func (r *Repository) RegisterDevice(ctx context.Context, d Device, now time.Time) (bson.ObjectID, error) {
	var got Device
	after := options.FindOneAndUpdate().SetReturnDocument(options.After).SetProjection(bson.D{{Key: "_id", Value: 1}})
	err := r.devices.FindOneAndUpdate(ctx, bson.D{{Key: "token_hash", Value: d.TokenHash}, {Key: "usuario_id", Value: d.UserID}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "ultimo_uso", Value: now}, {Key: "plataforma", Value: d.Platform}}}}, after).Decode(&got)
	if !errors.Is(err, mongo.ErrNoDocuments) {
		return got.ID, err
	}
	n, err := r.devices.CountDocuments(ctx, bson.D{{Key: "usuario_id", Value: d.UserID}, {Key: "token_hash", Value: bson.D{{Key: "$ne", Value: d.TokenHash}}}})
	if err != nil {
		return bson.ObjectID{}, err
	}
	if n >= MaxDevices {
		return bson.ObjectID{}, ErrTooManyDevs
	}
	err = r.devices.FindOneAndUpdate(ctx, bson.D{{Key: "token_hash", Value: d.TokenHash}}, bson.D{
		{Key: "$set", Value: bson.D{{Key: "usuario_id", Value: d.UserID}, {Key: "token", Value: d.Token}, {Key: "plataforma", Value: d.Platform}, {Key: "ultimo_uso", Value: now}}},
		{Key: "$setOnInsert", Value: bson.D{{Key: "_id", Value: bson.NewObjectID()}, {Key: "creado_en", Value: now}}},
	}, after.SetUpsert(true)).Decode(&got)
	return got.ID, err
}

// UpdateDevice ajusta la versión de la app y los canales de un teléfono propio y renueva su
// uso; ErrNotFound si no existe o es de otra cuenta (nv-71).
func (r *Repository) UpdateDevice(ctx context.Context, userID, id bson.ObjectID, set bson.D, now time.Time) (Device, error) {
	var d Device
	err := r.devices.FindOneAndUpdate(ctx, bson.D{{Key: "_id", Value: id}, {Key: "usuario_id", Value: userID}},
		bson.D{{Key: "$set", Value: append(set, bson.E{Key: "ultimo_uso", Value: now})}},
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&d)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Device{}, ErrNotFound
	}
	return d, err
}

// DeleteDevice borra un teléfono propio por id; ErrNotFound si no existe o es de otra cuenta.
func (r *Repository) DeleteDevice(ctx context.Context, userID, id bson.ObjectID) error {
	res, err := r.devices.DeleteOne(ctx, bson.D{{Key: "_id", Value: id}, {Key: "usuario_id", Value: userID}})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return ErrNotFound
	}
	return nil
}

// RemoveDevice quita el token de la cuenta; ErrNotFound si no era suyo.
func (r *Repository) RemoveDevice(ctx context.Context, userID bson.ObjectID, tokenHash string) error {
	res, err := r.devices.DeleteOne(ctx, bson.D{{Key: "token_hash", Value: tokenHash}, {Key: "usuario_id", Value: userID}})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return ErrNotFound
	}
	return nil
}

// RemoveDeviceByHash borra un token que FCM dio por no registrado (nv-41).
func (r *Repository) RemoveDeviceByHash(ctx context.Context, tokenHash string) error {
	_, err := r.devices.DeleteOne(ctx, bson.D{{Key: "token_hash", Value: tokenHash}})
	return err
}

// Devices trae los teléfonos del usuario, del uso más reciente al más viejo.
func (r *Repository) Devices(ctx context.Context, userID bson.ObjectID) ([]Device, error) {
	return findAll[Device](ctx, r.devices, bson.D{{Key: "usuario_id", Value: userID}}, options.Find().SetSort(bson.D{{Key: "ultimo_uso", Value: -1}}))
}

// DeleteDevicesByUser borra los teléfonos del usuario.
func (r *Repository) DeleteDevicesByUser(ctx context.Context, userID bson.ObjectID) error {
	_, err := r.devices.DeleteMany(ctx, bson.D{{Key: "usuario_id", Value: userID}})
	return err
}

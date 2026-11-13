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
			},
		},
		{
			Collection: DevicesCollection,
			Models: []mongo.IndexModel{
				{Keys: bson.D{{Key: "token_hash", Value: 1}}, Options: options.Index().SetName("ux_token_hash").SetUnique(true)},
				{Keys: bson.D{{Key: "usuario_id", Value: 1}, {Key: "ultimo_uso", Value: -1}}, Options: options.Index().SetName("ix_usuario_uso")},
			},
		},
	}
}

// Repository lee y escribe avisos y dispositivos; toda consulta filtra por usuario.
type Repository struct {
	notes   *mongo.Collection
	devices *mongo.Collection
}

// NewRepository recibe la base por constructor.
func NewRepository(db *mongo.Database) *Repository {
	return &Repository{notes: db.Collection(Collection), devices: db.Collection(DevicesCollection)}
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

// List trae hasta limit+1 avisos del usuario, del más nuevo al más viejo.
func (r *Repository) List(ctx context.Context, userID bson.ObjectID, after *Position, limit int) ([]Notification, error) {
	filter := bson.D{{Key: "usuario_id", Value: userID}}
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

// CountUnread cuenta los avisos sin leer del usuario.
func (r *Repository) CountUnread(ctx context.Context, userID bson.ObjectID) (int64, error) {
	return r.notes.CountDocuments(ctx, bson.D{{Key: "usuario_id", Value: userID}, {Key: "leida_en", Value: bson.D{{Key: "$exists", Value: false}}}})
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

// DeleteByUser borra los avisos del usuario.
func (r *Repository) DeleteByUser(ctx context.Context, userID bson.ObjectID) error {
	_, err := r.notes.DeleteMany(ctx, bson.D{{Key: "usuario_id", Value: userID}})
	return err
}

// RegisterDevice guarda el token para userID o renueva su uso. Si el token era de otra
// cuenta (cambió de sesión en el mismo teléfono), pasa a esta. Una cuenta con 5 teléfonos no
// registra un sexto (ErrTooManyDevs).
func (r *Repository) RegisterDevice(ctx context.Context, d Device, now time.Time) error {
	res, err := r.devices.UpdateOne(ctx, bson.D{{Key: "token_hash", Value: d.TokenHash}, {Key: "usuario_id", Value: d.UserID}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "ultimo_uso", Value: now}, {Key: "plataforma", Value: d.Platform}}}})
	if err != nil || res.MatchedCount == 1 {
		return err
	}
	n, err := r.devices.CountDocuments(ctx, bson.D{{Key: "usuario_id", Value: d.UserID}, {Key: "token_hash", Value: bson.D{{Key: "$ne", Value: d.TokenHash}}}})
	if err != nil {
		return err
	}
	if n >= MaxDevices {
		return ErrTooManyDevs
	}
	_, err = r.devices.UpdateOne(ctx, bson.D{{Key: "token_hash", Value: d.TokenHash}}, bson.D{
		{Key: "$set", Value: bson.D{{Key: "usuario_id", Value: d.UserID}, {Key: "token", Value: d.Token}, {Key: "plataforma", Value: d.Platform}, {Key: "ultimo_uso", Value: now}}},
		{Key: "$setOnInsert", Value: bson.D{{Key: "_id", Value: bson.NewObjectID()}, {Key: "creado_en", Value: now}}},
	}, options.UpdateOne().SetUpsert(true))
	return err
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

// Devices trae los teléfonos del usuario, del uso más reciente al más viejo.
func (r *Repository) Devices(ctx context.Context, userID bson.ObjectID) ([]Device, error) {
	return findAll[Device](ctx, r.devices, bson.D{{Key: "usuario_id", Value: userID}}, options.Find().SetSort(bson.D{{Key: "ultimo_uso", Value: -1}}))
}

// DeleteDevicesByUser borra los teléfonos del usuario.
func (r *Repository) DeleteDevicesByUser(ctx context.Context, userID bson.ObjectID) error {
	_, err := r.devices.DeleteMany(ctx, bson.D{{Key: "usuario_id", Value: userID}})
	return err
}

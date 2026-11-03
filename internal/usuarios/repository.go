// Package usuarios guarda a las personas de Wherever (hoja A1). En nv-04 solo trae lo que
// auth necesita: buscar por correo o id, crear, fijar contraseña y marcar el correo como
// verificado. El resto del módulo (registro, perfil, gustos, privacidad) llega en nv-05.
package usuarios

import (
	"context"
	"errors"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	platformmongo "github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
)

// Collection es el nombre de la colección (constitución §5: español, plural, minúsculas).
const Collection = "usuarios"

// Estados de la hoja A1.
const (
	StatusActive = "activo"
)

// ErrNotFound indica que no existe el usuario.
var ErrNotFound = errors.New("usuario no encontrado")

// User son los campos de la hoja A1 que usa auth; el esquema completo llega con nv-05.
type User struct {
	ID             bson.ObjectID `bson:"_id"`
	Name           string        `bson:"nombre"`
	NormalizedName string        `bson:"nombre_normalizado"`
	Email          string        `bson:"correo"`
	EmailVerified  bool          `bson:"correo_verificado"`
	PasswordHash   string        `bson:"password_hash,omitempty"`
	GoogleID       string        `bson:"google_id,omitempty"`
	BirthDate      *time.Time    `bson:"fecha_nacimiento,omitempty"`
	Roles          []string      `bson:"roles"`
	Status         string        `bson:"estado"`
	Version        int           `bson:"version"`
	CreatedAt      time.Time     `bson:"creado_en"`
	UpdatedAt      time.Time     `bson:"actualizado_en"`
}

// Active indica si la cuenta puede iniciar sesión.
func (u User) Active() bool { return u.Status == StatusActive }

// NormalizeEmail deja el correo en minúsculas y sin espacios (hoja A1: único y en minúsculas).
func NormalizeEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

// Repository accede a la colección usuarios.
type Repository struct {
	col *mongo.Collection
}

// NewRepository recibe la base por constructor.
func NewRepository(db *mongo.Database) *Repository {
	return &Repository{col: db.Collection(Collection)}
}

// Indexes son los índices de usuarios que crea nv-04; los demás de A1 llegan con nv-05.
func Indexes() []platformmongo.IndexSet {
	return []platformmongo.IndexSet{{
		Collection: Collection,
		Models: []mongo.IndexModel{
			{Keys: bson.D{{Key: "correo", Value: 1}}, Options: options.Index().SetName("ux_correo").SetUnique(true)},
			{
				Keys: bson.D{{Key: "google_id", Value: 1}},
				Options: options.Index().SetName("ux_google_id").SetUnique(true).
					SetPartialFilterExpression(bson.D{{Key: "google_id", Value: bson.D{{Key: "$exists", Value: true}}}}),
			},
		},
	}}
}

// Create inserta un usuario nuevo; fija id, versión y fechas si faltan.
func (r *Repository) Create(ctx context.Context, u *User) error {
	now := time.Now().UTC()
	if u.ID.IsZero() {
		u.ID = bson.NewObjectID()
	}
	if u.Version == 0 {
		u.Version = 1
	}
	u.Email = NormalizeEmail(u.Email)
	u.CreatedAt, u.UpdatedAt = now, now
	_, err := r.col.InsertOne(ctx, u)
	return err
}

// FindByEmail busca por correo normalizado.
func (r *Repository) FindByEmail(ctx context.Context, email string) (User, error) {
	return r.findOne(ctx, bson.D{{Key: "correo", Value: NormalizeEmail(email)}})
}

// FindByID busca por _id.
func (r *Repository) FindByID(ctx context.Context, id bson.ObjectID) (User, error) {
	return r.findOne(ctx, bson.D{{Key: "_id", Value: id}})
}

// SetPassword guarda el hash nuevo y sube la versión.
func (r *Repository) SetPassword(ctx context.Context, id bson.ObjectID, hash string) error {
	return r.update(ctx, id, bson.D{{Key: "password_hash", Value: hash}})
}

// MarkEmailVerified pone correo_verificado = true.
func (r *Repository) MarkEmailVerified(ctx context.Context, id bson.ObjectID) error {
	return r.update(ctx, id, bson.D{{Key: "correo_verificado", Value: true}})
}

func (r *Repository) findOne(ctx context.Context, filter bson.D) (User, error) {
	var u User
	err := r.col.FindOne(ctx, filter).Decode(&u)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return User{}, ErrNotFound
	}
	return u, err
}

func (r *Repository) update(ctx context.Context, id bson.ObjectID, set bson.D) error {
	set = append(set, bson.E{Key: "actualizado_en", Value: time.Now().UTC()})
	res, err := r.col.UpdateByID(ctx, id, bson.D{
		{Key: "$set", Value: set},
		{Key: "$inc", Value: bson.D{{Key: "version", Value: 1}}},
	})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}

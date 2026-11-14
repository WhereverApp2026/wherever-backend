// Package usuarios guarda a las personas de Wherever (hoja A1, spec nv-05): registro, perfil,
// gustos, preferencias, privacidad y derechos ARCO. Molde nv-02: handler → DTO + validator →
// service → repository. auth (nv-04) lo usa para buscar, crear y fijar contraseñas.
package usuarios

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/WhereverApp2026/wherever-backend/internal/platform/cifrado"
	platformmongo "github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
)

// Collection es el nombre de la colección (constitución §5: español, plural, minúsculas).
const Collection = "usuarios"

// Estados de la hoja A1.
const (
	StatusActive          = "activo"
	StatusBlocked         = "bloqueado"
	StatusPendingDeletion = "eliminacion_pendiente"
)

// Modos de la app (P-65).
const (
	ModeExplore  = "explorar"
	ModeBusiness = "negocio"
)

// Valores iniciales de la hoja A1.
const (
	WelcomePoints = 50
	InitialLevel  = "Explorador"
)

// friendCodeAlphabet evita caracteres que se confunden (0/O, 1/I/L); hoja A1: 8 caracteres.
const (
	friendCodeAlphabet = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"
	friendCodeLen      = 8
	friendCodeRetries  = 5
)

// Errores del repositorio; el service los traduce a apperr.
var (
	ErrNotFound        = errors.New("usuario no encontrado")
	ErrEmailTaken      = errors.New("el correo ya está registrado")
	ErrPhoneTaken      = errors.New("el teléfono ya está registrado")
	ErrUsernameTaken   = errors.New("el nombre de usuario ya está en uso")
	ErrVersionConflict = errors.New("la versión no coincide")
)

// City es la ciudad elegida a mano (`ciudad {id, nombre}`).
type City struct {
	ID   bson.ObjectID `bson:"id"`
	Name string        `bson:"nombre"`
}

// Tastes son las respuestas del onboarding (P-10 a P-12).
type Tastes struct {
	Categories   []string `bson:"categorias"`
	BudgetMin    int      `bson:"presupuesto_min"`
	BudgetMax    int      `bson:"presupuesto_max"`
	Restrictions []string `bson:"restricciones"`
	Allergies    string   `bson:"alergias"`
}

// QuietHours es el horario silencioso de notificaciones.
type QuietHours struct {
	From string `bson:"desde"`
	To   string `bson:"hasta"`
}

// Notifications dice qué notificaciones recibe; las de reservas no se apagan.
type Notifications struct {
	Bookings   bool `bson:"reservas"`
	Coupons    bool `bson:"cupones"`
	Groups     bool `bson:"grupos"`
	Reviews    bool `bson:"resenas"`
	Promotions bool `bson:"promociones"`
}

// Preferences son los ajustes de la app.
type Preferences struct {
	Mobility      string        `bson:"movilidad_default"`
	Theme         string        `bson:"tema"`
	Units         string        `bson:"unidades"`
	QuietHours    *QuietHours   `bson:"horario_silencioso,omitempty"`
	Notifications Notifications `bson:"notificaciones"`
}

// Privacy son los interruptores de P-62 y la aceptación de términos (LFPDPPP).
type Privacy struct {
	Location        bool `bson:"ubicacion"`
	Learning        bool `bson:"aprendizaje_personalizado"`
	Mood            bool `bson:"estado_animo"`
	SensitiveData   bool `bson:"datos_sensibles"`
	UsageData       bool `bson:"datos_uso"`
	PhoneSearchable bool `bson:"buscable_por_telefono"`
	// UsernameSearchable nace encendido (nv-34): ausente (cuentas previas) cuenta como true.
	// Para filtrar en Mongo usa `privacidad.buscable_por_usuario: {$ne: false}`.
	UsernameSearchable *bool  `bson:"buscable_por_usuario,omitempty"`
	TermsVersion       string `bson:"version_terminos"`
	// NoticeVersion es la versión del aviso de privacidad aceptada (nv-19); vacía en cuentas previas.
	NoticeVersion string    `bson:"version_aviso,omitempty"`
	AcceptedAt    time.Time `bson:"fecha_aceptacion"`
	PromoEmail    bool      `bson:"promociones_correo"`
	// BusinessData es el consentimiento expreso `datos_negocio` (nv-55): INE, comprobante,
	// datos fiscales y CLABE. Sin él, el expediente del negocio responde 422.
	BusinessData bool `bson:"datos_negocio,omitempty"`
}

// DefaultPreferences son los ajustes con los que nace una cuenta.
func DefaultPreferences() Preferences {
	return Preferences{
		Mobility: "a_pie", Theme: "sistema", Units: "km",
		Notifications: Notifications{Bookings: true, Coupons: true, Groups: true, Reviews: true, Promotions: false},
	}
}

// User es un documento de `usuarios` (hoja A1 §1 más `version`, constitución §5).
type User struct {
	ID             bson.ObjectID `bson:"_id"`
	Name           string        `bson:"nombre"`
	Username       string        `bson:"nombre_usuario,omitempty"`
	NormalizedName string        `bson:"nombre_normalizado"`
	Email          string        `bson:"correo"`
	EmailVerified  bool          `bson:"correo_verificado"`
	PhoneHash      string        `bson:"telefono_hash,omitempty"`
	PasswordHash   string        `bson:"password_hash,omitempty"`
	GoogleID       string        `bson:"google_id,omitempty"`
	// BirthDate es la fecha en claro para el resto del código; en la base va cifrada en
	// BirthDateStored (texto `enc:v1:…`, o la fecha de antes de la migración) (nv-49).
	BirthDate       *time.Time `bson:"-"`
	BirthDateStored any        `bson:"fecha_nacimiento,omitempty"`
	// Adult se guarda al registrar: la edad ya se validó y la fecha queda cifrada.
	Adult       bool            `bson:"mayor_de_edad,omitempty"`
	PhotoURL    string          `bson:"foto_url,omitempty"`
	FriendCode  string          `bson:"codigo_amigo"`
	Roles       []string        `bson:"roles"`
	Mode        string          `bson:"modo_actual"`
	BusinessIDs []bson.ObjectID `bson:"negocios_ids,omitempty"`
	City        *City           `bson:"ciudad,omitempty"`
	Tastes      *Tastes         `bson:"gustos,omitempty"`
	Preferences Preferences     `bson:"preferencias"`
	Privacy     Privacy         `bson:"privacidad"`
	Points      int             `bson:"puntos"`
	Level       string          `bson:"nivel"`
	StreakWeeks int             `bson:"racha_semanas"`
	Status      string          `bson:"estado"`
	DeleteAt    *time.Time      `bson:"eliminar_en,omitempty"`
	Version     int             `bson:"version"`
	CreatedAt   time.Time       `bson:"creado_en"`
	UpdatedAt   time.Time       `bson:"actualizado_en"`
}

// SearchableByUsername dice si la persona aparece al buscarla por nombre de usuario.
func (p Privacy) SearchableByUsername() bool {
	return p.UsernameSearchable == nil || *p.UsernameSearchable
}

// CanSignIn indica si la cuenta puede iniciar sesión. Una cuenta en eliminación pendiente sí
// puede: es la única forma de entrar a cancelar el borrado dentro de los 30 días.
func (u User) CanSignIn() bool {
	return u.Status == StatusActive || u.Status == StatusPendingDeletion
}

// NormalizeEmail deja el correo en minúsculas y sin espacios (hoja A1: único y en minúsculas).
func NormalizeEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

// Repository accede a la colección usuarios.
type Repository struct {
	col     *mongo.Collection
	newCode func() (string, error)
	// cipher cifra alergias y fecha de nacimiento (nv-49); nil = en claro.
	cipher *cifrado.Llavero
}

// NewRepository recibe la base por constructor.
func NewRepository(db *mongo.Database) *Repository {
	return &Repository{col: db.Collection(Collection), newCode: newFriendCode}
}

// WithCodeGenerator devuelve una copia que genera códigos de amigo con gen (pruebas de colisión).
func (r *Repository) WithCodeGenerator(gen func() (string, error)) *Repository {
	cp := *r
	cp.newCode = gen
	return &cp
}

// Indexes son los índices de la hoja A1.
func Indexes() []platformmongo.IndexSet {
	exists := func(field string) bson.D {
		return bson.D{{Key: field, Value: bson.D{{Key: "$exists", Value: true}}}}
	}
	return []platformmongo.IndexSet{{
		Collection: Collection,
		Models: []mongo.IndexModel{
			{Keys: bson.D{{Key: "correo", Value: 1}}, Options: options.Index().SetName("ux_correo").SetUnique(true)},
			{Keys: bson.D{{Key: "google_id", Value: 1}}, Options: options.Index().SetName("ux_google_id").SetUnique(true).SetPartialFilterExpression(exists("google_id"))},
			{Keys: bson.D{{Key: "codigo_amigo", Value: 1}}, Options: options.Index().SetName("ux_codigo_amigo").SetUnique(true)},
			{Keys: bson.D{{Key: "nombre_normalizado", Value: 1}}, Options: options.Index().SetName("ix_nombre_busqueda")},
			{Keys: bson.D{{Key: "ubicacion", Value: "2dsphere"}}, Options: options.Index().SetName("ix_ubicacion")},
			{Keys: bson.D{{Key: "estado", Value: 1}, {Key: "eliminar_en", Value: 1}}, Options: options.Index().SetName("ix_eliminacion")},
			{Keys: bson.D{{Key: "telefono_hash", Value: 1}}, Options: options.Index().SetName("ux_telefono_hash").SetUnique(true).SetPartialFilterExpression(exists("telefono_hash"))},
			{Keys: bson.D{{Key: "nombre_usuario", Value: 1}}, Options: options.Index().SetName("ux_nombre_usuario").SetUnique(true).SetPartialFilterExpression(exists("nombre_usuario"))},
		},
	}}
}

// Create inserta un usuario nuevo: fija id, versión, fechas y un código de amigo único
// (reintenta si choca con ux_codigo_amigo). Un correo repetido es ErrEmailTaken.
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
	generated := u.FriendCode == ""
	for attempt := 0; ; attempt++ {
		if generated {
			code, err := r.newCode()
			if err != nil {
				return err
			}
			u.FriendCode = code
		}
		doc, err := r.sealUser(u)
		if err != nil {
			return err
		}
		_, err = r.col.InsertOne(ctx, doc)
		if !generated || !isDuplicate(err, "ux_codigo_amigo") || attempt+1 >= friendCodeRetries {
			return mapDuplicate(err)
		}
	}
}

// FindByEmail busca por correo normalizado.
func (r *Repository) FindByEmail(ctx context.Context, email string) (User, error) {
	return r.findOne(ctx, bson.D{{Key: "correo", Value: NormalizeEmail(email)}})
}

// FindByID busca por _id.
func (r *Repository) FindByID(ctx context.Context, id bson.ObjectID) (User, error) {
	return r.findOne(ctx, bson.D{{Key: "_id", Value: id}})
}

// FindByFriendCode busca por `codigo_amigo` con ux_codigo_amigo (QR de amistad, nv-64).
func (r *Repository) FindByFriendCode(ctx context.Context, code string) (User, error) {
	return r.findOne(ctx, bson.D{{Key: "codigo_amigo", Value: code}})
}

// ValidFriendCode dice si code tiene el largo y el alfabeto de los códigos de amigo.
func ValidFriendCode(code string) bool {
	if len(code) != friendCodeLen {
		return false
	}
	for _, r := range code {
		if !strings.ContainsRune(friendCodeAlphabet, r) {
			return false
		}
	}
	return true
}

// EmailExists responde con el índice único ux_correo.
func (r *Repository) EmailExists(ctx context.Context, email string) (bool, error) {
	n, err := r.col.CountDocuments(ctx, bson.D{{Key: "correo", Value: NormalizeEmail(email)}}, options.Count().SetLimit(1))
	return n > 0, err
}

// SetPassword guarda el hash nuevo y sube la versión.
func (r *Repository) SetPassword(ctx context.Context, id bson.ObjectID, hash string) error {
	_, err := r.Update(ctx, id, bson.D{{Key: "password_hash", Value: hash}})
	return err
}

// MarkEmailVerified pone correo_verificado = true.
func (r *Repository) MarkEmailVerified(ctx context.Context, id bson.ObjectID) error {
	_, err := r.Update(ctx, id, bson.D{{Key: "correo_verificado", Value: true}})
	return err
}

// Update aplica `set` sin condición de versión (la sube igual) y devuelve el documento nuevo.
func (r *Repository) Update(ctx context.Context, id bson.ObjectID, set bson.D) (User, error) {
	return r.apply(ctx, bson.D{{Key: "_id", Value: id}}, set, nil)
}

// UpdateIfVersion aplica `set` solo si la versión sigue siendo `version` (If-Match).
func (r *Repository) UpdateIfVersion(ctx context.Context, id bson.ObjectID, version int, set bson.D) (User, error) {
	u, err := r.apply(ctx, bson.D{{Key: "_id", Value: id}, {Key: "version", Value: version}}, set, nil)
	if errors.Is(err, ErrNotFound) {
		if _, findErr := r.FindByID(ctx, id); findErr == nil {
			return User{}, ErrVersionConflict
		}
	}
	return u, err
}

// MarkForDeletion deja la cuenta en eliminación pendiente hasta `at`.
func (r *Repository) MarkForDeletion(ctx context.Context, id bson.ObjectID, at time.Time) (User, error) {
	return r.Update(ctx, id, bson.D{{Key: "estado", Value: StatusPendingDeletion}, {Key: "eliminar_en", Value: at}})
}

// CancelDeletion vuelve la cuenta a activa si sigue pendiente y no ha vencido el plazo.
func (r *Repository) CancelDeletion(ctx context.Context, id bson.ObjectID, now time.Time) (User, error) {
	return r.apply(ctx,
		bson.D{{Key: "_id", Value: id}, {Key: "estado", Value: StatusPendingDeletion}, {Key: "eliminar_en", Value: bson.D{{Key: "$gt", Value: now}}}},
		bson.D{{Key: "estado", Value: StatusActive}},
		bson.D{{Key: "eliminar_en", Value: ""}},
	)
}

// DueForDeletion devuelve hasta `limit` cuentas cuyo plazo de borrado ya llegó (ix_eliminacion).
func (r *Repository) DueForDeletion(ctx context.Context, now time.Time, limit int64) ([]bson.ObjectID, error) {
	cur, err := r.col.Find(ctx,
		bson.D{{Key: "estado", Value: StatusPendingDeletion}, {Key: "eliminar_en", Value: bson.D{{Key: "$lte", Value: now}}}},
		options.Find().SetProjection(bson.D{{Key: "_id", Value: 1}}).SetLimit(limit))
	if err != nil {
		return nil, err
	}
	var docs []struct {
		ID bson.ObjectID `bson:"_id"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	ids := make([]bson.ObjectID, len(docs))
	for i, d := range docs {
		ids[i] = d.ID
	}
	return ids, nil
}

// Delete borra el documento (lo usa el trabajo nocturno al vencer los 30 días).
func (r *Repository) Delete(ctx context.Context, id bson.ObjectID) error {
	_, err := r.col.DeleteOne(ctx, bson.D{{Key: "_id", Value: id}})
	return err
}

func (r *Repository) findOne(ctx context.Context, filter bson.D) (User, error) {
	var u User
	err := r.col.FindOne(ctx, filter).Decode(&u)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	return u, r.open(&u)
}

// apply hace $set (más `actualizado_en`), $unset opcional y `version + 1`, y devuelve el
// documento nuevo.
func (r *Repository) apply(ctx context.Context, filter, set, unset bson.D) (User, error) {
	set, err := r.sealSet(idOf(filter), set)
	if err != nil {
		return User{}, err
	}
	set = append(set, bson.E{Key: "actualizado_en", Value: time.Now().UTC()})
	update := bson.D{{Key: "$set", Value: set}, {Key: "$inc", Value: bson.D{{Key: "version", Value: 1}}}}
	if len(unset) > 0 {
		update = append(update, bson.E{Key: "$unset", Value: unset})
	}
	var u User
	err = r.col.FindOneAndUpdate(ctx, filter, update, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&u)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, mapDuplicate(err)
	}
	return u, r.open(&u)
}

func isDuplicate(err error, index string) bool {
	return err != nil && mongo.IsDuplicateKeyError(err) && strings.Contains(err.Error(), index)
}

func mapDuplicate(err error) error {
	switch {
	case isDuplicate(err, "ux_correo"):
		return ErrEmailTaken
	case isDuplicate(err, "ux_telefono_hash"):
		return ErrPhoneTaken
	case isDuplicate(err, "ux_nombre_usuario"):
		return ErrUsernameTaken
	default:
		return err
	}
}

func newFriendCode() (string, error) {
	out := make([]byte, friendCodeLen)
	limit := big.NewInt(int64(len(friendCodeAlphabet)))
	for i := range out {
		n, err := rand.Int(rand.Reader, limit)
		if err != nil {
			return "", err
		}
		out[i] = friendCodeAlphabet[n.Int64()]
	}
	return string(out), nil
}

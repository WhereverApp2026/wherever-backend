package auth

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	platformmongo "github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
)

// Colecciones de la hoja J1.
const (
	SessionsCollection = "sesiones"
	TokensCollection   = "tokens_un_uso" // #nosec G101 -- nombre de colección, no una credencial
)

// Métodos de inicio de sesión (hoja J1: correo o google).
const (
	MethodEmail = "correo"
)

var errNoDocument = errors.New("sin documento")

// Device es el teléfono que abrió la sesión.
type Device struct {
	Model      string `bson:"modelo" json:"modelo"`
	System     string `bson:"sistema" json:"sistema"`
	AppVersion string `bson:"app_version" json:"app_version"`
}

// Session es un documento de `sesiones` (hoja J1 §1). Agrega `refresh_hash_previo` (spec
// nv-04) para reconocer un refresh ya rotado, y `jti_vigente` para revocar el access token de
// la sesión cuando se cierra desde otro dispositivo (hoja J1 v3). El access se emite al mismo
// tiempo que se escribe `ultimo_uso`, así que vence a más tardar en `ultimo_uso` + vida del access.
type Session struct {
	ID                  bson.ObjectID `bson:"_id"`
	UserID              bson.ObjectID `bson:"usuario_id"`
	RefreshHash         string        `bson:"refresh_hash"`
	PreviousRefreshHash string        `bson:"refresh_hash_previo,omitempty"`
	FamilyID            string        `bson:"familia_id"`
	Method              string        `bson:"metodo"`
	Device              Device        `bson:"dispositivo"`
	IP                  string        `bson:"ip"`
	CurrentJTI          string        `bson:"jti_vigente"`
	LastUsed            time.Time     `bson:"ultimo_uso"`
	ExpiresAt           time.Time     `bson:"expira_en"`
	RevokedAt           *time.Time    `bson:"revocada_en,omitempty"`
	CreatedAt           time.Time     `bson:"creado_en"`
}

// OneTimeToken es un documento de `tokens_un_uso` (hoja J1 §2).
type OneTimeToken struct {
	ID        bson.ObjectID `bson:"_id"`
	UserID    bson.ObjectID `bson:"usuario_id"`
	Kind      string        `bson:"tipo"`
	TokenHash string        `bson:"token_hash"`
	ExpiresAt time.Time     `bson:"expira_en"`
	UsedAt    *time.Time    `bson:"usado_en,omitempty"`
	CreatedAt time.Time     `bson:"creado_en"`
}

// Repository accede a `sesiones` y `tokens_un_uso`.
type Repository struct {
	sessions *mongo.Collection
	tokens   *mongo.Collection
}

// NewRepository recibe la base por constructor.
func NewRepository(db *mongo.Database) *Repository {
	return &Repository{sessions: db.Collection(SessionsCollection), tokens: db.Collection(TokensCollection)}
}

// Indexes son los índices de la hoja J1 más `ix_refresh_previo` (spec nv-04).
func Indexes() []platformmongo.IndexSet {
	idx := func(name string, keys bson.D) mongo.IndexModel {
		return mongo.IndexModel{Keys: keys, Options: options.Index().SetName(name)}
	}
	unique := func(name string, keys bson.D) mongo.IndexModel {
		return mongo.IndexModel{Keys: keys, Options: options.Index().SetName(name).SetUnique(true)}
	}
	ttl := func(name string) mongo.IndexModel {
		return mongo.IndexModel{Keys: bson.D{{Key: "expira_en", Value: 1}}, Options: options.Index().SetName(name).SetExpireAfterSeconds(0)}
	}
	return []platformmongo.IndexSet{
		{Collection: SessionsCollection, Models: []mongo.IndexModel{
			unique("ux_refresh_hash", bson.D{{Key: "refresh_hash", Value: 1}}),
			idx("ix_refresh_previo", bson.D{{Key: "refresh_hash_previo", Value: 1}}),
			idx("ix_usuario_sesiones", bson.D{{Key: "usuario_id", Value: 1}, {Key: "revocada_en", Value: 1}}),
			idx("ix_familia", bson.D{{Key: "familia_id", Value: 1}}),
			ttl("ttl_sesion"),
		}},
		{Collection: TokensCollection, Models: []mongo.IndexModel{
			unique("ux_token_hash", bson.D{{Key: "token_hash", Value: 1}}),
			idx("ix_usuario_tipo", bson.D{{Key: "usuario_id", Value: 1}, {Key: "tipo", Value: 1}, {Key: "creado_en", Value: -1}}),
			ttl("ttl_token"),
		}},
	}
}

func activeFilter(now time.Time) bson.D {
	return bson.D{{Key: "revocada_en", Value: nil}, {Key: "expira_en", Value: bson.D{{Key: "$gt", Value: now}}}}
}

// InsertSession guarda una sesión nueva.
func (r *Repository) InsertSession(ctx context.Context, s Session) error {
	_, err := r.sessions.InsertOne(ctx, s)
	return err
}

// FindActiveByRefreshHash busca la sesión vigente dueña del refresh.
func (r *Repository) FindActiveByRefreshHash(ctx context.Context, hash string, now time.Time) (Session, error) {
	filter := append(bson.D{{Key: "refresh_hash", Value: hash}}, activeFilter(now)...)
	return r.findSession(ctx, filter)
}

// FindByPreviousHash busca la sesión cuyo refresh anterior es este (reutilización).
func (r *Repository) FindByPreviousHash(ctx context.Context, hash string) (Session, error) {
	return r.findSession(ctx, bson.D{{Key: "refresh_hash_previo", Value: hash}})
}

// Rotate cambia el refresh vigente solo si sigue siendo `oldHash`; false si otra petición ganó.
func (r *Repository) Rotate(ctx context.Context, s Session, oldHash string, now time.Time) (bool, error) {
	res, err := r.sessions.UpdateOne(ctx,
		bson.D{{Key: "_id", Value: s.ID}, {Key: "refresh_hash", Value: oldHash}, {Key: "revocada_en", Value: nil}},
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "refresh_hash", Value: s.RefreshHash},
			{Key: "refresh_hash_previo", Value: oldHash},
			{Key: "jti_vigente", Value: s.CurrentJTI},
			{Key: "ip", Value: s.IP},
			{Key: "ultimo_uso", Value: now},
		}}},
	)
	if err != nil {
		return false, err
	}
	return res.ModifiedCount == 1, nil
}

// ListActive devuelve las sesiones abiertas del usuario, la más reciente primero.
func (r *Repository) ListActive(ctx context.Context, userID bson.ObjectID, now time.Time) ([]Session, error) {
	filter := append(bson.D{{Key: "usuario_id", Value: userID}}, activeFilter(now)...)
	cur, err := r.sessions.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "ultimo_uso", Value: -1}}))
	if err != nil {
		return nil, err
	}
	var out []Session
	return out, cur.All(ctx, &out)
}

// RevokeUserSessions cierra las sesiones abiertas del usuario, menos `keep` si no es cero.
func (r *Repository) RevokeUserSessions(ctx context.Context, userID, keep bson.ObjectID, now time.Time) ([]Session, error) {
	filter := append(bson.D{{Key: "usuario_id", Value: userID}}, activeFilter(now)...)
	if !keep.IsZero() {
		filter = append(filter, bson.E{Key: "_id", Value: bson.D{{Key: "$ne", Value: keep}}})
	}
	return r.revoke(ctx, filter, now)
}

// RevokeSession cierra una sesión abierta del usuario; errNoDocument si no es suya o no existe.
func (r *Repository) RevokeSession(ctx context.Context, userID, id bson.ObjectID, now time.Time) (Session, error) {
	filter := append(bson.D{{Key: "_id", Value: id}, {Key: "usuario_id", Value: userID}}, activeFilter(now)...)
	revoked, err := r.revoke(ctx, filter, now)
	if err != nil {
		return Session{}, err
	}
	if len(revoked) == 0 {
		return Session{}, errNoDocument
	}
	return revoked[0], nil
}

// RevokeFamily cierra todas las sesiones de la familia (reutilización de refresh).
func (r *Repository) RevokeFamily(ctx context.Context, familyID string, now time.Time) ([]Session, error) {
	return r.revoke(ctx, bson.D{{Key: "familia_id", Value: familyID}, {Key: "revocada_en", Value: nil}}, now)
}

// revoke marca revocada_en y devuelve las sesiones afectadas para revocar sus jti.
func (r *Repository) revoke(ctx context.Context, filter bson.D, now time.Time) ([]Session, error) {
	cur, err := r.sessions.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	var found []Session
	if err := cur.All(ctx, &found); err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, nil
	}
	ids := make([]bson.ObjectID, len(found))
	for i, s := range found {
		ids[i] = s.ID
	}
	_, err = r.sessions.UpdateMany(ctx,
		bson.D{{Key: "_id", Value: bson.D{{Key: "$in", Value: ids}}}, {Key: "revocada_en", Value: nil}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "revocada_en", Value: now}}}},
	)
	return found, err
}

func (r *Repository) findSession(ctx context.Context, filter bson.D) (Session, error) {
	var s Session
	err := r.sessions.FindOne(ctx, filter).Decode(&s)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Session{}, errNoDocument
	}
	return s, err
}

// InsertToken guarda un token de enlace.
func (r *Repository) InsertToken(ctx context.Context, t OneTimeToken) error {
	_, err := r.tokens.InsertOne(ctx, t)
	return err
}

// LastTokenAt devuelve cuándo se creó el último token de ese tipo; ok=false si no hay.
func (r *Repository) LastTokenAt(ctx context.Context, userID bson.ObjectID, kind string) (time.Time, bool, error) {
	var t OneTimeToken
	err := r.tokens.FindOne(ctx,
		bson.D{{Key: "usuario_id", Value: userID}, {Key: "tipo", Value: kind}},
		options.FindOne().SetSort(bson.D{{Key: "creado_en", Value: -1}}),
	).Decode(&t)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	return t.CreatedAt, true, nil
}

// InvalidateTokens deja sin efecto los enlaces sin usar de ese tipo (los marca como usados).
func (r *Repository) InvalidateTokens(ctx context.Context, userID bson.ObjectID, kind string, now time.Time) error {
	_, err := r.tokens.UpdateMany(ctx,
		bson.D{{Key: "usuario_id", Value: userID}, {Key: "tipo", Value: kind}, {Key: "usado_en", Value: nil}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "usado_en", Value: now}}}})
	return err
}

// ConsumeToken marca el token como usado si es de ese tipo, no se usó y no venció.
// Es atómico: dos peticiones con el mismo enlace no lo pueden usar las dos.
func (r *Repository) ConsumeToken(ctx context.Context, hash, kind string, now time.Time) (OneTimeToken, error) {
	var t OneTimeToken
	err := r.tokens.FindOneAndUpdate(ctx,
		bson.D{
			{Key: "token_hash", Value: hash},
			{Key: "tipo", Value: kind},
			{Key: "usado_en", Value: nil},
			{Key: "expira_en", Value: bson.D{{Key: "$gt", Value: now}}},
		},
		bson.D{{Key: "$set", Value: bson.D{{Key: "usado_en", Value: now}}}},
	).Decode(&t)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return OneTimeToken{}, errNoDocument
	}
	return t, err
}

// DeleteUserData borra las sesiones y los enlaces del usuario (borrado ARCO a los 30 días).
func (r *Repository) DeleteUserData(ctx context.Context, userID bson.ObjectID) error {
	filter := bson.D{{Key: "usuario_id", Value: userID}}
	if _, err := r.sessions.DeleteMany(ctx, filter); err != nil {
		return err
	}
	_, err := r.tokens.DeleteMany(ctx, filter)
	return err
}

// ListByUser devuelve todas las sesiones del usuario, abiertas o no (exportación ARCO).
func (r *Repository) ListByUser(ctx context.Context, userID bson.ObjectID) ([]Session, error) {
	cur, err := r.sessions.Find(ctx, bson.D{{Key: "usuario_id", Value: userID}}, options.Find().SetSort(bson.D{{Key: "creado_en", Value: -1}}))
	if err != nil {
		return nil, err
	}
	var out []Session
	return out, cur.All(ctx, &out)
}

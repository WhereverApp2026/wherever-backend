package usuarios

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// HashPhone es el HMAC-SHA-256 del teléfono en E.164 (nv-05). El teléfono nunca se guarda en
// claro: registrar (nv-05) y buscar (nv-35) usan esta misma función con la misma llave.
func HashPhone(key []byte, phone string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(phone))
	return hex.EncodeToString(mac.Sum(nil))
}

// searchProjection limita lo que sale de Mongo en una búsqueda: nunca correo ni teléfono.
var searchProjection = bson.D{
	{Key: "nombre", Value: 1}, {Key: "nombre_usuario", Value: 1}, {Key: "foto_url", Value: 1},
}

// SearchByUsernamePrefix busca cuentas activas cuyo nombre de usuario empieza con prefix
// (ya validado: `[a-z0-9_]`), con `buscable_por_usuario` encendido, sin incluir a exclude
// (quien busca y quien tenga relación bloqueada con él, nv-36). La regex anclada usa el
// índice ux_nombre_usuario.
func (r *Repository) SearchByUsernamePrefix(ctx context.Context, prefix string, exclude []bson.ObjectID, limit int64) ([]User, error) {
	filter := bson.D{
		{Key: "nombre_usuario", Value: bson.D{{Key: "$regex", Value: "^" + prefix}}},
		{Key: "estado", Value: StatusActive},
		{Key: "privacidad.buscable_por_usuario", Value: bson.D{{Key: "$ne", Value: false}}},
		{Key: "_id", Value: bson.D{{Key: "$nin", Value: exclude}}},
	}
	opts := options.Find().SetProjection(searchProjection).
		SetSort(bson.D{{Key: "nombre_usuario", Value: 1}}).SetLimit(limit)
	cur, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	out := []User{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// FindByPhoneSearchable busca la cuenta activa con ese hash de teléfono que aceptó ser
// encontrada por teléfono. Es una sola consulta por ux_telefono_hash, exista o no la persona,
// para que el tiempo no delate si el número está registrado. exclude, como en
// SearchByUsernamePrefix.
func (r *Repository) FindByPhoneSearchable(ctx context.Context, phoneHash string, exclude []bson.ObjectID) (User, bool, error) {
	filter := bson.D{
		{Key: "telefono_hash", Value: phoneHash},
		{Key: "estado", Value: StatusActive},
		{Key: "privacidad.buscable_por_telefono", Value: true},
		{Key: "_id", Value: bson.D{{Key: "$nin", Value: exclude}}},
	}
	var u User
	err := r.col.FindOne(ctx, filter, options.FindOne().SetProjection(searchProjection)).Decode(&u)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return User{}, false, nil
	}
	if err != nil {
		return User{}, false, err
	}
	return u, true, nil
}

// FindPublicByIDs devuelve lo público (nombre, nombre de usuario y foto) de las cuentas activas
// con esos ids; las que no existen o no están activas no aparecen (lista de amigos, nv-36).
func (r *Repository) FindPublicByIDs(ctx context.Context, ids []bson.ObjectID) ([]User, error) {
	out := []User{}
	if len(ids) == 0 {
		return out, nil
	}
	filter := bson.D{{Key: "_id", Value: bson.D{{Key: "$in", Value: ids}}}, {Key: "estado", Value: StatusActive}}
	cur, err := r.col.Find(ctx, filter, options.Find().SetProjection(searchProjection))
	if err != nil {
		return nil, err
	}
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

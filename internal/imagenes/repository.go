package imagenes

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
	ErrNotFound  = errors.New("imagen no encontrada")
	ErrNoTomable = errors.New("imagen no confirmable en su estado")
)

// Indexes son los índices de `imagenes`. El único creador es scripts/indexes.
func Indexes() []platformmongo.IndexSet {
	return []platformmongo.IndexSet{{
		Collection: Collection,
		Models: []mongo.IndexModel{
			{Keys: bson.D{{Key: "dueno_id", Value: 1}, {Key: "creada_en", Value: -1}}, Options: options.Index().SetName("ix_dueno_creada")},
			{Keys: bson.D{{Key: "expira_en", Value: 1}}, Options: options.Index().SetName("ttl_imagen_pendiente").SetExpireAfterSeconds(0)},
		},
	}}
}

// Repository lee y escribe `imagenes`; toda consulta de la persona filtra por dueño.
type Repository struct {
	col *mongo.Collection
}

// NewRepository recibe la base por constructor.
func NewRepository(db *mongo.Database) *Repository {
	return &Repository{col: db.Collection(Collection)}
}

// Insert guarda una imagen nueva.
func (r *Repository) Insert(ctx context.Context, img *Imagen) error {
	_, err := r.col.InsertOne(ctx, img)
	return err
}

// Buscar devuelve la imagen si es del dueño.
func (r *Repository) Buscar(ctx context.Context, id string, dueno bson.ObjectID) (*Imagen, error) {
	var img Imagen
	err := r.col.FindOne(ctx, bson.M{"_id": id, "dueno_id": dueno}).Decode(&img)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &img, nil
}

// Tomar pasa la imagen a `procesando` de forma atómica: solo una petición la procesa. Se puede
// tomar si está pendiente, si la moderación falló (no hubo veredicto) o si otra petición la
// dejó procesando antes de `abandonadaAntes` (la instancia murió a la mitad).
func (r *Repository) Tomar(ctx context.Context, id string, dueno bson.ObjectID, ahora, abandonadaAntes time.Time) (*Imagen, error) {
	filtro := bson.M{"_id": id, "dueno_id": dueno, "$or": bson.A{
		bson.M{"estado": EstadoPendiente},
		bson.M{"estado": EstadoEnRevision, "moderacion.falla": bson.M{"$exists": true, "$ne": ""}},
		bson.M{"estado": EstadoProcesando, "actualizada_en": bson.M{"$lt": abandonadaAntes}},
	}}
	cambio := bson.M{"$set": bson.M{"estado": EstadoProcesando, "actualizada_en": ahora}}
	var img Imagen
	err := r.col.FindOneAndUpdate(ctx, filtro, cambio, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&img)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrNoTomable
	}
	if err != nil {
		return nil, err
	}
	return &img, nil
}

// Terminar guarda el resultado si la imagen sigue `procesando`; devuelve false si mientras
// tanto la borraron.
func (r *Repository) Terminar(ctx context.Context, img *Imagen) (bool, error) {
	res, err := r.col.ReplaceOne(ctx, bson.M{"_id": img.ID, "estado": EstadoProcesando}, img)
	if err != nil {
		return false, err
	}
	return res.MatchedCount == 1, nil
}

// Eliminar borra la imagen del dueño; devuelve false si no existe o es ajena.
func (r *Repository) Eliminar(ctx context.Context, id string, dueno bson.ObjectID) (bool, error) {
	res, err := r.col.DeleteOne(ctx, bson.M{"_id": id, "dueno_id": dueno})
	if err != nil {
		return false, err
	}
	return res.DeletedCount == 1, nil
}

// ListarPorDueno devuelve todas las imágenes de la persona, la más reciente primero.
func (r *Repository) ListarPorDueno(ctx context.Context, dueno bson.ObjectID) ([]Imagen, error) {
	cur, err := r.col.Find(ctx, bson.M{"dueno_id": dueno}, options.Find().SetSort(bson.D{{Key: "creada_en", Value: -1}}))
	if err != nil {
		return nil, err
	}
	var out []Imagen
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// EliminarPorDueno borra todas las filas de la persona.
func (r *Repository) EliminarPorDueno(ctx context.Context, dueno bson.ObjectID) error {
	_, err := r.col.DeleteMany(ctx, bson.M{"dueno_id": dueno})
	return err
}

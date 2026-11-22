package reacciones

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/postales"
)

// ErrNoEncontrado: el comentario no existe o ya no aplica.
var ErrNoEncontrado = errors.New("reacciones: no encontrado")

// Campos de los contadores en el documento del objeto. Los momentos ya tenían
// `reacciones_total` (nv-63); los planes usan los nombres de nv-84.
const (
	campoMeGustaMomento = "reacciones_total"
	campoMeGustaPlan    = "me_gusta_total"
	campoComentarios    = "comentarios_total"
)

// Repository lee y escribe `me_gusta` y `comentarios`, y mueve los contadores del objeto
// (`momentos` o `planes`) con `$inc`. Es el único que importa el driver en este paquete.
type Repository struct {
	likes    *mongo.Collection
	comments *mongo.Collection
	objetos  map[string]*mongo.Collection
	now      func() time.Time
}

// NewRepository recibe la base por constructor.
func NewRepository(db *mongo.Database) *Repository {
	return &Repository{
		likes: db.Collection(MeGustaCollection), comments: db.Collection(ComentariosCollection),
		objetos: map[string]*mongo.Collection{ObjetoMomento: db.Collection(postales.MomentosCollection), ObjetoPlan: db.Collection(planes.Collection)},
		now:     time.Now,
	}
}

func objeto(tipo string, id bson.ObjectID) bson.D {
	return bson.D{{Key: "objeto_tipo", Value: tipo}, {Key: "objeto_id", Value: id}}
}

func campoMeGusta(tipo string) string {
	if tipo == ObjetoPlan {
		return campoMeGustaPlan
	}
	return campoMeGustaMomento
}

// contar mueve un contador del objeto; un objeto borrado en medio no es error.
func (r *Repository) contar(ctx context.Context, tipo string, id bson.ObjectID, campo string, delta int) error {
	col, ok := r.objetos[tipo]
	if !ok || delta == 0 {
		return nil
	}
	_, err := col.UpdateOne(ctx, bson.D{{Key: "_id", Value: id}}, bson.D{{Key: "$inc", Value: bson.D{{Key: campo, Value: delta}}}})
	return err
}

// DarMeGusta guarda el me gusta y sube el contador solo si es nuevo. true = nuevo.
func (r *Repository) DarMeGusta(ctx context.Context, m MeGusta) (bool, error) {
	_, err := r.likes.InsertOne(ctx, m)
	if mongo.IsDuplicateKeyError(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, r.contar(ctx, m.ObjetoTipo, m.ObjetoID, campoMeGusta(m.ObjetoTipo), 1)
}

// QuitarMeGusta borra el me gusta y baja el contador solo si existía.
func (r *Repository) QuitarMeGusta(ctx context.Context, tipo string, id, usuario bson.ObjectID) error {
	res, err := r.likes.DeleteOne(ctx, append(objeto(tipo, id), bson.E{Key: "usuario_id", Value: usuario}))
	if err != nil || res.DeletedCount == 0 {
		return err
	}
	return r.contar(ctx, tipo, id, campoMeGusta(tipo), -1)
}

// MeGustaDe son quienes dieron me gusta al objeto, del más reciente al más viejo.
func (r *Repository) MeGustaDe(ctx context.Context, tipo string, id bson.ObjectID, limite int64) ([]MeGusta, error) {
	return findAll[MeGusta](ctx, r.likes, objeto(tipo, id), options.Find().SetSort(bson.D{{Key: "creado_en", Value: -1}}).SetLimit(limite))
}

// ContarMeGusta cuenta los me gusta del objeto.
func (r *Repository) ContarMeGusta(ctx context.Context, tipo string, id bson.ObjectID) (int64, error) {
	return r.likes.CountDocuments(ctx, objeto(tipo, id))
}

// ContarMeGustaDe cuenta cuántas de esas personas dieron me gusta al objeto.
func (r *Repository) ContarMeGustaDe(ctx context.Context, tipo string, id bson.ObjectID, usuarios []bson.ObjectID) (int64, error) {
	if len(usuarios) == 0 {
		return 0, nil
	}
	return r.likes.CountDocuments(ctx, append(objeto(tipo, id), bson.E{Key: "usuario_id", Value: bson.D{{Key: "$in", Value: usuarios}}}))
}

// InsertarComentario guarda el comentario y, si nace publicado, suma al contador del objeto.
func (r *Repository) InsertarComentario(ctx context.Context, c Comentario) error {
	if _, err := r.comments.InsertOne(ctx, c); err != nil {
		return err
	}
	if c.Estado != EstadoPublicado {
		return nil
	}
	return r.contar(ctx, c.ObjetoTipo, c.ObjetoID, campoComentarios, 1)
}

// Comentario trae un comentario por id.
func (r *Repository) Comentario(ctx context.Context, id bson.ObjectID) (Comentario, error) {
	var c Comentario
	err := r.comments.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&c)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return c, ErrNoEncontrado
	}
	return c, err
}

// Pos es la posición del último comentario de una página (más viejo primero).
type Pos struct {
	Fecha time.Time
	ID    bson.ObjectID
}

// Principales pagina los comentarios de primer nivel, del más viejo al más nuevo; trae
// limite+1 para saber si hay más.
func (r *Repository) Principales(ctx context.Context, tipo string, id bson.ObjectID, after *Pos, limite int) ([]Comentario, error) {
	filter := append(objeto(tipo, id), bson.E{Key: "padre_id", Value: nil})
	if after != nil {
		filter = append(filter, bson.E{Key: "$or", Value: bson.A{
			bson.D{{Key: "creado_en", Value: bson.D{{Key: "$gt", Value: after.Fecha}}}},
			bson.D{{Key: "creado_en", Value: after.Fecha}, {Key: "_id", Value: bson.D{{Key: "$gt", Value: after.ID}}}},
		}})
	}
	return findAll[Comentario](ctx, r.comments, filter,
		options.Find().SetSort(bson.D{{Key: "creado_en", Value: 1}, {Key: "_id", Value: 1}}).SetLimit(int64(limite)+1))
}

// Respuestas trae las respuestas de esos comentarios, del más viejo al más nuevo.
func (r *Repository) Respuestas(ctx context.Context, tipo string, id bson.ObjectID, padres []bson.ObjectID) ([]Comentario, error) {
	if len(padres) == 0 {
		return []Comentario{}, nil
	}
	filter := append(objeto(tipo, id), bson.E{Key: "padre_id", Value: bson.D{{Key: "$in", Value: padres}}})
	return findAll[Comentario](ctx, r.comments, filter,
		options.Find().SetSort(bson.D{{Key: "creado_en", Value: 1}, {Key: "_id", Value: 1}}).SetLimit(MaxRespuestas))
}

// Eliminar deja el comentario como «Comentario eliminado» (sin texto) y baja el contador si
// estaba publicado. Otro borrado que ganó la carrera = ErrNoEncontrado.
func (r *Repository) Eliminar(ctx context.Context, c Comentario, now time.Time) error {
	res, err := r.comments.UpdateOne(ctx, bson.D{{Key: "_id", Value: c.ID}, {Key: "estado", Value: c.Estado}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "estado", Value: EstadoEliminado}, {Key: "texto", Value: ""}, {Key: "eliminado_en", Value: now}}}})
	if err != nil {
		return err
	}
	if res.ModifiedCount == 0 {
		return ErrNoEncontrado
	}
	if c.Estado != EstadoPublicado {
		return nil
	}
	return r.contar(ctx, c.ObjetoTipo, c.ObjetoID, campoComentarios, -1)
}

// Moderar aplica la decisión de la cola de nv-43 a un comentario que sigue en moderación.
// true = cambió (y, si se aprobó, ya sumó al contador).
func (r *Repository) Moderar(ctx context.Context, id bson.ObjectID, estado string) (Comentario, bool, error) {
	var c Comentario
	err := r.comments.FindOneAndUpdate(ctx, bson.D{{Key: "_id", Value: id}, {Key: "estado", Value: EstadoEnModeracion}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "estado", Value: estado}}}}, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&c)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return c, false, nil
	}
	if err != nil {
		return c, false, err
	}
	if estado == EstadoPublicado {
		return c, true, r.contar(ctx, c.ObjetoTipo, c.ObjetoID, campoComentarios, 1)
	}
	return c, true, nil
}

// MeGustaDeUsuario trae los me gusta de la persona (exportación ARCO).
func (r *Repository) MeGustaDeUsuario(ctx context.Context, usuario bson.ObjectID) ([]MeGusta, error) {
	return findAll[MeGusta](ctx, r.likes, bson.D{{Key: "usuario_id", Value: usuario}}, options.Find().SetSort(bson.D{{Key: "creado_en", Value: 1}}))
}

// ComentariosDe trae los comentarios no eliminados de la persona (exportación ARCO).
func (r *Repository) ComentariosDe(ctx context.Context, usuario bson.ObjectID) ([]Comentario, error) {
	return findAll[Comentario](ctx, r.comments, bson.D{{Key: "autor_id", Value: usuario}, {Key: "estado", Value: bson.D{{Key: "$ne", Value: EstadoEliminado}}}},
		options.Find().SetSort(bson.D{{Key: "creado_en", Value: 1}}))
}

// BorrarDe es el borrado de cuenta (ARCO): quita sus me gusta (bajando cada contador), deja
// sus comentarios como «Comentario eliminado» sin autor (bajando el contador de los publicados)
// y borra lo que recibieron sus propios objetos.
func (r *Repository) BorrarDe(ctx context.Context, usuario bson.ObjectID) error {
	likes, err := r.MeGustaDeUsuario(ctx, usuario)
	if err != nil {
		return err
	}
	for _, m := range likes {
		if err := r.QuitarMeGusta(ctx, m.ObjetoTipo, m.ObjetoID, usuario); err != nil {
			return err
		}
	}
	comments, err := r.ComentariosDe(ctx, usuario)
	if err != nil {
		return err
	}
	now := r.now().UTC()
	for _, c := range comments {
		if err := r.Eliminar(ctx, c, now); err != nil && !errors.Is(err, ErrNoEncontrado) {
			return err
		}
	}
	if _, err := r.comments.UpdateMany(ctx, bson.D{{Key: "autor_id", Value: usuario}}, bson.D{{Key: "$unset", Value: bson.D{{Key: "autor_id", Value: ""}}}}); err != nil {
		return err
	}
	// Sus momentos y planes ya no existen: se van los me gusta y comentarios que recibieron.
	if _, err := r.likes.DeleteMany(ctx, bson.D{{Key: "dueno_id", Value: usuario}}); err != nil {
		return err
	}
	_, err = r.comments.DeleteMany(ctx, bson.D{{Key: "dueno_id", Value: usuario}})
	return err
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

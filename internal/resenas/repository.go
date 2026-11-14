package resenas

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Errores del repositorio; el service los traduce a HTTP.
var (
	ErrNoEncontrado = errors.New("resenas: no encontrado")
	ErrDuplicado    = errors.New("resenas: duplicado")
	ErrNoAplica     = errors.New("resenas: el documento no cumple la condición")
)

// Repository accede a `checkins`, `resenas` y `resenas_utiles`.
type Repository struct {
	checkins *mongo.Collection
	resenas  *mongo.Collection
	utiles   *mongo.Collection
}

// NewRepository recibe la base por constructor.
func NewRepository(db *mongo.Database) *Repository {
	return &Repository{checkins: db.Collection(CheckinsCollection), resenas: db.Collection(Collection), utiles: db.Collection(UtilesCollection)}
}

// Pos es la posición de un cursor: el valor del orden y el _id que desempata.
type Pos struct {
	Fecha time.Time
	Num   int
	ID    bson.ObjectID
}

func despues(campo string, valor any, id bson.ObjectID) bson.D {
	return bson.D{{Key: "$or", Value: bson.A{
		bson.D{{Key: campo, Value: bson.D{{Key: "$lt", Value: valor}}}},
		bson.D{{Key: campo, Value: valor}, {Key: "_id", Value: bson.D{{Key: "$lt", Value: id}}}},
	}}}
}

func listar[T any](ctx context.Context, col *mongo.Collection, filter, sort bson.D, limit int) ([]T, error) {
	cur, err := col.Find(ctx, filter, options.Find().SetSort(sort).SetLimit(int64(limit)))
	if err != nil {
		return nil, err
	}
	out := []T{}
	return out, cur.All(ctx, &out)
}

// --- check-ins ---

// InsertarCheckin guarda el check-in; el segundo válido del día choca con ux_checkin_dia.
func (r *Repository) InsertarCheckin(ctx context.Context, c Checkin) error {
	_, err := r.checkins.InsertOne(ctx, c)
	if mongo.IsDuplicateKeyError(err) {
		return ErrDuplicado
	}
	return err
}

// UltimoCheckinValido trae el check-in válido más reciente de la persona (ix_mis_checkins).
func (r *Repository) UltimoCheckinValido(ctx context.Context, usuario bson.ObjectID) (Checkin, bool, error) {
	var c Checkin
	err := r.checkins.FindOne(ctx, bson.D{{Key: "usuario_id", Value: usuario}, {Key: "valido", Value: true}},
		options.FindOne().SetSort(bson.D{{Key: "creado_en", Value: -1}})).Decode(&c)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Checkin{}, false, nil
	}
	return c, err == nil, err
}

// Checkin trae un check-in por id.
func (r *Repository) Checkin(ctx context.Context, id bson.ObjectID) (Checkin, error) {
	var c Checkin
	err := r.checkins.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&c)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Checkin{}, ErrNoEncontrado
	}
	return c, err
}

// MisCheckins pagina los check-ins de la persona, del más reciente al más viejo.
func (r *Repository) MisCheckins(ctx context.Context, usuario bson.ObjectID, after *Pos, limit int) ([]Checkin, error) {
	filter := bson.D{{Key: "usuario_id", Value: usuario}}
	if after != nil {
		filter = append(filter, despues("creado_en", after.Fecha, after.ID)...)
	}
	return listar[Checkin](ctx, r.checkins, filter, bson.D{{Key: "creado_en", Value: -1}, {Key: "_id", Value: -1}}, limit+1)
}

// PorResenar son los check-ins válidos sin reseña desde `desde` (ix_por_resenar).
func (r *Repository) PorResenar(ctx context.Context, usuario bson.ObjectID, desde time.Time, limit int) ([]Checkin, error) {
	filter := bson.D{
		{Key: "usuario_id", Value: usuario}, {Key: "valido", Value: true},
		{Key: "creado_en", Value: bson.D{{Key: "$gte", Value: desde}}},
		{Key: "resena_id", Value: bson.D{{Key: "$exists", Value: false}}},
	}
	return listar[Checkin](ctx, r.checkins, filter, bson.D{{Key: "creado_en", Value: -1}}, limit)
}

// MarcarResenado enlaza el check-in con su reseña.
func (r *Repository) MarcarResenado(ctx context.Context, checkin, resena bson.ObjectID) error {
	_, err := r.checkins.UpdateOne(ctx, bson.D{{Key: "_id", Value: checkin}}, bson.D{{Key: "$set", Value: bson.D{{Key: "resena_id", Value: resena}}}})
	return err
}

// Llegadas cuenta los check-ins válidos del negocio en [desde, hasta) (ix_checkins_negocio).
func (r *Repository) Llegadas(ctx context.Context, negocio bson.ObjectID, desde, hasta time.Time) (int64, error) {
	return r.checkins.CountDocuments(ctx, bson.D{
		{Key: "negocio_id", Value: negocio},
		{Key: "creado_en", Value: bson.D{{Key: "$gte", Value: desde}, {Key: "$lt", Value: hasta}}},
		{Key: "valido", Value: true},
	})
}

// BorrarCheckins borra los check-ins de la persona; con soloSinResena conserva los reseñados.
func (r *Repository) BorrarCheckins(ctx context.Context, usuario bson.ObjectID, soloSinResena bool) error {
	filter := bson.D{{Key: "usuario_id", Value: usuario}}
	if soloSinResena {
		filter = append(filter, bson.E{Key: "resena_id", Value: bson.D{{Key: "$exists", Value: false}}})
	}
	_, err := r.checkins.DeleteMany(ctx, filter)
	return err
}

// CheckinsDe trae todos los check-ins de la persona (exportación ARCO).
func (r *Repository) CheckinsDe(ctx context.Context, usuario bson.ObjectID) ([]Checkin, error) {
	return listar[Checkin](ctx, r.checkins, bson.D{{Key: "usuario_id", Value: usuario}}, bson.D{{Key: "creado_en", Value: -1}}, 0)
}

// --- reseñas ---

// Insertar guarda la reseña; una segunda para el mismo check-in choca con ux_resena_checkin.
func (r *Repository) Insertar(ctx context.Context, res Resena) error {
	_, err := r.resenas.InsertOne(ctx, res)
	if mongo.IsDuplicateKeyError(err) {
		return ErrDuplicado
	}
	return err
}

// RecientesDe cuenta las reseñas de la persona en el negocio desde `desde`, incluidas las
// eliminadas (ix_mis_resenas): así borrar y volver a reseñar no salta la regla de 30 días.
func (r *Repository) RecientesDe(ctx context.Context, usuario, negocio bson.ObjectID, desde time.Time) (int64, error) {
	return r.resenas.CountDocuments(ctx, bson.D{
		{Key: "usuario_id", Value: usuario},
		{Key: "creado_en", Value: bson.D{{Key: "$gte", Value: desde}}},
		{Key: "negocio_id", Value: negocio},
	})
}

// Buscar trae una reseña por id.
func (r *Repository) Buscar(ctx context.Context, id bson.ObjectID) (Resena, error) {
	var res Resena
	err := r.resenas.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&res)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Resena{}, ErrNoEncontrado
	}
	return res, err
}

// Transicion aplica `set` (más `version + 1` y `actualizado_en`) si la reseña cumple el filtro y
// devuelve el documento de antes. ErrNoAplica si no coincide.
func (r *Repository) Transicion(ctx context.Context, filter, set bson.D, now time.Time) (Resena, error) {
	set = append(set, bson.E{Key: "actualizado_en", Value: now})
	update := bson.D{{Key: "$set", Value: set}, {Key: "$inc", Value: bson.D{{Key: "version", Value: 1}}}}
	var antes Resena
	err := r.resenas.FindOneAndUpdate(ctx, filter, update, options.FindOneAndUpdate().SetReturnDocument(options.Before)).Decode(&antes)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Resena{}, ErrNoAplica
	}
	return antes, err
}

// Revertir deja la reseña como estaba si nadie la cambió después de la transición.
func (r *Repository) Revertir(ctx context.Context, antes Resena) error {
	_, err := r.resenas.ReplaceOne(ctx, bson.D{{Key: "_id", Value: antes.ID}, {Key: "version", Value: antes.Version + 1}}, antes)
	return err
}

// FiltroPublico es lo que acepta la lista pública de un negocio.
type FiltroPublico struct {
	Estrellas int
	Aspecto   string
	ConFotos  bool
	Q         string
	Utiles    bool
}

// Publicas pagina las reseñas publicadas del negocio.
func (r *Repository) Publicas(ctx context.Context, negocio bson.ObjectID, f FiltroPublico, after *Pos, limit int) ([]Resena, error) {
	filter := bson.D{{Key: "negocio_id", Value: negocio}, {Key: "estado", Value: EstadoPublicada}}
	if f.Estrellas > 0 {
		filter = append(filter, bson.E{Key: "calificacion", Value: f.Estrellas})
	}
	if f.Aspecto != "" {
		filter = append(filter, bson.E{Key: "aspectos." + f.Aspecto, Value: bson.D{{Key: "$gte", Value: 4}}})
	}
	if f.ConFotos {
		filter = append(filter, bson.E{Key: "fotos.0", Value: bson.D{{Key: "$exists", Value: true}}})
	}
	if f.Q != "" {
		filter = append(filter, bson.E{Key: "$text", Value: bson.D{{Key: "$search", Value: f.Q}, {Key: "$language", Value: "spanish"}}})
	}
	campo, valor := "creado_en", any(nil)
	if after != nil {
		valor = after.Fecha
	}
	if f.Utiles {
		campo = "util"
		if after != nil {
			valor = after.Num
		}
	}
	if after != nil {
		filter = append(filter, despues(campo, valor, after.ID)...)
	}
	return listar[Resena](ctx, r.resenas, filter, bson.D{{Key: campo, Value: -1}, {Key: "_id", Value: -1}}, limit+1)
}

// Mias pagina las reseñas de la persona (sin las eliminadas).
func (r *Repository) Mias(ctx context.Context, usuario bson.ObjectID, after *Pos, limit int) ([]Resena, error) {
	filter := bson.D{{Key: "usuario_id", Value: usuario}, {Key: "estado", Value: bson.D{{Key: "$ne", Value: EstadoEliminada}}}}
	if after != nil {
		filter = append(filter, despues("creado_en", after.Fecha, after.ID)...)
	}
	return listar[Resena](ctx, r.resenas, filter, bson.D{{Key: "creado_en", Value: -1}, {Key: "_id", Value: -1}}, limit+1)
}

// AgregarReporte agrega el reporte si la reseña está publicada, no es de quien reporta y esa
// persona no la reportó ya. Devuelve el documento después del cambio.
func (r *Repository) AgregarReporte(ctx context.Context, id, por bson.ObjectID, rep Reporte) (Resena, error) {
	filter := bson.D{
		{Key: "_id", Value: id}, {Key: "estado", Value: EstadoPublicada},
		{Key: "usuario_id", Value: bson.D{{Key: "$ne", Value: por}}},
		{Key: "reportes.por", Value: bson.D{{Key: "$ne", Value: por}}},
	}
	var despuesDoc Resena
	err := r.resenas.FindOneAndUpdate(ctx, filter, bson.D{{Key: "$push", Value: bson.D{{Key: "reportes", Value: rep}}}},
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&despuesDoc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Resena{}, ErrNoAplica
	}
	return despuesDoc, err
}

// MarcarUtil agrega o quita la marca de la persona y mueve `util` solo si cambió. Devuelve el
// conteo vigente.
func (r *Repository) MarcarUtil(ctx context.Context, resena, usuario bson.ObjectID, util bool, now time.Time) (int, error) {
	cambio := 0
	if util {
		_, err := r.utiles.InsertOne(ctx, Util{ID: bson.NewObjectID(), ResenaID: resena, UsuarioID: usuario, CreadoEn: now})
		switch {
		case err == nil:
			cambio = 1
		case !mongo.IsDuplicateKeyError(err):
			return 0, err
		}
	} else {
		res, err := r.utiles.DeleteOne(ctx, bson.D{{Key: "resena_id", Value: resena}, {Key: "usuario_id", Value: usuario}})
		if err != nil {
			return 0, err
		}
		cambio = -int(res.DeletedCount)
	}
	var doc Resena
	err := r.resenas.FindOneAndUpdate(ctx, bson.D{{Key: "_id", Value: resena}}, bson.D{{Key: "$inc", Value: bson.D{{Key: "util", Value: cambio}}}},
		options.FindOneAndUpdate().SetReturnDocument(options.After).SetProjection(bson.D{{Key: "util", Value: 1}})).Decode(&doc)
	return doc.Util, err
}

// FijarRespuesta escribe la respuesta del negocio sin subir `version` (no es un cambio del autor).
func (r *Repository) FijarRespuesta(ctx context.Context, id bson.ObjectID, resp Respuesta) error {
	_, err := r.resenas.UpdateOne(ctx, bson.D{{Key: "_id", Value: id}}, bson.D{{Key: "$set", Value: bson.D{{Key: "respuesta_negocio", Value: resp}}}})
	return err
}

// EstadoRespuesta cambia el estado de la respuesta (decisión de la cola de nv-43).
func (r *Repository) EstadoRespuesta(ctx context.Context, id bson.ObjectID, estado string) error {
	_, err := r.resenas.UpdateOne(ctx, bson.D{{Key: "_id", Value: id}, {Key: "respuesta_negocio.estado", Value: EstadoEnModeracion}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "respuesta_negocio.estado", Value: estado}}}})
	return err
}

// Grupo es una suma de reseñas publicadas: cuántas, estrellas y aspectos.
type Grupo struct {
	Clave         string `bson:"_id"`
	N             int64  `bson:"n"`
	Estrellas     int64  `bson:"estrellas"`
	Sabor         int64  `bson:"sabor"`
	Atencion      int64  `bson:"atencion"`
	Ambiente      int64  `bson:"ambiente"`
	Espera        int64  `bson:"espera"`
	PrecioCalidad int64  `bson:"precio_calidad"`
}

// Tramo es un intervalo con nombre para agrupar: la reseña cae en el primero que la contiene.
type Tramo struct {
	Clave string
	Desde time.Time
}

// Sumar agrupa las reseñas publicadas del negocio creadas en [desde, hasta) por tramo (los
// tramos van del más reciente al más viejo) y, si porEstrellas, también por calificación.
func (r *Repository) Sumar(ctx context.Context, negocio bson.ObjectID, desde, hasta time.Time, tramos []Tramo, porEstrellas bool) ([]Grupo, error) {
	ramas := bson.A{}
	for _, t := range tramos {
		ramas = append(ramas, bson.D{{Key: "case", Value: bson.D{{Key: "$gte", Value: bson.A{"$creado_en", t.Desde}}}}, {Key: "then", Value: t.Clave}})
	}
	clave := any(bson.D{{Key: "$switch", Value: bson.D{{Key: "branches", Value: ramas}, {Key: "default", Value: "fuera"}}}})
	if porEstrellas {
		clave = bson.D{{Key: "$concat", Value: bson.A{clave, ":", bson.D{{Key: "$toString", Value: "$calificacion"}}}}}
	}
	suma := func(campo string) bson.D { return bson.D{{Key: "$sum", Value: campo}} }
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.D{
			{Key: "negocio_id", Value: negocio}, {Key: "estado", Value: EstadoPublicada},
			{Key: "creado_en", Value: bson.D{{Key: "$gte", Value: desde}, {Key: "$lt", Value: hasta}}},
		}}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: clave}, {Key: "n", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "estrellas", Value: suma("$calificacion")}, {Key: "sabor", Value: suma("$aspectos.sabor")},
			{Key: "atencion", Value: suma("$aspectos.atencion")}, {Key: "ambiente", Value: suma("$aspectos.ambiente")},
			{Key: "espera", Value: suma("$aspectos.espera")}, {Key: "precio_calidad", Value: suma("$aspectos.precio_calidad")},
		}}},
	}
	cur, err := r.resenas.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	out := []Grupo{}
	return out, cur.All(ctx, &out)
}

// SinResponder cuenta las publicadas del negocio sin respuesta.
func (r *Repository) SinResponder(ctx context.Context, negocio bson.ObjectID) (int64, error) {
	return r.resenas.CountDocuments(ctx, bson.D{
		{Key: "negocio_id", Value: negocio}, {Key: "estado", Value: EstadoPublicada},
		{Key: "respuesta_negocio", Value: bson.D{{Key: "$exists", Value: false}}},
	})
}

// Rechazadas cuenta las reseñas del negocio rechazadas por moderación desde `desde`.
func (r *Repository) Rechazadas(ctx context.Context, negocio bson.ObjectID, desde time.Time) (int64, error) {
	return r.resenas.CountDocuments(ctx, bson.D{
		{Key: "negocio_id", Value: negocio}, {Key: "estado", Value: EstadoRechazada},
		{Key: "moderada_en", Value: bson.D{{Key: "$gte", Value: desde}}},
	})
}

// Anonimizar quita el autor y las fotos de sus reseñas (texto y calificación se quedan, nv-05)
// y lo borra de los reportes que hizo.
func (r *Repository) Anonimizar(ctx context.Context, usuario bson.ObjectID) error {
	if _, err := r.resenas.UpdateMany(ctx, bson.D{{Key: "usuario_id", Value: usuario}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "usuario_id", Value: nil}, {Key: "fotos", Value: bson.A{}}}}}); err != nil {
		return err
	}
	_, err := r.resenas.UpdateMany(ctx, bson.D{{Key: "reportes.por", Value: usuario}},
		bson.D{{Key: "$unset", Value: bson.D{{Key: "reportes.$[r].por", Value: ""}}}},
		options.UpdateMany().SetArrayFilters([]any{bson.D{{Key: "r.por", Value: usuario}}}))
	return err
}

// BorrarUtiles borra las marcas de «me fue útil» de la persona (los contadores quedan).
func (r *Repository) BorrarUtiles(ctx context.Context, usuario bson.ObjectID) error {
	_, err := r.utiles.DeleteMany(ctx, bson.D{{Key: "usuario_id", Value: usuario}})
	return err
}

// ResenasDe trae las reseñas que escribió la persona (exportación ARCO).
func (r *Repository) ResenasDe(ctx context.Context, usuario bson.ObjectID) ([]Resena, error) {
	return listar[Resena](ctx, r.resenas, bson.D{{Key: "usuario_id", Value: usuario}}, bson.D{{Key: "creado_en", Value: -1}}, 0)
}

package postales

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
	ErrNoEncontrado = errors.New("postales: no encontrado")
	ErrDuplicado    = errors.New("postales: duplicado")
	ErrNoAplica     = errors.New("postales: el documento no cumple la condición")
)

// Repository accede a `postales`, `recuerdos` y `momentos`.
type Repository struct {
	postales  *mongo.Collection
	recuerdos *mongo.Collection
	momentos  *mongo.Collection
}

// NewRepository recibe la base por constructor.
func NewRepository(db *mongo.Database) *Repository {
	return &Repository{postales: db.Collection(Collection), recuerdos: db.Collection(RecuerdosCollection), momentos: db.Collection(MomentosCollection)}
}

// Pos es la posición del cursor: fecha y _id del último elemento.
type Pos struct {
	Fecha time.Time
	ID    bson.ObjectID
}

func despues(campo string, p *Pos) bson.E {
	return bson.E{Key: "$or", Value: bson.A{
		bson.D{{Key: campo, Value: bson.D{{Key: "$lt", Value: p.Fecha}}}},
		bson.D{{Key: campo, Value: p.Fecha}, {Key: "_id", Value: bson.D{{Key: "$lt", Value: p.ID}}}},
	}}
}

func listar[T any](ctx context.Context, col *mongo.Collection, filter, sort bson.D, limit int) ([]T, error) {
	opts := options.Find().SetSort(sort)
	if limit > 0 {
		opts.SetLimit(int64(limit))
	}
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

func uno[T any](ctx context.Context, col *mongo.Collection, filter bson.D) (T, error) {
	var out T
	err := col.FindOne(ctx, filter).Decode(&out)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return out, ErrNoEncontrado
	}
	return out, err
}

func porFecha(campo string) bson.D {
	return bson.D{{Key: campo, Value: -1}, {Key: "_id", Value: -1}}
}

// --- postales ---

// InsertarPostal guarda una postal nueva; la imagen ya usada = ErrDuplicado.
func (r *Repository) InsertarPostal(ctx context.Context, p Postal) error {
	_, err := r.postales.InsertOne(ctx, p)
	if mongo.IsDuplicateKeyError(err) {
		return ErrDuplicado
	}
	return err
}

// Postal trae una postal por id.
func (r *Repository) Postal(ctx context.Context, id bson.ObjectID) (Postal, error) {
	return uno[Postal](ctx, r.postales, bson.D{{Key: "_id", Value: id}})
}

// PostalesPorIDs trae varias postales (para los momentos de una página).
func (r *Repository) PostalesPorIDs(ctx context.Context, ids []bson.ObjectID) (map[bson.ObjectID]Postal, error) {
	out := map[bson.ObjectID]Postal{}
	if len(ids) == 0 {
		return out, nil
	}
	ps, err := listar[Postal](ctx, r.postales, bson.D{{Key: "_id", Value: bson.D{{Key: "$in", Value: ids}}}}, bson.D{{Key: "_id", Value: 1}}, 0)
	if err != nil {
		return nil, err
	}
	for _, p := range ps {
		out[p.ID] = p
	}
	return out, nil
}

// FiltroAlbum son los filtros de GET /postales.
type FiltroAlbum struct {
	PlanID    *bson.ObjectID
	NegocioID *bson.ObjectID
	Estilo    string
	Desde     time.Time
	Hasta     time.Time
}

// Album pagina las postales del dueño (ix_album), una de más para saber si sigue.
func (r *Repository) Album(ctx context.Context, usuario bson.ObjectID, f FiltroAlbum, after *Pos, limit int) ([]Postal, error) {
	filter := bson.D{{Key: "usuario_id", Value: usuario}}
	if f.PlanID != nil {
		filter = append(filter, bson.E{Key: "plan_id", Value: *f.PlanID})
	}
	if f.NegocioID != nil {
		filter = append(filter, bson.E{Key: "negocio_id", Value: *f.NegocioID})
	}
	if f.Estilo != "" {
		filter = append(filter, bson.E{Key: "estilo", Value: f.Estilo})
	}
	if !f.Desde.IsZero() {
		filter = append(filter, bson.E{Key: "creado_en", Value: bson.D{{Key: "$gte", Value: f.Desde}, {Key: "$lt", Value: f.Hasta}}})
	}
	if after != nil {
		filter = append(filter, despues("creado_en", after))
	}
	return listar[Postal](ctx, r.postales, filter, porFecha("creado_en"), limit+1)
}

// Galeria pagina las postales públicas y listas de un negocio (ix_postales_negocio).
func (r *Repository) Galeria(ctx context.Context, negocio bson.ObjectID, after *Pos, limit int) ([]Postal, error) {
	filter := bson.D{{Key: "negocio_id", Value: negocio}, {Key: "visibilidad", Value: VisPublica}, {Key: "estado", Value: EstadoLista}}
	if after != nil {
		filter = append(filter, despues("creado_en", after))
	}
	return listar[Postal](ctx, r.postales, filter, porFecha("creado_en"), limit+1)
}

// IDsDePlan son las postales del usuario en un plan (para el recuerdo), de la más vieja a la
// más nueva.
func (r *Repository) IDsDePlan(ctx context.Context, usuario, plan bson.ObjectID) ([]bson.ObjectID, error) {
	ps, err := listar[Postal](ctx, r.postales, bson.D{{Key: "usuario_id", Value: usuario}, {Key: "plan_id", Value: plan}},
		bson.D{{Key: "creado_en", Value: 1}, {Key: "_id", Value: 1}}, 0)
	if err != nil {
		return nil, err
	}
	out := make([]bson.ObjectID, 0, len(ps))
	for _, p := range ps {
		out = append(out, p.ID)
	}
	return out, nil
}

// ActualizarPostal aplica `set` si la postal es del dueño y sigue en `version`; sube la versión.
func (r *Repository) ActualizarPostal(ctx context.Context, id, dueno bson.ObjectID, version int, set bson.D) (Postal, error) {
	var out Postal
	err := r.postales.FindOneAndUpdate(ctx,
		bson.D{{Key: "_id", Value: id}, {Key: "usuario_id", Value: dueno}, {Key: "version", Value: version}},
		bson.D{{Key: "$set", Value: set}, {Key: "$inc", Value: bson.D{{Key: "version", Value: 1}}}},
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&out)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return out, ErrNoAplica
	}
	if mongo.IsDuplicateKeyError(err) {
		return out, ErrDuplicado
	}
	return out, err
}

// ModerarPostal cambia el estado de la imagen o del texto (decisión de nv-43) sin tocar la
// versión de la persona. Filtra por el valor anterior para no pisar una edición nueva.
// Devuelve las postales ya cambiadas.
func (r *Repository) ModerarPostal(ctx context.Context, filter bson.D, set bson.D) ([]Postal, error) {
	antes, err := listar[Postal](ctx, r.postales, filter, bson.D{{Key: "_id", Value: 1}}, 0)
	if err != nil || len(antes) == 0 {
		return nil, err
	}
	ids := make([]bson.ObjectID, 0, len(antes))
	for _, p := range antes {
		ids = append(ids, p.ID)
	}
	porID := append(bson.D{{Key: "_id", Value: bson.D{{Key: "$in", Value: ids}}}}, filter...)
	if _, err := r.postales.UpdateMany(ctx, porID, bson.D{{Key: "$set", Value: set}}); err != nil {
		return nil, err
	}
	return listar[Postal](ctx, r.postales, bson.D{{Key: "_id", Value: bson.D{{Key: "$in", Value: ids}}}}, bson.D{{Key: "_id", Value: 1}}, 0)
}

// FijarEstado guarda el estado recalculado de una postal.
func (r *Repository) FijarEstado(ctx context.Context, id bson.ObjectID, estado string, now time.Time) error {
	_, err := r.postales.UpdateOne(ctx, bson.D{{Key: "_id", Value: id}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "estado", Value: estado}, {Key: "actualizado_en", Value: now}}}})
	return err
}

// BorrarPostal borra la postal del dueño en `version` y su momento.
func (r *Repository) BorrarPostal(ctx context.Context, id, dueno bson.ObjectID, version int) error {
	res, err := r.postales.DeleteOne(ctx, bson.D{{Key: "_id", Value: id}, {Key: "usuario_id", Value: dueno}, {Key: "version", Value: version}})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return ErrNoAplica
	}
	_, err = r.momentos.DeleteMany(ctx, bson.D{{Key: "postal_id", Value: id}})
	return err
}

// --- recuerdos ---

// GuardarRecuerdo crea o reemplaza el recuerdo de la persona para el plan (ux_recuerdo).
// Devuelve si ya existía.
func (r *Repository) GuardarRecuerdo(ctx context.Context, rec Recuerdo) (Recuerdo, bool, error) {
	filter := bson.D{{Key: "usuario_id", Value: rec.UsuarioID}, {Key: "plan_id", Value: rec.PlanID}}
	set := bson.D{
		{Key: "titulo", Value: rec.Titulo}, {Key: "fecha", Value: rec.Fecha}, {Key: "paradas", Value: rec.Paradas},
		{Key: "postales_ids", Value: rec.PostalesIDs}, {Key: "gasto_total", Value: rec.GastoTotal}, {Key: "puntos_ganados", Value: rec.PuntosGanados},
		{Key: "tarjeta_imagen_id", Value: rec.TarjetaImagenID}, {Key: "tarjeta_url", Value: rec.TarjetaURL}, {Key: "actualizado_en", Value: rec.ActualizadoEn},
	}
	var antes Recuerdo
	err := r.recuerdos.FindOneAndUpdate(ctx, filter,
		bson.D{{Key: "$set", Value: set}, {Key: "$setOnInsert", Value: bson.D{{Key: "_id", Value: rec.ID}, {Key: "creado_en", Value: rec.CreadoEn}}}},
		options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.Before)).Decode(&antes)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return rec, false, nil
	}
	if mongo.IsDuplicateKeyError(err) {
		// Dos regeneraciones a la vez: la otra insertó primero; esta lo reemplaza.
		return r.GuardarRecuerdo(ctx, rec)
	}
	if err != nil {
		return Recuerdo{}, false, err
	}
	rec.ID, rec.CreadoEn = antes.ID, antes.CreadoEn
	return rec, true, nil
}

// RecuerdoDePlan trae el recuerdo de la persona para un plan.
func (r *Repository) RecuerdoDePlan(ctx context.Context, usuario, plan bson.ObjectID) (Recuerdo, error) {
	return uno[Recuerdo](ctx, r.recuerdos, bson.D{{Key: "usuario_id", Value: usuario}, {Key: "plan_id", Value: plan}})
}

// Recuerdos pagina los recuerdos de la persona por fecha del plan (ix_recuerdos_fecha).
func (r *Repository) Recuerdos(ctx context.Context, usuario bson.ObjectID, desde, hasta time.Time, after *Pos, limit int) ([]Recuerdo, error) {
	filter := bson.D{{Key: "usuario_id", Value: usuario}}
	if !desde.IsZero() {
		filter = append(filter, bson.E{Key: "fecha", Value: bson.D{{Key: "$gte", Value: desde}, {Key: "$lt", Value: hasta}}})
	}
	if after != nil {
		filter = append(filter, despues("fecha", after))
	}
	return listar[Recuerdo](ctx, r.recuerdos, filter, porFecha("fecha"), limit+1)
}

// BorrarRecuerdo borra el recuerdo propio; las postales se quedan.
func (r *Repository) BorrarRecuerdo(ctx context.Context, id, usuario bson.ObjectID) (bool, error) {
	res, err := r.recuerdos.DeleteOne(ctx, bson.D{{Key: "_id", Value: id}, {Key: "usuario_id", Value: usuario}})
	if err != nil {
		return false, err
	}
	return res.DeletedCount > 0, nil
}

// --- momentos ---

// InsertarMomento guarda el momento; una postal con momento = ErrDuplicado.
func (r *Repository) InsertarMomento(ctx context.Context, m Momento) error {
	_, err := r.momentos.InsertOne(ctx, m)
	if mongo.IsDuplicateKeyError(err) {
		return ErrDuplicado
	}
	return err
}

// Momento trae un momento por id.
func (r *Repository) Momento(ctx context.Context, id bson.ObjectID) (Momento, error) {
	return uno[Momento](ctx, r.momentos, bson.D{{Key: "_id", Value: id}})
}

// MomentoDePostal trae el momento de una postal.
func (r *Repository) MomentoDePostal(ctx context.Context, postal bson.ObjectID) (Momento, error) {
	return uno[Momento](ctx, r.momentos, bson.D{{Key: "postal_id", Value: postal}})
}

// Feed pagina los momentos publicados de los autores (ix_feed).
func (r *Repository) Feed(ctx context.Context, autores []bson.ObjectID, after *Pos, limit int) ([]Momento, error) {
	filter := bson.D{
		{Key: "autor_id", Value: bson.D{{Key: "$in", Value: autores}}},
		{Key: "estado", Value: MomentoPublicado},
	}
	if after != nil {
		filter = append(filter, despues("creado_en", after))
	}
	return listar[Momento](ctx, r.momentos, filter, porFecha("creado_en"), limit+1)
}

// DelNegocio pagina los momentos públicos y publicados de un negocio (ix_momentos_negocio).
func (r *Repository) DelNegocio(ctx context.Context, negocio bson.ObjectID, destacados bool, after *Pos, limit int) ([]Momento, error) {
	filter := bson.D{{Key: "negocio_id", Value: negocio}, {Key: "visibilidad", Value: VisPublica}, {Key: "estado", Value: MomentoPublicado}}
	if destacados {
		filter = append(filter, bson.E{Key: "destacado_por_negocio", Value: true})
	}
	if after != nil {
		filter = append(filter, despues("creado_en", after))
	}
	return listar[Momento](ctx, r.momentos, filter, porFecha("creado_en"), limit+1)
}

// ActualizarMomento aplica `set` si el momento es del autor y sigue en `version`.
func (r *Repository) ActualizarMomento(ctx context.Context, id, autor bson.ObjectID, version int, set bson.D) (Momento, error) {
	var out Momento
	err := r.momentos.FindOneAndUpdate(ctx,
		bson.D{{Key: "_id", Value: id}, {Key: "autor_id", Value: autor}, {Key: "version", Value: version}},
		bson.D{{Key: "$set", Value: set}, {Key: "$inc", Value: bson.D{{Key: "version", Value: 1}}}},
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&out)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return out, ErrNoAplica
	}
	return out, err
}

// MarcarMomento cambia las marcas del negocio y devuelve el documento de antes.
func (r *Repository) MarcarMomento(ctx context.Context, id, negocio bson.ObjectID, set bson.D) (Momento, error) {
	var antes Momento
	err := r.momentos.FindOneAndUpdate(ctx,
		bson.D{{Key: "_id", Value: id}, {Key: "negocio_id", Value: negocio}, {Key: "visibilidad", Value: VisPublica}, {Key: "estado", Value: MomentoPublicado}},
		bson.D{{Key: "$set", Value: set}}, options.FindOneAndUpdate().SetReturnDocument(options.Before)).Decode(&antes)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return antes, ErrNoEncontrado
	}
	return antes, err
}

// ModerarMomento aplica la decisión de nv-43 al texto del momento si sigue en moderación.
func (r *Repository) ModerarMomento(ctx context.Context, id bson.ObjectID, estado string, now time.Time) error {
	_, err := r.momentos.UpdateOne(ctx, bson.D{{Key: "_id", Value: id}, {Key: "estado", Value: MomentoEnModeracion}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "estado", Value: estado}, {Key: "actualizado_en", Value: now}}}})
	return err
}

// BorrarMomento borra el momento del autor en `version`.
func (r *Repository) BorrarMomento(ctx context.Context, id, autor bson.ObjectID, version int) error {
	res, err := r.momentos.DeleteOne(ctx, bson.D{{Key: "_id", Value: id}, {Key: "autor_id", Value: autor}, {Key: "version", Value: version}})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return ErrNoAplica
	}
	return nil
}

// --- ARCO ---

// PostalesDe, RecuerdosDe y MomentosDe traen todo lo de la persona (exportación).
func (r *Repository) PostalesDe(ctx context.Context, usuario bson.ObjectID) ([]Postal, error) {
	return listar[Postal](ctx, r.postales, bson.D{{Key: "usuario_id", Value: usuario}}, bson.D{{Key: "creado_en", Value: 1}}, 0)
}

// RecuerdosDe trae los recuerdos de la persona.
func (r *Repository) RecuerdosDe(ctx context.Context, usuario bson.ObjectID) ([]Recuerdo, error) {
	return listar[Recuerdo](ctx, r.recuerdos, bson.D{{Key: "usuario_id", Value: usuario}}, bson.D{{Key: "fecha", Value: 1}}, 0)
}

// MomentosDe trae los momentos de la persona.
func (r *Repository) MomentosDe(ctx context.Context, usuario bson.ObjectID) ([]Momento, error) {
	return listar[Momento](ctx, r.momentos, bson.D{{Key: "autor_id", Value: usuario}}, bson.D{{Key: "creado_en", Value: 1}}, 0)
}

// EtiquetadoEn son los ids de momentos ajenos donde etiquetaron a la persona (ix_etiquetados).
func (r *Repository) EtiquetadoEn(ctx context.Context, usuario bson.ObjectID) ([]bson.ObjectID, error) {
	ms, err := listar[Momento](ctx, r.momentos, bson.D{{Key: "etiquetados", Value: usuario}}, bson.D{{Key: "etiquetados", Value: 1}, {Key: "creado_en", Value: 1}}, 0)
	if err != nil {
		return nil, err
	}
	out := make([]bson.ObjectID, 0, len(ms))
	for _, m := range ms {
		out = append(out, m.ID)
	}
	return out, nil
}

// BorrarDe borra postales, recuerdos y momentos de la persona y la quita de los etiquetados
// ajenos (borrado de cuenta).
func (r *Repository) BorrarDe(ctx context.Context, usuario bson.ObjectID) error {
	if _, err := r.momentos.DeleteMany(ctx, bson.D{{Key: "autor_id", Value: usuario}}); err != nil {
		return err
	}
	if _, err := r.momentos.UpdateMany(ctx, bson.D{{Key: "etiquetados", Value: usuario}},
		bson.D{{Key: "$pull", Value: bson.D{{Key: "etiquetados", Value: usuario}}}}); err != nil {
		return err
	}
	if _, err := r.recuerdos.DeleteMany(ctx, bson.D{{Key: "usuario_id", Value: usuario}}); err != nil {
		return err
	}
	_, err := r.postales.DeleteMany(ctx, bson.D{{Key: "usuario_id", Value: usuario}})
	return err
}

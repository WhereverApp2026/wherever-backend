package insignias

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	platformmongo "github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
)

// Colecciones propias: cuándo se ganó cada insignia (nv-78) y la bitácora de puntos (nv-79).
const (
	Collection      = "insignias_usuarios"
	MovimientosColl = "puntos_movimientos"
	idxMovimiento   = "ux_movimiento"
	idxInsignia     = "ux_insignia_usuario"
)

// Colecciones que se leen. Son las de cada módulo; aquí van como constantes porque postales y
// amistades dependen de personas, que usa este paquete.
// TestInsignias_ColeccionesIgualesALosModulos (Integration) evita que se desalineen.
const (
	ColUsuarios  = "usuarios"
	ColPlanes    = "planes"
	ColResenas   = "resenas"
	ColCheckins  = "checkins"
	ColPostales  = "postales"
	ColAmistades = "amistades"
	ColGrupos    = "grupos"
	ColNegocios  = "negocios"
)

// Valores que cuentan en cada colección.
const (
	UsuarioActivo      = "activo"
	ResenaPublicada    = "publicada"
	PostalLista        = "lista"
	CategoriaCafeteria = "cafeteria"
)

// Registro es un documento de `insignias_usuarios`.
type Registro struct {
	ID        bson.ObjectID `bson:"_id"`
	UsuarioID bson.ObjectID `bson:"usuario_id"`
	Insignia  string        `bson:"insignia"`
	GanadaEn  time.Time     `bson:"ganada_en"`
}

// Movimiento es un documento de `puntos_movimientos`: una vez por persona, motivo y referencia.
type Movimiento struct {
	ID        bson.ObjectID `bson:"_id"`
	UsuarioID bson.ObjectID `bson:"usuario_id"`
	Motivo    string        `bson:"motivo"`
	RefID     string        `bson:"ref_id"`
	Puntos    int           `bson:"puntos"`
	CreadoEn  time.Time     `bson:"creado_en"`
}

// Indexes: una insignia por persona y un movimiento por persona, motivo y referencia.
func Indexes() []platformmongo.IndexSet {
	return []platformmongo.IndexSet{
		{Collection: Collection, Models: []mongo.IndexModel{
			{Keys: bson.D{{Key: "usuario_id", Value: 1}, {Key: "insignia", Value: 1}}, Options: options.Index().SetName(idxInsignia).SetUnique(true)},
		}},
		{Collection: MovimientosColl, Models: []mongo.IndexModel{
			{Keys: bson.D{{Key: "usuario_id", Value: 1}, {Key: "motivo", Value: 1}, {Key: "ref_id", Value: 1}}, Options: options.Index().SetName(idxMovimiento).SetUnique(true)},
		}},
	}
}

// Visita es un check-in válido.
type Visita struct {
	ID        bson.ObjectID `bson:"_id"`
	NegocioID bson.ObjectID `bson:"negocio_id"`
	Dia       string        `bson:"dia"`
}

// PlanCreado es un plan de la persona, en cualquier estado (borrar no resta).
type PlanCreado struct {
	ID       bson.ObjectID `bson:"_id"`
	CreadoEn time.Time     `bson:"creado_en"`
}

// Actividad es lo que la persona ya hizo y da insignias o puntos.
type Actividad struct {
	Visitas    []Visita
	Cafeterias int
	// Resenas son los check-ins de las reseñas publicadas: una reseña por visita (ux_resena_checkin).
	Resenas  []bson.ObjectID
	Postales []bson.ObjectID
	Planes   []PlanCreado
	Grupos   int
}

// Repository lee la actividad y guarda insignias, movimientos y nivel.
type Repository struct {
	db *mongo.Database
}

// NewRepository recibe la base por constructor.
func NewRepository(db *mongo.Database) *Repository { return &Repository{db: db} }

func ids(ctx context.Context, col *mongo.Collection, filtro bson.D, campo string, opts ...options.Lister[options.FindOptions]) ([]bson.ObjectID, error) {
	cur, err := col.Find(ctx, filtro, append([]options.Lister[options.FindOptions]{options.Find().SetProjection(bson.D{{Key: campo, Value: 1}})}, opts...)...)
	if err != nil {
		return nil, err
	}
	var docs []bson.M
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]bson.ObjectID, 0, len(docs))
	for _, d := range docs {
		if id, ok := d[campo].(bson.ObjectID); ok {
			out = append(out, id)
		}
	}
	return out, nil
}

// visitas lee los check-ins válidos y cuenta cuántos de los lugares distintos son cafeterías.
func (r *Repository) visitas(ctx context.Context, userID bson.ObjectID, a *Actividad) error {
	cur, err := r.db.Collection(ColCheckins).Find(ctx, bson.D{{Key: "usuario_id", Value: userID}, {Key: "valido", Value: true}},
		options.Find().SetProjection(bson.D{{Key: "negocio_id", Value: 1}, {Key: "dia", Value: 1}}))
	if err != nil {
		return err
	}
	if err := cur.All(ctx, &a.Visitas); err != nil {
		return err
	}
	lugares := map[bson.ObjectID]bool{}
	distintos := bson.A{}
	for _, v := range a.Visitas {
		if !lugares[v.NegocioID] {
			lugares[v.NegocioID] = true
			distintos = append(distintos, v.NegocioID)
		}
	}
	if len(distintos) == 0 {
		return nil
	}
	n, err := r.db.Collection(ColNegocios).CountDocuments(ctx, bson.D{{Key: "_id", Value: bson.D{{Key: "$in", Value: distintos}}}, {Key: "categoria", Value: CategoriaCafeteria}})
	a.Cafeterias = int(n)
	return err
}

// Actividad lee lo que cuenta, con el índice por persona de cada colección.
func (r *Repository) Actividad(ctx context.Context, userID bson.ObjectID) (Actividad, error) {
	var a Actividad
	if err := r.visitas(ctx, userID, &a); err != nil {
		return a, err
	}
	var err error
	if a.Resenas, err = ids(ctx, r.db.Collection(ColResenas), bson.D{{Key: "usuario_id", Value: userID}, {Key: "estado", Value: ResenaPublicada}}, "checkin_id"); err != nil {
		return a, err
	}
	if a.Postales, err = ids(ctx, r.db.Collection(ColPostales), bson.D{{Key: "usuario_id", Value: userID}, {Key: "estado", Value: PostalLista}}, "_id"); err != nil {
		return a, err
	}
	cur, err := r.db.Collection(ColPlanes).Find(ctx, bson.D{{Key: "dueno_id", Value: userID}},
		options.Find().SetProjection(bson.D{{Key: "creado_en", Value: 1}}).SetSort(bson.D{{Key: "creado_en", Value: 1}, {Key: "_id", Value: 1}}))
	if err != nil {
		return a, err
	}
	if err := cur.All(ctx, &a.Planes); err != nil {
		return a, err
	}
	g, err := r.db.Collection(ColGrupos).CountDocuments(ctx, bson.D{{Key: "organizador_id", Value: userID}, {Key: "eliminado_en", Value: bson.D{{Key: "$exists", Value: false}}}})
	if err != nil {
		return a, err
	}
	a.Grupos = int(g)
	return a, nil
}

// Ganadas devuelve la fecha de cada insignia ya guardada.
func (r *Repository) Ganadas(ctx context.Context, userID bson.ObjectID) (map[string]time.Time, error) {
	cur, err := r.db.Collection(Collection).Find(ctx, bson.D{{Key: "usuario_id", Value: userID}})
	if err != nil {
		return nil, err
	}
	var regs []Registro
	if err := cur.All(ctx, &regs); err != nil {
		return nil, err
	}
	out := make(map[string]time.Time, len(regs))
	for _, g := range regs {
		out[g.Insignia] = g.GanadaEn
	}
	return out, nil
}

// Guardar registra una insignia recién ganada. Si otra petición ya la guardó, gana la
// primera fecha (índice único): no es error.
func (r *Repository) Guardar(ctx context.Context, userID bson.ObjectID, insignia string, ahora time.Time) error {
	_, err := r.db.Collection(Collection).InsertOne(ctx, Registro{ID: bson.NewObjectID(), UsuarioID: userID, Insignia: insignia, GanadaEn: ahora})
	if err != nil && !mongo.IsDuplicateKeyError(err) {
		return err
	}
	return nil
}

// Movimientos devuelve los movimientos de la persona, del más viejo al más nuevo.
func (r *Repository) Movimientos(ctx context.Context, userID bson.ObjectID) ([]Movimiento, error) {
	cur, err := r.db.Collection(MovimientosColl).Find(ctx, bson.D{{Key: "usuario_id", Value: userID}}, options.Find().SetSort(bson.D{{Key: "creado_en", Value: 1}, {Key: "_id", Value: 1}}))
	if err != nil {
		return nil, err
	}
	out := []Movimiento{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Registrar inserta el movimiento. false = ya existía (otra petición o una conciliación
// anterior lo guardó): no se vuelve a sumar.
func (r *Repository) Registrar(ctx context.Context, m Movimiento) (bool, error) {
	_, err := r.db.Collection(MovimientosColl).InsertOne(ctx, m)
	if mongo.IsDuplicateKeyError(err) {
		return false, nil
	}
	return err == nil, err
}

// perfilPuntos es lo que la conciliación lee y escribe en `usuarios`.
type perfilPuntos struct {
	Puntos      int    `bson:"puntos"`
	Nivel       string `bson:"nivel"`
	NivelNumero int    `bson:"nivel_numero"`
	Racha       int    `bson:"racha_semanas"`
}

var proyeccionPuntos = bson.D{{Key: "puntos", Value: 1}, {Key: "nivel", Value: 1}, {Key: "nivel_numero", Value: 1}, {Key: "racha_semanas", Value: 1}}

// SumarPuntos suma con `$inc` y devuelve el perfil ya sumado. Sin puntos solo lo lee.
func (r *Repository) SumarPuntos(ctx context.Context, userID bson.ObjectID, puntos int) (perfilPuntos, error) {
	var p perfilPuntos
	col := r.db.Collection(ColUsuarios)
	filtro := bson.D{{Key: "_id", Value: userID}}
	if puntos == 0 {
		err := col.FindOne(ctx, filtro, options.FindOne().SetProjection(proyeccionPuntos)).Decode(&p)
		return p, err
	}
	err := col.FindOneAndUpdate(ctx, filtro, bson.D{{Key: "$inc", Value: bson.D{{Key: "puntos", Value: puntos}}}},
		options.FindOneAndUpdate().SetReturnDocument(options.After).SetProjection(proyeccionPuntos)).Decode(&p)
	return p, err
}

// GuardarNivel escribe nivel y racha solo si los puntos siguen siendo los leídos: si otra
// conciliación sumó en medio, ella escribe el nivel que corresponde (no se pisa con uno viejo).
func (r *Repository) GuardarNivel(ctx context.Context, userID bson.ObjectID, p perfilPuntos) error {
	_, err := r.db.Collection(ColUsuarios).UpdateOne(ctx, bson.D{{Key: "_id", Value: userID}, {Key: "puntos", Value: p.Puntos}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "nivel", Value: p.Nivel}, {Key: "nivel_numero", Value: p.NivelNumero}, {Key: "racha_semanas", Value: p.Racha}}}})
	return err
}

// Activos devuelve hasta `limite` personas activas después de `desde` (trabajo nocturno).
func (r *Repository) Activos(ctx context.Context, desde bson.ObjectID, limite int64) ([]bson.ObjectID, error) {
	return ids(ctx, r.db.Collection(ColUsuarios), bson.D{{Key: "_id", Value: bson.D{{Key: "$gt", Value: desde}}}, {Key: "estado", Value: UsuarioActivo}}, "_id",
		options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}).SetLimit(limite))
}

// BorrarDe borra las insignias de la persona (ARCO).
func (r *Repository) BorrarDe(ctx context.Context, userID bson.ObjectID) error {
	_, err := r.db.Collection(Collection).DeleteMany(ctx, bson.D{{Key: "usuario_id", Value: userID}})
	return err
}

// BorrarMovimientosDe borra la bitácora de puntos de la persona (ARCO).
func (r *Repository) BorrarMovimientosDe(ctx context.Context, userID bson.ObjectID) error {
	_, err := r.db.Collection(MovimientosColl).DeleteMany(ctx, bson.D{{Key: "usuario_id", Value: userID}})
	return err
}

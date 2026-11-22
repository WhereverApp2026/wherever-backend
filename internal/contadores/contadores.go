// Package contadores cuenta lo que una persona tiene en la app para su perfil (nv-81): planes,
// amigos y reseñas, y por cada amigo de la lista cuántos planes tiene y si está planeando
// ahora. Lee las colecciones por nombre y no importa sus módulos: planes, reseñas y amistades
// lo usan para invalidar, y personas para mostrar (un import haría ciclo).
//
// Presupuesto: los contadores de una persona se cachean 5 min en Redis bajo
// `cache:perfil_contadores:{usuario}` (constitución §6) y se invalidan con un DEL al crear o
// borrar un plan, una reseña o una amistad. Lo que cambia sin pasar por esos services (una
// reseña que sale de moderación) se ve al vencer la llave.
package contadores

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/WhereverApp2026/wherever-backend/internal/platform/cache"
)

// Entidad y vida de la caché (`cache:perfil_contadores:{usuario}`).
const (
	Entidad = "perfil_contadores"
	Vida    = 5 * time.Minute
)

// Nombres y valores de las colecciones que se cuentan; contadores_test los compara con los de
// cada módulo.
const (
	ColPlanes       = "planes"
	ColAmistades    = "amistades"
	ColResenas      = "resenas"
	ColVotaciones   = "votaciones"
	ColGrupos       = "grupos"
	PlanActivo      = "activo"
	PlanPublico     = "publico"
	AmistadAceptada = "aceptada"
	ResenaVisible   = "publicada"
	VotoAbierto     = "abierta"
	Confirmado      = "confirmado"
	// maxVotaciones acota las votaciones abiertas que se leen para «planeando ahora».
	maxVotaciones = 1000
)

// Contadores son los totales de una persona. PlanesPublicos es lo que ve otra persona (solo
// cuenta lo que su pestaña puede mostrar); Planes, lo que ve la dueña.
type Contadores struct {
	Planes         int64 `json:"planes"`
	PlanesPublicos int64 `json:"planes_publicos"`
	Amigos         int64 `json:"amigos"`
	Resenas        int64 `json:"resenas"`
}

// Amigo son los datos de un amigo para GET /amistades: sus planes activos públicos (los que
// muestra su pestaña) y si tiene un plan activo para hoy o un grupo con votación abierta.
type Amigo struct {
	Planes         int64
	PlaneandoAhora bool
}

// Repository cuenta con CountDocuments y agregaciones sobre índices por persona.
type Repository struct {
	planes, amistades, resenas, votaciones, grupos *mongo.Collection
}

// NewRepository recibe la base de datos.
func NewRepository(db *mongo.Database) *Repository {
	return &Repository{
		planes: db.Collection(ColPlanes), amistades: db.Collection(ColAmistades), resenas: db.Collection(ColResenas),
		votaciones: db.Collection(ColVotaciones), grupos: db.Collection(ColGrupos),
	}
}

// Contar usa ix_dueno_estado_fecha, ix_de_estado, ix_para_estado e ix_mis_resenas.
func (r *Repository) Contar(ctx context.Context, id bson.ObjectID) (Contadores, error) {
	var out Contadores
	activos := bson.D{{Key: "dueno_id", Value: id}, {Key: "estado", Value: PlanActivo}}
	consultas := []struct {
		col    *mongo.Collection
		filtro bson.D
		dest   *int64
	}{
		{r.planes, activos, &out.Planes},
		{r.planes, append(activos, bson.E{Key: "visibilidad", Value: PlanPublico}), &out.PlanesPublicos},
		{r.resenas, bson.D{{Key: "usuario_id", Value: id}, {Key: "estado", Value: ResenaVisible}}, &out.Resenas},
	}
	for _, q := range consultas {
		n, err := q.col.CountDocuments(ctx, q.filtro)
		if err != nil {
			return Contadores{}, err
		}
		*q.dest = n
	}
	for _, lado := range []string{"de", "para"} {
		n, err := r.amistades.CountDocuments(ctx, bson.D{{Key: lado, Value: id}, {Key: "estado", Value: AmistadAceptada}})
		if err != nil {
			return Contadores{}, err
		}
		out.Amigos += n
	}
	return out, nil
}

// DeAmigos da los planes activos y si planea ahora cada uno de ids; hoy es [desde, hasta).
func (r *Repository) DeAmigos(ctx context.Context, ids []bson.ObjectID, desde, hasta, now time.Time) (map[bson.ObjectID]Amigo, error) {
	out := make(map[bson.ObjectID]Amigo, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	if err := r.planesDe(ctx, ids, desde, hasta, out); err != nil {
		return nil, err
	}
	votando, err := r.votando(ctx, ids, now)
	if err != nil {
		return nil, err
	}
	for id := range votando {
		a := out[id]
		a.PlaneandoAhora = true
		out[id] = a
	}
	return out, nil
}

// planesDe agrupa los planes activos por dueño (ix_dueno_estado_fecha): cuenta los públicos y
// mira la fecha de todos.
func (r *Repository) planesDe(ctx context.Context, ids []bson.ObjectID, desde, hasta time.Time, out map[bson.ObjectID]Amigo) error {
	hoy := bson.D{{Key: "$and", Value: bson.A{
		bson.D{{Key: "$gte", Value: bson.A{"$fecha", desde}}},
		bson.D{{Key: "$lt", Value: bson.A{"$fecha", hasta}}},
	}}}
	pipe := mongo.Pipeline{
		{{Key: "$match", Value: bson.D{{Key: "dueno_id", Value: bson.D{{Key: "$in", Value: ids}}}, {Key: "estado", Value: PlanActivo}}}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$dueno_id"},
			{Key: "n", Value: bson.D{{Key: "$sum", Value: bson.D{{Key: "$cond", Value: bson.A{
				bson.D{{Key: "$eq", Value: bson.A{"$visibilidad", PlanPublico}}}, 1, 0,
			}}}}}},
			{Key: "hoy", Value: bson.D{{Key: "$max", Value: hoy}}},
		}}},
	}
	cur, err := r.planes.Aggregate(ctx, pipe)
	if err != nil {
		return err
	}
	var rows []struct {
		ID  bson.ObjectID `bson:"_id"`
		N   int64         `bson:"n"`
		Hoy bool          `bson:"hoy"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return err
	}
	for _, row := range rows {
		out[row.ID] = Amigo{Planes: row.N, PlaneandoAhora: row.Hoy}
	}
	return nil
}

// votando dice cuáles de ids son integrantes confirmados de un grupo con una votación abierta
// (ix_estado_cierra y el _id de grupos).
func (r *Repository) votando(ctx context.Context, ids []bson.ObjectID, now time.Time) (map[bson.ObjectID]bool, error) {
	out := map[bson.ObjectID]bool{}
	var gids []bson.ObjectID
	err := r.votaciones.Distinct(ctx, "grupo_id",
		bson.D{{Key: "estado", Value: VotoAbierto}, {Key: "cierra_en", Value: bson.D{{Key: "$gt", Value: now}}}}).Decode(&gids)
	if err != nil {
		return nil, err
	}
	if len(gids) == 0 {
		return out, nil
	}
	gids = gids[:min(len(gids), maxVotaciones)]
	buscados := make(map[bson.ObjectID]bool, len(ids))
	for _, id := range ids {
		buscados[id] = true
	}
	cur, err := r.grupos.Find(ctx, bson.D{
		{Key: "_id", Value: bson.D{{Key: "$in", Value: gids}}},
		{Key: "integrantes", Value: bson.D{{Key: "$elemMatch", Value: bson.D{
			{Key: "usuario_id", Value: bson.D{{Key: "$in", Value: ids}}}, {Key: "estado", Value: Confirmado},
		}}}},
	}, options.Find().SetProjection(bson.D{{Key: "integrantes", Value: 1}}))
	if err != nil {
		return nil, err
	}
	var rows []struct {
		Integrantes []struct {
			UsuarioID bson.ObjectID `bson:"usuario_id"`
			Estado    string        `bson:"estado"`
		} `bson:"integrantes"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	for _, g := range rows {
		for _, m := range g.Integrantes {
			if m.Estado == Confirmado && buscados[m.UsuarioID] {
				out[m.UsuarioID] = true
			}
		}
	}
	return out, nil
}

// Fuente es lo que el Service lee; lo cumple *Repository.
type Fuente interface {
	Contar(ctx context.Context, id bson.ObjectID) (Contadores, error)
	DeAmigos(ctx context.Context, ids []bson.ObjectID, desde, hasta, now time.Time) (map[bson.ObjectID]Amigo, error)
}

// Service cachea los contadores. Un *Service nil no cuenta ni invalida.
type Service struct {
	fuente Fuente
	cache  *cache.Cache
	loc    *time.Location
	now    func() time.Time
}

// NewService recibe la fuente, la caché (nil = sin caché) y la zona del «hoy» de los planes.
func NewService(f Fuente, c *cache.Cache, loc *time.Location) *Service {
	if loc == nil {
		loc = time.UTC
	}
	return &Service{fuente: f, cache: c, loc: loc, now: time.Now}
}

// Clave es `cache:perfil_contadores:{usuario}`.
func Clave(id bson.ObjectID) string { return cache.Clave(Entidad, id.Hex()) }

// De da los contadores de la persona, de la caché si están.
func (s *Service) De(ctx context.Context, id bson.ObjectID) (Contadores, error) {
	return cache.Obtener(ctx, s.cache, Clave(id), Vida, func(ctx context.Context) (Contadores, error) {
		return s.fuente.Contar(ctx, id)
	})
}

// DeAmigos da los conteos de la lista de amigos con el «hoy» de la zona; no se cachea.
func (s *Service) DeAmigos(ctx context.Context, ids []bson.ObjectID) (map[bson.ObjectID]Amigo, error) {
	now := s.now()
	local := now.In(s.loc)
	desde := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, s.loc)
	return s.fuente.DeAmigos(ctx, ids, desde, desde.AddDate(0, 0, 1), now)
}

// Invalidar borra los contadores de las personas con un solo DEL; no espera a la petición.
func (s *Service) Invalidar(ctx context.Context, ids ...bson.ObjectID) {
	if s == nil || len(ids) == 0 {
		return
	}
	claves := make([]string, 0, len(ids))
	for _, id := range ids {
		claves = append(claves, Clave(id))
	}
	s.cache.Invalidar(context.WithoutCancel(ctx), claves...)
}

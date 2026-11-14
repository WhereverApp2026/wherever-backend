package reservas

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
	ErrNotFound  = errors.New("reserva no encontrada")
	ErrDuplicate = errors.New("código repetido")
	ErrNoAplica  = errors.New("la reserva cambió")
)

// Indexes son los índices de J5 (los crea scripts/indexes).
func Indexes() []platformmongo.IndexSet {
	return []platformmongo.IndexSet{
		{Collection: Collection, Models: []mongo.IndexModel{
			{Keys: bson.D{{Key: "codigo", Value: 1}}, Options: options.Index().SetName("ux_codigo").SetUnique(true)},
			{Keys: bson.D{{Key: "usuario_id", Value: 1}, {Key: "fecha_hora", Value: -1}}, Options: options.Index().SetName("ix_mis_reservas")},
			{Keys: bson.D{{Key: "negocio_id", Value: 1}, {Key: "fecha_hora", Value: 1}, {Key: "estado", Value: 1}}, Options: options.Index().SetName("ix_agenda_negocio")},
			{
				Keys:    bson.D{{Key: "bloqueo_expira_en", Value: 1}},
				Options: options.Index().SetName("ix_bloqueos").SetPartialFilterExpression(bson.D{{Key: "estado", Value: EstadoBloqueada}}),
			},
			{Keys: bson.D{{Key: "estado", Value: 1}, {Key: "fecha_hora", Value: 1}}, Options: options.Index().SetName("ix_recordatorios")},
		}},
		{Collection: BloqueosCollection, Models: []mongo.IndexModel{
			{Keys: bson.D{{Key: "negocio_id", Value: 1}, {Key: "desde", Value: 1}, {Key: "hasta", Value: 1}}, Options: options.Index().SetName("ix_bloqueo_negocio")},
		}},
	}
}

// Repository lee y escribe `reservas` y `bloqueos_horario`.
type Repository struct {
	col      *mongo.Collection
	bloqueos *mongo.Collection
}

// NewRepository recibe la base por constructor.
func NewRepository(db *mongo.Database) *Repository {
	return &Repository{col: db.Collection(Collection), bloqueos: db.Collection(BloqueosCollection)}
}

// Insert guarda la reserva; ErrDuplicate si el código ya existe (ux_codigo).
func (r *Repository) Insert(ctx context.Context, res *Reserva) error {
	_, err := r.col.InsertOne(ctx, res)
	if mongo.IsDuplicateKeyError(err) {
		return ErrDuplicate
	}
	return err
}

func (r *Repository) one(ctx context.Context, filter bson.D) (Reserva, error) {
	var res Reserva
	err := r.col.FindOne(ctx, filter).Decode(&res)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Reserva{}, ErrNotFound
	}
	return res, err
}

// FindByID trae una reserva.
func (r *Repository) FindByID(ctx context.Context, id bson.ObjectID) (Reserva, error) {
	return r.one(ctx, bson.D{{Key: "_id", Value: id}})
}

// FindByCodigo trae la reserva de ese código exacto en ese negocio.
func (r *Repository) FindByCodigo(ctx context.Context, negocio bson.ObjectID, codigo string) (Reserva, error) {
	return r.one(ctx, bson.D{{Key: "codigo", Value: codigo}, {Key: "negocio_id", Value: negocio}})
}

// DelHorario trae las reservas activas del negocio en ese horario (para contar el cupo).
func (r *Repository) DelHorario(ctx context.Context, negocio bson.ObjectID, hora time.Time) ([]Reserva, error) {
	return findAll[Reserva](ctx, r.col, bson.D{
		{Key: "negocio_id", Value: negocio}, {Key: "fecha_hora", Value: hora},
		{Key: "estado", Value: bson.D{{Key: "$in", Value: activos}}},
	}, options.Find())
}

// DelPeriodo trae las reservas del negocio en [desde, hasta), con filtro de estado opcional,
// ordenadas por hora (a lo más MaxAgendaDia).
func (r *Repository) DelPeriodo(ctx context.Context, negocio bson.ObjectID, desde, hasta time.Time, estados []string) ([]Reserva, error) {
	q := bson.D{{Key: "negocio_id", Value: negocio}, {Key: "fecha_hora", Value: bson.D{{Key: "$gte", Value: desde}, {Key: "$lt", Value: hasta}}}}
	if len(estados) > 0 {
		q = append(q, bson.E{Key: "estado", Value: bson.D{{Key: "$in", Value: estados}}})
	}
	return findAll[Reserva](ctx, r.col, q, options.Find().SetSort(bson.D{{Key: "fecha_hora", Value: 1}, {Key: "_id", Value: 1}}).SetLimit(MaxAgendaDia))
}

// ApartadosVigentes cuenta las reservas `bloqueada` sin vencer de la persona.
func (r *Repository) ApartadosVigentes(ctx context.Context, usuario bson.ObjectID, now time.Time) (int64, error) {
	return r.col.CountDocuments(ctx, bson.D{
		{Key: "usuario_id", Value: usuario}, {Key: "estado", Value: EstadoBloqueada},
		{Key: "bloqueo_expira_en", Value: bson.D{{Key: "$gt", Value: now}}},
	})
}

// HayFuturas dice si el negocio tiene reservas activas desde `desde`.
func (r *Repository) HayFuturas(ctx context.Context, negocio bson.ObjectID, desde time.Time) (bool, error) {
	n, err := r.col.CountDocuments(ctx, bson.D{
		{Key: "negocio_id", Value: negocio}, {Key: "fecha_hora", Value: bson.D{{Key: "$gte", Value: desde}}},
		{Key: "estado", Value: bson.D{{Key: "$in", Value: activos}}},
	}, options.Count().SetLimit(1))
	return n > 0, err
}

// Posicion es el punto de la página de "mis reservas": hora y id de la última fila.
type Posicion struct {
	Hora time.Time
	ID   bson.ObjectID
}

// Mias trae hasta limit+1 reservas de la persona: próximas (desde `now`, de la más cercana a la
// más lejana) o pasadas (antes de `now`, de la más reciente a la más vieja).
func (r *Repository) Mias(ctx context.Context, usuario bson.ObjectID, proximas bool, now time.Time, estado string, after *Posicion, limit int) ([]Reserva, error) {
	op, orden, cmp := "$lt", -1, "$lt"
	if proximas {
		op, orden, cmp = "$gte", 1, "$gt"
	}
	q := bson.D{{Key: "usuario_id", Value: usuario}, {Key: "fecha_hora", Value: bson.D{{Key: op, Value: now}}}}
	if estado != "" {
		q = append(q, bson.E{Key: "estado", Value: estado})
	}
	if after != nil {
		q = append(q, bson.E{Key: "$or", Value: bson.A{
			bson.D{{Key: "fecha_hora", Value: bson.D{{Key: cmp, Value: after.Hora}}}},
			bson.D{{Key: "fecha_hora", Value: after.Hora}, {Key: "_id", Value: bson.D{{Key: cmp, Value: after.ID}}}},
		}})
	}
	return findAll[Reserva](ctx, r.col, q, options.Find().SetSort(bson.D{{Key: "fecha_hora", Value: orden}, {Key: "_id", Value: orden}}).SetLimit(int64(limit+1)))
}

// CambiarSi aplica `set` si la reserva sigue en el estado (y versión, si > 0) esperado; agrega
// la fila al historial y sube la versión. ErrNoAplica si no coincide.
func (r *Repository) CambiarSi(ctx context.Context, id bson.ObjectID, estados []string, version int, set bson.D, cambio *Cambio, unset ...string) (Reserva, error) {
	filter := bson.D{{Key: "_id", Value: id}, {Key: "estado", Value: bson.D{{Key: "$in", Value: estados}}}}
	if version > 0 {
		filter = append(filter, bson.E{Key: "version", Value: version})
	}
	now := time.Now().UTC()
	set = append(set, bson.E{Key: "actualizado_en", Value: now})
	update := bson.D{{Key: "$set", Value: set}, {Key: "$inc", Value: bson.D{{Key: "version", Value: 1}}}}
	if cambio != nil {
		update = append(update, bson.E{Key: "$push", Value: bson.D{{Key: "historial", Value: *cambio}}})
	}
	if len(unset) > 0 {
		u := bson.D{}
		for _, f := range unset {
			u = append(u, bson.E{Key: f, Value: ""})
		}
		update = append(update, bson.E{Key: "$unset", Value: u})
	}
	var res Reserva
	err := r.col.FindOneAndUpdate(ctx, filter, update, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&res)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Reserva{}, ErrNoAplica
	}
	return res, err
}

// CambiarVarias aplica un cambio de estado a todas las que cumplen el filtro (barrido, ARCO,
// baja del negocio). Devuelve cuántas cambió.
func (r *Repository) CambiarVarias(ctx context.Context, filter bson.D, estado, por string, now time.Time) (int64, error) {
	res, err := r.col.UpdateMany(ctx, filter, bson.D{
		{Key: "$set", Value: bson.D{{Key: "estado", Value: estado}, {Key: "actualizado_en", Value: now}}},
		{Key: "$inc", Value: bson.D{{Key: "version", Value: 1}}},
		{Key: "$push", Value: bson.D{{Key: "historial", Value: Cambio{Estado: estado, Fecha: now, Por: por}}}},
	})
	if err != nil {
		return 0, err
	}
	return res.ModifiedCount, nil
}

// QuitarNotas borra la nota de las reservas de la persona (ARCO).
func (r *Repository) QuitarNotas(ctx context.Context, usuario bson.ObjectID) error {
	_, err := r.col.UpdateMany(ctx, bson.D{{Key: "usuario_id", Value: usuario}, {Key: "nota", Value: bson.D{{Key: "$exists", Value: true}}}},
		bson.D{{Key: "$unset", Value: bson.D{{Key: "nota", Value: ""}}}})
	return err
}

// DeUsuario trae todas las reservas de la persona (exportación).
func (r *Repository) DeUsuario(ctx context.Context, usuario bson.ObjectID) ([]Reserva, error) {
	return findAll[Reserva](ctx, r.col, bson.D{{Key: "usuario_id", Value: usuario}}, options.Find().SetSort(bson.D{{Key: "fecha_hora", Value: -1}}))
}

// InsertBloqueo guarda un bloqueo de horas.
func (r *Repository) InsertBloqueo(ctx context.Context, b *Bloqueo) error {
	_, err := r.bloqueos.InsertOne(ctx, b)
	return err
}

// BorrarBloqueo borra el bloqueo si es de ese negocio; ErrNotFound si no.
func (r *Repository) BorrarBloqueo(ctx context.Context, negocio, id bson.ObjectID) error {
	res, err := r.bloqueos.DeleteOne(ctx, bson.D{{Key: "_id", Value: id}, {Key: "negocio_id", Value: negocio}})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return ErrNotFound
	}
	return nil
}

// BloqueosEn trae los bloqueos del negocio que se cruzan con [desde, hasta).
func (r *Repository) BloqueosEn(ctx context.Context, negocio bson.ObjectID, desde, hasta time.Time) ([]Bloqueo, error) {
	return findAll[Bloqueo](ctx, r.bloqueos, bson.D{
		{Key: "negocio_id", Value: negocio},
		{Key: "desde", Value: bson.D{{Key: "$lt", Value: hasta}}},
		{Key: "hasta", Value: bson.D{{Key: "$gt", Value: desde}}},
	}, options.Find())
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

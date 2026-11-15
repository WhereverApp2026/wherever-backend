// Package reportesnegocio guarda los avisos de información incorrecta de un negocio (nv-67,
// US-073) y la cola donde el administrador los marca como corregidos o descartados.
package reportesnegocio

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	platformmongo "github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
)

// Collection guarda los reportes (A2 §4).
const Collection = "reportes_negocio"

// Motivos de un reporte.
const (
	MotivoHorario   = "horario"
	MotivoPrecio    = "precio"
	MotivoCerrado   = "cerrado_permanente"
	MotivoUbicacion = "ubicacion"
	MotivoOtro      = "otro"
)

// Estados de un reporte; solo se resuelve desde abierto.
const (
	EstadoAbierto    = "abierto"
	EstadoCorregido  = "corregido"
	EstadoDescartado = "descartado"
)

// Límites de la spec.
const (
	MaxDetalle = 300
	MaxPorDia  = 10
)

// Errores del repositorio; el service los traduce.
var (
	ErrNoEncontrado = errors.New("reporte no encontrado")
	ErrDuplicado    = errors.New("ya hay un reporte abierto")
	ErrVersion      = errors.New("el reporte cambió")
)

// Reporte es un documento de `reportes_negocio`.
type Reporte struct {
	ID            bson.ObjectID  `bson:"_id"`
	NegocioID     bson.ObjectID  `bson:"negocio_id"`
	UsuarioID     bson.ObjectID  `bson:"usuario_id"`
	Motivo        string         `bson:"motivo"`
	Detalle       string         `bson:"detalle,omitempty"`
	Estado        string         `bson:"estado"`
	ResueltoPor   *bson.ObjectID `bson:"resuelto_por,omitempty"`
	ResueltoEn    *time.Time     `bson:"resuelto_en,omitempty"`
	Version       int            `bson:"version"`
	CreadoEn      time.Time      `bson:"creado_en"`
	ActualizadoEn time.Time      `bson:"actualizado_en"`
}

// Indexes son los índices de `reportes_negocio`. El único creador es scripts/indexes.
func Indexes() []platformmongo.IndexSet {
	return []platformmongo.IndexSet{{
		Collection: Collection,
		Models: []mongo.IndexModel{
			{Keys: bson.D{{Key: "negocio_id", Value: 1}, {Key: "estado", Value: 1}}, Options: options.Index().SetName("ix_reportes_negocio")},
			{Keys: bson.D{{Key: "estado", Value: 1}, {Key: "creado_en", Value: 1}, {Key: "_id", Value: 1}}, Options: options.Index().SetName("ix_reportes_cola")},
			{
				Keys: bson.D{{Key: "negocio_id", Value: 1}, {Key: "usuario_id", Value: 1}},
				Options: options.Index().SetName("ux_reporte_abierto").SetUnique(true).
					SetPartialFilterExpression(bson.D{{Key: "estado", Value: EstadoAbierto}}),
			},
			{Keys: bson.D{{Key: "usuario_id", Value: 1}, {Key: "creado_en", Value: -1}}, Options: options.Index().SetName("ix_reportes_usuario")},
		},
	}}
}

// Repository lee y escribe `reportes_negocio`.
type Repository struct {
	col *mongo.Collection
}

// NewRepository recibe la base por constructor.
func NewRepository(db *mongo.Database) *Repository {
	return &Repository{col: db.Collection(Collection)}
}

// Insertar guarda un reporte; ErrDuplicado si la persona ya tiene uno abierto en ese negocio.
func (r *Repository) Insertar(ctx context.Context, rep *Reporte) error {
	_, err := r.col.InsertOne(ctx, rep)
	if mongo.IsDuplicateKeyError(err) {
		return ErrDuplicado
	}
	return err
}

// DelDia cuenta los reportes que user guardó desde `desde`.
func (r *Repository) DelDia(ctx context.Context, user bson.ObjectID, desde time.Time) (int64, error) {
	return r.col.CountDocuments(ctx, bson.D{{Key: "usuario_id", Value: user}, {Key: "creado_en", Value: bson.D{{Key: "$gte", Value: desde}}}})
}

// Posicion es el último reporte de la página anterior.
type Posicion struct {
	CreadoEn time.Time
	ID       bson.ObjectID
}

// Cola lista los reportes en `estado` (y de un negocio, si se pide) del más viejo al más nuevo;
// trae uno de más para saber si hay siguiente página.
func (r *Repository) Cola(ctx context.Context, estado string, negocio *bson.ObjectID, after *Posicion, limite int) ([]Reporte, error) {
	filter := bson.D{{Key: "estado", Value: estado}}
	if negocio != nil {
		filter = append(filter, bson.E{Key: "negocio_id", Value: *negocio})
	}
	if after != nil {
		filter = append(filter, bson.E{Key: "$or", Value: bson.A{
			bson.D{{Key: "creado_en", Value: bson.D{{Key: "$gt", Value: after.CreadoEn}}}},
			bson.D{{Key: "creado_en", Value: after.CreadoEn}, {Key: "_id", Value: bson.D{{Key: "$gt", Value: after.ID}}}},
		}})
	}
	opts := options.Find().SetSort(bson.D{{Key: "creado_en", Value: 1}, {Key: "_id", Value: 1}}).SetLimit(int64(limite) + 1)
	return r.find(ctx, filter, opts)
}

func (r *Repository) find(ctx context.Context, filter bson.D, opts *options.FindOptionsBuilder) ([]Reporte, error) {
	cur, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	out := []Reporte{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Resolver pasa el reporte de abierto a estado con If-Match. ErrNoEncontrado si no existe,
// ErrResuelto si ya no está abierto y ErrVersion si la versión cambió.
func (r *Repository) Resolver(ctx context.Context, id bson.ObjectID, version int, estado string, admin bson.ObjectID, now time.Time) (Reporte, error) {
	var out Reporte
	err := r.col.FindOneAndUpdate(ctx,
		bson.D{{Key: "_id", Value: id}, {Key: "version", Value: version}, {Key: "estado", Value: EstadoAbierto}},
		bson.D{
			{Key: "$set", Value: bson.D{{Key: "estado", Value: estado}, {Key: "resuelto_por", Value: admin}, {Key: "resuelto_en", Value: now}, {Key: "actualizado_en", Value: now}}},
			{Key: "$inc", Value: bson.D{{Key: "version", Value: 1}}},
		},
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&out)
	if !errors.Is(err, mongo.ErrNoDocuments) {
		return out, err
	}
	cur, err := r.Buscar(ctx, id)
	if err != nil {
		return Reporte{}, err
	}
	if cur.Estado != EstadoAbierto {
		return cur, ErrResuelto
	}
	return cur, ErrVersion
}

// ErrResuelto: el reporte ya no está abierto.
var ErrResuelto = errors.New("el reporte ya se resolvió")

// Buscar trae un reporte por id.
func (r *Repository) Buscar(ctx context.Context, id bson.ObjectID) (Reporte, error) {
	var out Reporte
	err := r.col.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&out)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Reporte{}, ErrNoEncontrado
	}
	return out, err
}

// DelUsuario trae los reportes de user (ARCO), del más nuevo al más viejo.
func (r *Repository) DelUsuario(ctx context.Context, user bson.ObjectID) ([]Reporte, error) {
	return r.find(ctx, bson.D{{Key: "usuario_id", Value: user}}, options.Find().SetSort(bson.D{{Key: "creado_en", Value: -1}}))
}

// BorrarDelUsuario borra todos los reportes de user (abiertos y resueltos).
func (r *Repository) BorrarDelUsuario(ctx context.Context, user bson.ObjectID) error {
	_, err := r.col.DeleteMany(ctx, bson.D{{Key: "usuario_id", Value: user}})
	return err
}

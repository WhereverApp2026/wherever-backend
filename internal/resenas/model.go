// Package resenas son los check-ins y las reseñas verificadas por aspecto (nv-62, hoja J7):
// solo quien llegó al negocio reseña, una vez por visita, y cada reseña publicada mueve
// `negocios.calificacion` de forma atómica.
package resenas

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	platformmongo "github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
)

// Colecciones de nv-62.
const (
	CheckinsCollection = "checkins"
	Collection         = "resenas"
	UtilesCollection   = "resenas_utiles"
)

// Métodos de check-in (sin los de reservas, que llegan con su spec).
const (
	MetodoUbicacion = "ubicacion"
	MetodoQR        = "qr_negocio"
)

// Motivos de un check-in no válido.
const (
	MotivoFueraDeRadio   = "fuera_de_radio"
	MotivoUbicacionFalsa = "ubicacion_falsa"
)

// Estados de una reseña. Solo `publicada` se ve y cuenta en la calificación.
const (
	EstadoEnModeracion = "en_moderacion"
	EstadoPublicada    = "publicada"
	EstadoOculta       = "oculta"
	EstadoRechazada    = "rechazada"
	EstadoEliminada    = "eliminada"
)

// Tipos con que la reseña y la respuesta del negocio entran a la cola de nv-43.
const (
	TipoColaResena    = "resena"
	TipoColaRespuesta = "resena_respuesta"
	// FallaReportes marca en la cola lo que entró por reportes, no por el moderador.
	FallaReportes = "reportes"
)

// Reglas fijas de J7.
const (
	RadioMaxM           = 100
	PrecisionMaxM       = 100
	VelocidadMaxKmh     = 300
	SaltoMinKm          = 1.0
	DiasPorResenar      = 30
	DiasEntreResenas    = 30
	PlazoEdicion        = 48 * time.Hour
	ReportesParaOcultar = 3
	MaxComentario       = 500
	MaxFotos            = 3
	MaxChips            = 5
	MaxRespuesta        = 500
)

// AutorAnonimo es como se muestra una reseña cuyo autor eliminó su cuenta.
const AutorAnonimo = "Usuario de Wherever"

// Aspectos son los cinco de `negocios.Aspectos`, iguales para todos los giros.
type Aspectos struct {
	Sabor         int `bson:"sabor" json:"sabor"`
	Atencion      int `bson:"atencion" json:"atencion"`
	Ambiente      int `bson:"ambiente" json:"ambiente"`
	Espera        int `bson:"espera" json:"espera"`
	PrecioCalidad int `bson:"precio_calidad" json:"precio_calidad"`
}

// Checkin es un documento de `checkins`. Nunca guarda la ubicación de la persona: basta la
// distancia al negocio.
type Checkin struct {
	ID            bson.ObjectID  `bson:"_id"`
	UsuarioID     bson.ObjectID  `bson:"usuario_id"`
	NegocioID     bson.ObjectID  `bson:"negocio_id"`
	PlanID        *bson.ObjectID `bson:"plan_id,omitempty"`
	Metodo        string         `bson:"metodo"`
	DistanciaM    *int           `bson:"distancia_m,omitempty"`
	PrecisionM    *int           `bson:"precision_m,omitempty"`
	Valido        bool           `bson:"valido"`
	MotivoRechazo string         `bson:"motivo_rechazo,omitempty"`
	Dia           string         `bson:"dia"`
	ResenaID      *bson.ObjectID `bson:"resena_id,omitempty"`
	CreadoEn      time.Time      `bson:"creado_en"`
}

// Foto es una imagen publicada de nv-45 (id y URL de CloudFront).
type Foto struct {
	ID  string `bson:"id" json:"id"`
	URL string `bson:"url" json:"url"`
}

// Respuesta es la respuesta del negocio; pasa por la misma moderación.
type Respuesta struct {
	Texto   string        `bson:"texto"`
	AutorID bson.ObjectID `bson:"autor_id"`
	Fecha   time.Time     `bson:"fecha"`
	Estado  string        `bson:"estado"`
}

// Reporte es uno por persona y reseña. `Por` se quita si la persona elimina su cuenta.
type Reporte struct {
	Por    *bson.ObjectID `bson:"por,omitempty"`
	Motivo string         `bson:"motivo"`
	Fecha  time.Time      `bson:"fecha"`
}

// Resena es un documento de `resenas`. `UsuarioID` nil = anonimizada. `Contada` dice si ya suma
// en `negocios.calificacion_suma`.
type Resena struct {
	ID               bson.ObjectID  `bson:"_id"`
	NegocioID        bson.ObjectID  `bson:"negocio_id"`
	UsuarioID        *bson.ObjectID `bson:"usuario_id"`
	CheckinID        bson.ObjectID  `bson:"checkin_id"`
	Calificacion     int            `bson:"calificacion"`
	Aspectos         Aspectos       `bson:"aspectos"`
	Chips            []string       `bson:"chips"`
	Comentario       string         `bson:"comentario"`
	Fotos            []Foto         `bson:"fotos"`
	Estado           string         `bson:"estado"`
	Contada          bool           `bson:"contada"`
	Util             int            `bson:"util"`
	RespuestaNegocio *Respuesta     `bson:"respuesta_negocio,omitempty"`
	Reportes         []Reporte      `bson:"reportes"`
	ModeradaEn       *time.Time     `bson:"moderada_en,omitempty"`
	Version          int            `bson:"version"`
	CreadoEn         time.Time      `bson:"creado_en"`
	ActualizadoEn    time.Time      `bson:"actualizado_en"`
}

// Aporte es lo que la reseña suma a la calificación del negocio (cero si no está contada).
func (r Resena) Aporte() negocios.Suma {
	if !r.Contada {
		return negocios.Suma{}
	}
	a := r.Aspectos
	return negocios.Suma{
		Total: 1, Estrellas: int64(r.Calificacion), Sabor: int64(a.Sabor), Atencion: int64(a.Atencion),
		Ambiente: int64(a.Ambiente), Espera: int64(a.Espera), PrecioCalidad: int64(a.PrecioCalidad),
	}
}

// Util es un documento de `resenas_utiles`: una marca de «me fue útil» por persona y reseña.
type Util struct {
	ID        bson.ObjectID `bson:"_id"`
	ResenaID  bson.ObjectID `bson:"resena_id"`
	UsuarioID bson.ObjectID `bson:"usuario_id"`
	CreadoEn  time.Time     `bson:"creado_en"`
}

func idx(keys bson.D, opts *options.IndexOptionsBuilder) mongo.IndexModel {
	return mongo.IndexModel{Keys: keys, Options: opts}
}

// Indexes son los índices de nv-62; los crea solo `scripts/indexes`.
func Indexes() []platformmongo.IndexSet {
	valido := bson.D{{Key: "valido", Value: true}}
	return []platformmongo.IndexSet{
		{Collection: CheckinsCollection, Models: []mongo.IndexModel{
			idx(bson.D{{Key: "usuario_id", Value: 1}, {Key: "creado_en", Value: -1}, {Key: "_id", Value: -1}}, options.Index().SetName("ix_mis_checkins")),
			idx(bson.D{{Key: "negocio_id", Value: 1}, {Key: "creado_en", Value: -1}}, options.Index().SetName("ix_checkins_negocio")),
			idx(bson.D{{Key: "usuario_id", Value: 1}, {Key: "creado_en", Value: -1}}, options.Index().SetName("ix_por_resenar").SetPartialFilterExpression(valido)),
			idx(bson.D{{Key: "usuario_id", Value: 1}, {Key: "negocio_id", Value: 1}, {Key: "dia", Value: 1}}, options.Index().SetName("ux_checkin_dia").SetUnique(true).SetPartialFilterExpression(valido)),
		}},
		{Collection: Collection, Models: []mongo.IndexModel{
			idx(bson.D{{Key: "checkin_id", Value: 1}}, options.Index().SetName("ux_resena_checkin").SetUnique(true)),
			idx(bson.D{{Key: "negocio_id", Value: 1}, {Key: "estado", Value: 1}, {Key: "util", Value: -1}, {Key: "_id", Value: -1}}, options.Index().SetName("ix_resenas_utiles")),
			idx(bson.D{{Key: "negocio_id", Value: 1}, {Key: "estado", Value: 1}, {Key: "calificacion", Value: 1}, {Key: "creado_en", Value: -1}, {Key: "_id", Value: -1}}, options.Index().SetName("ix_resenas_estrellas")),
			idx(bson.D{{Key: "negocio_id", Value: 1}, {Key: "estado", Value: 1}, {Key: "creado_en", Value: -1}, {Key: "_id", Value: -1}}, options.Index().SetName("ix_resenas_recientes")),
			idx(bson.D{{Key: "usuario_id", Value: 1}, {Key: "creado_en", Value: -1}, {Key: "_id", Value: -1}}, options.Index().SetName("ix_mis_resenas")),
			idx(bson.D{{Key: "negocio_id", Value: 1}, {Key: "comentario", Value: "text"}}, options.Index().SetName("ix_resenas_texto").SetDefaultLanguage("spanish")),
			idx(bson.D{{Key: "reportes.por", Value: 1}}, options.Index().SetName("ix_reportadas").SetPartialFilterExpression(bson.D{{Key: "reportes.0", Value: bson.D{{Key: "$exists", Value: true}}}})),
		}},
		{Collection: UtilesCollection, Models: []mongo.IndexModel{
			idx(bson.D{{Key: "resena_id", Value: 1}, {Key: "usuario_id", Value: 1}}, options.Index().SetName("ux_util").SetUnique(true)),
			idx(bson.D{{Key: "usuario_id", Value: 1}}, options.Index().SetName("ix_util_usuario")),
		}},
	}
}

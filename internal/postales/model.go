// Package postales convierte fotos de la salida en postales, arma el recuerdo de un plan
// terminado y publica postales como momentos (spec nv-63, hoja J8). La app dibuja la postal y la
// tarjeta y las sube por nv-45; aquí solo se guardan, se moderan y se decide quién las ve.
package postales

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	platformmongo "github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
)

// Colecciones de nv-63.
const (
	Collection          = "postales"
	RecuerdosCollection = "recuerdos"
	MomentosCollection  = "momentos"
)

// Estilos que la app sabe dibujar.
const (
	EstiloClasica   = "clasica"
	EstiloPolaroid  = "polaroid"
	EstiloPasaporte = "pasaporte"
)

// Visibilidades: una postal puede ser privada; un momento solo amigos o pública.
const (
	VisPrivada = "privada"
	VisAmigos  = "amigos"
	VisPublica = "publica"
)

// Estados de una postal.
const (
	EstadoEnModeracion = "en_moderacion"
	EstadoLista        = "lista"
	EstadoRechazada    = "rechazada"
)

// Estados de un momento (su texto se modera aparte de la postal).
const (
	MomentoEnModeracion = "en_moderacion"
	MomentoPublicado    = "publicado"
	MomentoRechazado    = "rechazado"
)

// Estados del texto de una postal o un momento ante la moderación.
const (
	TextoAprobado   = "aprobado"
	TextoEnRevision = "en_revision"
	TextoRechazado  = "rechazado"
)

// Tipos con los que los textos entran a la cola de nv-43.
const (
	TipoColaPostal  = "postal"
	TipoColaMomento = "momento"
)

// Topes de la spec.
const (
	MaxTextoPostal  = 80
	MaxTextoMomento = 280
	MaxEtiquetados  = 10
	// MaxAmigosFeed es el tope de amigos que entran al `$in` del feed (nv-36 lista hasta 500).
	MaxAmigosFeed = 500
)

// Postal es un documento de `postales`. `estado` se recalcula de la imagen y del texto.
type Postal struct {
	ID           bson.ObjectID  `bson:"_id"`
	UsuarioID    bson.ObjectID  `bson:"usuario_id"`
	ImagenID     string         `bson:"imagen_id"`
	ImagenEstado string         `bson:"imagen_estado"`
	ImagenURL    string         `bson:"imagen_url,omitempty"`
	PlanID       *bson.ObjectID `bson:"plan_id,omitempty"`
	NegocioID    *bson.ObjectID `bson:"negocio_id,omitempty"`
	Estilo       string         `bson:"estilo"`
	Texto        string         `bson:"texto,omitempty"`
	TextoEstado  string         `bson:"texto_estado"`
	Visibilidad  string         `bson:"visibilidad"`
	Estado       string         `bson:"estado"`
	Version      int            `bson:"version"`
	CreadoEn     time.Time      `bson:"creado_en"`
	// ActualizadoEn cambia con cada edición o decisión de moderación.
	ActualizadoEn time.Time `bson:"actualizado_en"`
}

// Parada es la copia fija de una parada del plan en el recuerdo.
type Parada struct {
	Nombre  string    `bson:"nombre" json:"nombre"`
	Hora    time.Time `bson:"hora" json:"hora"`
	FotoURL string    `bson:"foto_url,omitempty" json:"foto_url,omitempty"`
}

// Recuerdo es un documento de `recuerdos`: uno por persona y plan.
type Recuerdo struct {
	ID              bson.ObjectID   `bson:"_id"`
	UsuarioID       bson.ObjectID   `bson:"usuario_id"`
	PlanID          bson.ObjectID   `bson:"plan_id"`
	Titulo          string          `bson:"titulo"`
	Fecha           time.Time       `bson:"fecha"`
	Paradas         []Parada        `bson:"paradas"`
	PostalesIDs     []bson.ObjectID `bson:"postales_ids"`
	GastoTotal      int64           `bson:"gasto_total"`
	PuntosGanados   int             `bson:"puntos_ganados"`
	TarjetaImagenID string          `bson:"tarjeta_imagen_id,omitempty"`
	TarjetaURL      string          `bson:"tarjeta_url,omitempty"`
	CreadoEn        time.Time       `bson:"creado_en"`
	ActualizadoEn   time.Time       `bson:"actualizado_en"`
}

// Momento es un documento de `momentos`: una postal compartida con los amigos o el público.
type Momento struct {
	ID          bson.ObjectID   `bson:"_id"`
	AutorID     bson.ObjectID   `bson:"autor_id"`
	PostalID    bson.ObjectID   `bson:"postal_id"`
	NegocioID   *bson.ObjectID  `bson:"negocio_id,omitempty"`
	Texto       string          `bson:"texto,omitempty"`
	Visibilidad string          `bson:"visibilidad"`
	Estado      string          `bson:"estado"`
	Etiquetados []bson.ObjectID `bson:"etiquetados"`
	PlanID      *bson.ObjectID  `bson:"plan_id,omitempty"`
	// ReaccionesTotal queda en 0 hasta el módulo social B2.
	ReaccionesTotal int       `bson:"reacciones_total"`
	Destacado       bool      `bson:"destacado_por_negocio"`
	Agradecido      bool      `bson:"agradecido_por_negocio"`
	Version         int       `bson:"version"`
	CreadoEn        time.Time `bson:"creado_en"`
	ActualizadoEn   time.Time `bson:"actualizado_en"`
}

func idx(keys bson.D, o *options.IndexOptionsBuilder) mongo.IndexModel {
	return mongo.IndexModel{Keys: keys, Options: o}
}

// Indexes son los índices de nv-63; los crea solo `scripts/indexes`. Postales y momentos se
// borran de verdad (sin `eliminado_en`), así que los únicos no necesitan filtro parcial.
func Indexes() []platformmongo.IndexSet {
	return []platformmongo.IndexSet{
		{Collection: Collection, Models: []mongo.IndexModel{
			idx(bson.D{{Key: "usuario_id", Value: 1}, {Key: "creado_en", Value: -1}, {Key: "_id", Value: -1}}, options.Index().SetName("ix_album")),
			idx(bson.D{{Key: "usuario_id", Value: 1}, {Key: "plan_id", Value: 1}}, options.Index().SetName("ix_postales_plan")),
			idx(bson.D{{Key: "negocio_id", Value: 1}, {Key: "visibilidad", Value: 1}, {Key: "creado_en", Value: -1}, {Key: "_id", Value: -1}}, options.Index().SetName("ix_postales_negocio")),
			idx(bson.D{{Key: "imagen_id", Value: 1}}, options.Index().SetName("ux_postal_imagen").SetUnique(true)),
		}},
		{Collection: RecuerdosCollection, Models: []mongo.IndexModel{
			idx(bson.D{{Key: "usuario_id", Value: 1}, {Key: "plan_id", Value: 1}}, options.Index().SetName("ux_recuerdo").SetUnique(true)),
			idx(bson.D{{Key: "usuario_id", Value: 1}, {Key: "fecha", Value: -1}, {Key: "_id", Value: -1}}, options.Index().SetName("ix_recuerdos_fecha")),
		}},
		{Collection: MomentosCollection, Models: []mongo.IndexModel{
			idx(bson.D{{Key: "autor_id", Value: 1}, {Key: "creado_en", Value: -1}, {Key: "_id", Value: -1}}, options.Index().SetName("ix_feed")),
			idx(bson.D{{Key: "etiquetados", Value: 1}, {Key: "creado_en", Value: -1}}, options.Index().SetName("ix_etiquetados")),
			idx(bson.D{{Key: "negocio_id", Value: 1}, {Key: "visibilidad", Value: 1}, {Key: "creado_en", Value: -1}, {Key: "_id", Value: -1}}, options.Index().SetName("ix_momentos_negocio")),
			idx(bson.D{{Key: "postal_id", Value: 1}}, options.Index().SetName("ux_momento_postal").SetUnique(true)),
		}},
	}
}

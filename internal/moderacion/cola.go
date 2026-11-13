package moderacion

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Colecciones de la cola de revisión y de las reglas (nv-43).
const (
	ColeccionCola   = "moderacion_cola"
	ColeccionReglas = "moderacion_reglas"
)

// Estados de un elemento de la cola. Solo pendiente acepta una decisión.
const (
	EstadoPendiente = "pendiente"
	EstadoAprobado  = "aprobado"
	EstadoRechazado = "rechazado"
)

// idReglas es el _id del único documento de moderacion_reglas: hay un solo juego vigente.
const idReglas = "vigentes"

// Límites de la cola y de las palabras bloqueadas (los de los DTO van en sus etiquetas).
const (
	MaxPalabras      = 200
	MaxRunasTipo     = 30
	MaxRunasRefOrig  = 100
	MaxRunasImgClave = 300
)

// Entrada es lo que el llamador (nv-45 y siguientes) manda a la cola cuando Evaluar deja el
// contenido en cuarentena. Referencia es el id del recurso de origen (foto, reseña, postal);
// ImagenClave es la llave del objeto en S3 (nv-44), nunca los bytes.
type Entrada struct {
	Tipo        string
	Referencia  string
	AutorID     bson.ObjectID
	Texto       string
	ImagenClave string
	Resultado   Resultado
}

// Item es un elemento de moderacion_cola. La decisión guarda quién (RevisorID) y cuándo
// (DecididoEn); Version sube con cada cambio para If-Match.
type Item struct {
	ID          bson.ObjectID      `bson:"_id"`
	Tipo        string             `bson:"tipo"`
	Referencia  string             `bson:"referencia"`
	AutorID     bson.ObjectID      `bson:"autor_id"`
	Texto       string             `bson:"texto,omitempty"`
	ImagenClave string             `bson:"imagen_clave,omitempty"`
	Veredicto   Veredicto          `bson:"veredicto"`
	Puntajes    map[string]float64 `bson:"puntajes"`
	Falla       string             `bson:"falla,omitempty"`
	Estado      string             `bson:"estado"`
	Motivo      string             `bson:"motivo,omitempty"`
	RevisorID   *bson.ObjectID     `bson:"revisor_id,omitempty"`
	DecididoEn  *time.Time         `bson:"decidido_en,omitempty"`
	CreadoEn    time.Time          `bson:"creado_en"`
	Version     int                `bson:"version"`
}

// reglasDoc es el documento de moderacion_reglas.
type reglasDoc struct {
	ID             string        `bson:"_id"`
	Cuarentena     float64       `bson:"cuarentena"`
	Rechazo        float64       `bson:"rechazo"`
	Palabras       []string      `bson:"palabras"`
	Version        int           `bson:"version"`
	ActualizadoPor bson.ObjectID `bson:"actualizado_por"`
	ActualizadoEn  time.Time     `bson:"actualizado_en"`
}

func (d reglasDoc) reglas() Reglas {
	return Reglas{Cuarentena: d.Cuarentena, Rechazo: d.Rechazo, Palabras: d.Palabras}
}

// Destino recibe la decisión del administrador para publicar o descartar el recurso de
// origen. Lo registra el módulo dueño del recurso (nv-45) con Service.WithDestino.
type Destino interface {
	AlDecidir(ctx context.Context, it Item) error
}

// Package eventos publica eventos, ferias y tianguis cercanos (nv-65, hoja A2): modelo,
// repositorio, service, handler y ARCO. Los publica un administrador o el comerciante dueño de
// un negocio aprobado; la lista es pública.
package eventos

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Collection guarda los eventos (A2 §2).
const Collection = "eventos"

// Tipos de evento.
const (
	TipoEvento   = "evento"
	TipoFeria    = "feria"
	TipoTianguis = "tianguis"
)

// Estados de un evento. Uno cancelado no vuelve a publicado.
const (
	EstadoBorrador  = "borrador"
	EstadoPublicado = "publicado"
	EstadoCancelado = "cancelado"
)

// TipoCola es el tipo de los textos de eventos en la cola de moderación (nv-43).
const TipoCola = "evento"

// Límites de la spec.
const (
	MaxTitulo        = 60
	MaxDescripcion   = 300
	MaxDireccion     = 200
	maxAdelanto      = 180 * 24 * time.Hour
	maxDuracion      = 30 * 24 * time.Hour
	toleranciaPasado = time.Hour
	RadioDefecto     = 10000
	RadioMaximo      = 50000
	DiasDefecto      = 7
)

// Punto es un GeoJSON Point `[longitud, latitud]`.
type Punto struct {
	Type        string    `bson:"type"`
	Coordinates []float64 `bson:"coordinates"`
}

func nuevoPunto(lat, lng float64) Punto {
	return Punto{Type: "Point", Coordinates: []float64{lng, lat}}
}

// Evento es un documento de `eventos`. `creado_por` falta cuando la cuenta que lo creó se
// borró y el evento sigue siendo del negocio. `en_revision` = la moderación lo dejó en la cola:
// no se muestra hasta que un administrador lo apruebe.
type Evento struct {
	ID            bson.ObjectID  `bson:"_id"`
	Titulo        string         `bson:"titulo"`
	Tipo          string         `bson:"tipo"`
	Descripcion   string         `bson:"descripcion,omitempty"`
	NegocioID     *bson.ObjectID `bson:"negocio_id,omitempty"`
	Ubicacion     Punto          `bson:"ubicacion"`
	Direccion     string         `bson:"direccion"`
	Inicio        time.Time      `bson:"inicio"`
	Fin           time.Time      `bson:"fin"`
	Precio        int64          `bson:"precio"`
	FotoURL       string         `bson:"foto_url,omitempty"`
	Estado        string         `bson:"estado"`
	EnRevision    bool           `bson:"en_revision,omitempty"`
	CreadoPor     *bson.ObjectID `bson:"creado_por,omitempty"`
	Version       int            `bson:"version"`
	CreadoEn      time.Time      `bson:"creado_en"`
	ActualizadoEn time.Time      `bson:"actualizado_en"`
}

// Visible dice si cualquiera puede ver el evento: publicado y fuera de revisión.
func (e Evento) Visible() bool { return e.Estado == EstadoPublicado && !e.EnRevision }

// Fila es un evento de la lista con su distancia al punto pedido.
type Fila struct {
	Evento     `bson:",inline"`
	DistanciaM float64 `bson:"distancia_m"`
}

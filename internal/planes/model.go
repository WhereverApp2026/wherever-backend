// Package planes es el modelo de plan de nv-29: paradas, traslados estimados y costos que
// se recalculan siempre con Recalcular. Los endpoints (nv-30) están en service.go y handler.go.
package planes

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Collection guarda los planes (A4).
const Collection = "planes"

// Estados del plan. El borrado es lógico (nv-30).
const (
	StatusActive  = "activo"
	StatusDeleted = "borrado"
)

// Origen del plan.
const (
	OriginManual = "manual"
	OriginAI     = "ia"
	OriginRules  = "reglas"
	// OriginCloned es la copia de un plan público de otra persona (nv-68).
	OriginCloned = "clonado"
)

// Visibilidad del plan (nv-68). Los planes de antes no tienen el campo y son privados.
// `amigos` (nv-84) lo ven las amistades aceptadas del dueño.
const (
	VisibilityPrivate = "privado"
	VisibilityFriends = "amigos"
	VisibilityPublic  = "publico"
)

// GeoPoint es un punto GeoJSON `[lng, lat]`.
type GeoPoint struct {
	Type        string    `bson:"type"`
	Coordinates []float64 `bson:"coordinates"`
}

// Movilidades: el mismo catálogo que `preferencias.movilidad_default` de nv-05.
const (
	MobilityWalk    = "a_pie"
	MobilityBike    = "bicicleta"
	MobilityCar     = "auto"
	MobilityTransit = "transporte_publico"
)

// Límites de A4: de 1 a 6 paradas.
const (
	MinStops = 1
	MaxStops = 6
)

// Parada es un negocio con hora de llegada, duración en minutos y precio estimado por persona.
type Parada struct {
	ID        bson.ObjectID `bson:"id" json:"id"`
	NegocioID bson.ObjectID `bson:"negocio_id" json:"negocio_id"`
	Llega     time.Time     `bson:"llega" json:"llega"`
	Minutos   int           `bson:"minutos" json:"minutos"`
	Precio    int           `bson:"precio" json:"precio"`
	Nota      string        `bson:"nota,omitempty" json:"nota,omitempty"`
}

// Traslado es el tramo estimado entre dos paradas consecutivas (ids de parada).
type Traslado struct {
	Desde   bson.ObjectID `bson:"desde" json:"desde"`
	Hasta   bson.ObjectID `bson:"hasta" json:"hasta"`
	Minutos int           `bson:"minutos" json:"minutos"`
	Km      float64       `bson:"km" json:"km"`
}

// Plan es el documento de `planes`. `traslados`, `costo_total` y `costo_por_persona` solo
// los escribe Recalcular.
type Plan struct {
	ID              bson.ObjectID `bson:"_id" json:"id"`
	OwnerID         bson.ObjectID `bson:"dueno_id" json:"-"`
	Nombre          string        `bson:"nombre" json:"nombre"`
	Fecha           time.Time     `bson:"fecha" json:"fecha"`
	Personas        int           `bson:"personas" json:"personas"`
	Presupuesto     int           `bson:"presupuesto" json:"presupuesto"`
	Movilidad       string        `bson:"movilidad" json:"movilidad"`
	Paradas         []Parada      `bson:"paradas" json:"paradas"`
	Traslados       []Traslado    `bson:"traslados" json:"traslados"`
	CostoTotal      int           `bson:"costo_total" json:"costo_total"`
	CostoPorPersona int           `bson:"costo_por_persona" json:"costo_por_persona"`
	Estado          string        `bson:"estado" json:"-"`
	Origen          string        `bson:"origen" json:"origen"`
	Version         int           `bson:"version" json:"version"`
	// Visibilidad: `publico` lo pueden ver y copiar otras personas, sin saber de quién es.
	Visibilidad  string `bson:"visibilidad,omitempty" json:"visibilidad"`
	VecesClonado int    `bson:"veces_clonado" json:"veces_clonado"`
	// Contadores de nv-83: solo los mueve reacciones con `$inc` (las ediciones no los tocan).
	MeGustaTotal     int            `bson:"me_gusta_total" json:"me_gusta_total"`
	ComentariosTotal int            `bson:"comentarios_total" json:"comentarios_total"`
	PlanOrigenID     *bson.ObjectID `bson:"plan_origen_id,omitempty" json:"plan_origen_id,omitempty"`
	// Zona es la ubicación de la primera parada; la fija Calculadora.Aplicar (populares).
	Zona *GeoPoint `bson:"zona,omitempty" json:"-"`
	// SyncSeq es el número del último cambio en la sincronización del dueño (nv-46).
	// Sorpresa sale solo a la persona sorprendida del grupo (nv-69); nunca se guarda.
	Sorpresa      *Sorpresa `bson:"-" json:"sorpresa,omitempty"`
	SyncSeq       int64     `bson:"sync_seq" json:"-"`
	CreadoEn      time.Time `bson:"creado_en" json:"creado_en"`
	ActualizadoEn time.Time `bson:"actualizado_en" json:"actualizado_en"`
}

// Public dice si otras personas pueden verlo y copiarlo.
func (p Plan) Public() bool { return p.Visibilidad == VisibilityPublic }

// SharedWithFriends dice si sus amistades lo ven: `amigos` o `publico` (nv-84).
func (p Plan) SharedWithFriends() bool {
	return p.Visibilidad == VisibilityFriends || p.Visibilidad == VisibilityPublic
}

// normalize deja `visibilidad` explícita en la respuesta (los planes de antes no la tienen).
func (p *Plan) normalize() {
	if p.Visibilidad == "" {
		p.Visibilidad = VisibilityPrivate
	}
}

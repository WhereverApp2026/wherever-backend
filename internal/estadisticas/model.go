// Package estadisticas guarda las cifras diarias de cada negocio (nv-74, US-144): visitas,
// reservas, llegadas, cupones, favoritos y reseñas por día local, sin datos de personas. De ahí
// salen GET /negocios/{id}/estadisticas y el reporte mensual en PDF.
package estadisticas

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Colecciones de la spec. Ninguna lleva `usuario_id`: no entran en ARCO.
const (
	Collection          = "estadisticas_diarias"
	CategoriaCollection = "estadisticas_categoria"
	RellenoCollection   = "estadisticas_relleno"
)

// Límites de la spec nv-74.
const (
	Zona                 = "America/Mexico_City"
	FormatoDia           = "2006-01-02"
	FormatoMes           = "2006-01"
	DiasRetencion        = 400
	DiasRelleno          = 90
	LoteRelleno          = 50
	DiasPorOmision       = 30
	DiasMaximos          = 90
	MesesReporte         = 12
	MinNegociosCategoria = 5
	MinDiasFrase         = 7
	AgruparDia           = "dia"
	AgruparSemana        = "semana"
)

// Origen cuenta de dónde llegó cada vista a la ficha (valores de nv-66).
type Origen struct {
	Busqueda int `bson:"busqueda" json:"busqueda"`
	Mapa     int `bson:"mapa" json:"mapa"`
	Inicio   int `bson:"inicio" json:"inicio"`
	PlanIA   int `bson:"plan_ia" json:"plan_ia"`
	Cupon    int `bson:"cupon" json:"cupon"`
}

func (o *Origen) sumar(x Origen) {
	o.Busqueda += x.Busqueda
	o.Mapa += x.Mapa
	o.Inicio += x.Inicio
	o.PlanIA += x.PlanIA
	o.Cupon += x.Cupon
}

// Aspectos son los cinco aspectos de nv-62. En `estadisticas_diarias` guardan la suma del día
// (para promediar rangos con el peso de cada día); en `estadisticas_categoria`, el promedio.
type Aspectos struct {
	Sabor         float64 `bson:"sabor"`
	Atencion      float64 `bson:"atencion"`
	Ambiente      float64 `bson:"ambiente"`
	Espera        float64 `bson:"espera"`
	PrecioCalidad float64 `bson:"precio_calidad"`
}

func (a *Aspectos) sumar(x Aspectos) {
	a.Sabor += x.Sabor
	a.Atencion += x.Atencion
	a.Ambiente += x.Ambiente
	a.Espera += x.Espera
	a.PrecioCalidad += x.PrecioCalidad
}

func (a Aspectos) entre(n float64) Aspectos {
	return Aspectos{Sabor: a.Sabor / n, Atencion: a.Atencion / n, Ambiente: a.Ambiente / n, Espera: a.Espera / n, PrecioCalidad: a.PrecioCalidad / n}
}

// Diaria es un documento de `estadisticas_diarias`: un negocio en un día local.
type Diaria struct {
	ID               bson.ObjectID `bson:"_id"`
	NegocioID        bson.ObjectID `bson:"negocio_id"`
	Fecha            string        `bson:"fecha"`
	Categoria        string        `bson:"categoria,omitempty"`
	VistasFicha      int           `bson:"vistas_ficha"`
	Reservas         int           `bson:"reservas"`
	Llegadas         int           `bson:"llegadas"`
	CuponesCanjeados int           `bson:"cupones_canjeados"`
	FavoritosNuevos  int           `bson:"favoritos_nuevos"`
	Origen           Origen        `bson:"origen"`
	PorHora          []int         `bson:"por_hora"`
	CalificacionDia  *float64      `bson:"calificacion_dia,omitempty"`
	ResenasDia       int           `bson:"resenas_dia"`
	AspectosSuma     Aspectos      `bson:"aspectos_suma"`
	ActualizadoEn    time.Time     `bson:"actualizado_en"`
}

// activa dice si el día tuvo algún movimiento (para la frase).
func (d Diaria) activa() bool {
	return d.VistasFicha+d.Reservas+d.Llegadas+d.CuponesCanjeados+d.FavoritosNuevos+d.ResenasDia > 0
}

// Categoria es un documento de `estadisticas_categoria`: promedios de una categoría en un mes.
type Categoria struct {
	ID            bson.ObjectID `bson:"_id"`
	Categoria     string        `bson:"categoria"`
	Mes           string        `bson:"mes"`
	Aspectos      Aspectos      `bson:"aspectos"`
	Calificacion  float64       `bson:"calificacion"`
	Resenas       int           `bson:"resenas"`
	Negocios      int           `bson:"negocios"`
	ActualizadoEn time.Time     `bson:"actualizado_en"`
}

// Actividad es lo que el trabajo calcula de las otras colecciones para un día.
type Actividad struct {
	Reservas         int
	Llegadas         int
	CuponesCanjeados int
	FavoritosNuevos  int
	PorHora          [24]int
	Resenas          int
	SumaCalificacion float64
	Aspectos         Aspectos
}

// TopCupon es un cupón del negocio con sus canjes del mes (reporte).
type TopCupon struct {
	Titulo string
	Canjes int
}

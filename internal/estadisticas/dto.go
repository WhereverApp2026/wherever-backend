package estadisticas

import "time"

// EstadisticasQuery es GET /negocios/{id}/estadisticas (días locales, ≤ 90).
type EstadisticasQuery struct {
	Desde   string `query:"desde" validate:"omitempty,datetime=2006-01-02"`
	Hasta   string `query:"hasta" validate:"omitempty,datetime=2006-01-02"`
	Agrupar string `query:"agrupar" validate:"omitempty,oneof=dia semana"`
}

// ReporteQuery es GET /negocios/{id}/reportes/mensual.
type ReporteQuery struct {
	Mes string `query:"mes" validate:"required,datetime=2006-01"`
}

// Cifras son los totales de un día, una semana o un periodo.
type Cifras struct {
	VistasFicha      int      `json:"vistas_ficha"`
	Reservas         int      `json:"reservas"`
	Llegadas         int      `json:"llegadas"`
	CuponesCanjeados int      `json:"cupones_canjeados"`
	FavoritosNuevos  int      `json:"favoritos_nuevos"`
	Calificacion     *float64 `json:"calificacion"`
}

// Punto es un día o una semana de la serie; `inicio` es su primer día dentro del rango.
type Punto struct {
	Inicio string `json:"inicio"`
	Cifras
}

// Cambio es el porcentaje contra el periodo anterior; null si el anterior fue 0.
type Cambio struct {
	VistasFicha      *float64 `json:"vistas_ficha"`
	Reservas         *float64 `json:"reservas"`
	Llegadas         *float64 `json:"llegadas"`
	CuponesCanjeados *float64 `json:"cupones_canjeados"`
	FavoritosNuevos  *float64 `json:"favoritos_nuevos"`
}

// AspectoComparado es un aspecto de nv-62 del negocio contra su categoría (null si no hay
// reseñas o la categoría tiene menos de 5 negocios).
type AspectoComparado struct {
	Aspecto   string   `json:"aspecto"`
	Tuyo      *float64 `json:"tuyo"`
	Categoria *float64 `json:"categoria"`
}

// Avanzadas son las cifras del plan premium.
type Avanzadas struct {
	PorHora  []int              `json:"por_hora"`
	Origen   Origen             `json:"origen"`
	Aspectos []AspectoComparado `json:"aspectos"`
}

// EstadisticasResponse es GET /negocios/{id}/estadisticas.
type EstadisticasResponse struct {
	Desde            string     `json:"desde"`
	Hasta            string     `json:"hasta"`
	Agrupar          string     `json:"agrupar"`
	Serie            []Punto    `json:"serie"`
	Totales          Cifras     `json:"totales"`
	Anterior         Cifras     `json:"anterior"`
	Cambio           Cambio     `json:"cambio"`
	Frase            string     `json:"frase"`
	Avanzadas        *Avanzadas `json:"avanzadas"`
	PremiumRequerido bool       `json:"premium_requerido"`
	ActualizadoEn    *time.Time `json:"actualizado_en"`
}

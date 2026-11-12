package busquedas

import (
	"time"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
)

// Límites de las consultas.
const (
	DefaultAutocompletar = 8
	MaxAutocompletar     = 20
	DefaultBuscar        = 10
	MaxBuscar            = 30
	minLetras            = 2
)

// AutocompletarQuery es `?q=&limite=` de GET /lugares/autocompletar.
type AutocompletarQuery struct {
	Q      string `query:"q" validate:"required,max=60"`
	Limite int    `query:"limite" validate:"min=1,max=20"`
}

// BuscarQuery es `?q=&limite=` de GET /lugares/buscar.
type BuscarQuery struct {
	Q      string `query:"q" validate:"required,max=100"`
	Limite int    `query:"limite" validate:"min=1,max=30"`
}

// Sugerencia es un elemento del autocompletar.
type Sugerencia struct {
	ID        string `json:"id"`
	Slug      string `json:"slug"`
	Nombre    string `json:"nombre"`
	Categoria string `json:"categoria"`
}

// AutocompletarRespuesta es la lista de sugerencias.
type AutocompletarRespuesta struct {
	Items []Sugerencia `json:"items"`
}

// BuscarRespuesta son las tarjetas de GET /lugares (sin distancia). `aproximada` dice que
// alguna salió por errores menores, para que la app muestre «Resultados parecidos».
type BuscarRespuesta struct {
	Items      []negocios.PlaceItem `json:"items"`
	Aproximada bool                 `json:"aproximada"`
}

// RecienteItem es una búsqueda reciente.
type RecienteItem struct {
	Texto     string    `json:"texto"`
	BuscadoEn time.Time `json:"buscado_en"`
}

// RecientesRespuesta es la lista de búsquedas recientes, de la más nueva a la más vieja.
type RecientesRespuesta struct {
	Items []RecienteItem `json:"items"`
}

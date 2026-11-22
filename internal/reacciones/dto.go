package reacciones

import (
	"time"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
)

// ComentarioRequest es el cuerpo de POST /{momentos|planes}/{id}/comentarios. `padre_id` lo
// hace respuesta (un nivel).
type ComentarioRequest struct {
	Texto   string `json:"texto" validate:"required,max=300"`
	PadreID string `json:"padre_id" validate:"omitempty,objectid"`
}

// ListaQuery es GET /{momentos|planes}/{id}/comentarios.
type ListaQuery struct {
	Cursor string `query:"cursor" validate:"omitempty,max=300,cursor"`
	Limite int    `query:"limite" validate:"min=1,max=50"`
}

// NewListaQuery trae el límite por omisión (20).
func NewListaQuery() ListaQuery { return ListaQuery{Limite: pagination.DefaultLimit} }

// ReaccionesQuery es GET /{momentos|planes}/{id}/reacciones.
type ReaccionesQuery struct {
	Pestana string `query:"pestana" validate:"oneof=todos me_gusta van"`
}

// NewReaccionesQuery abre la pestaña `todos`.
func NewReaccionesQuery() ReaccionesQuery { return ReaccionesQuery{Pestana: PestanaTodos} }

// AutorView es lo único que se muestra del autor de un comentario: nunca correo ni teléfono.
type AutorView struct {
	ID            string `json:"id"`
	Nombre        string `json:"nombre"`
	NombreUsuario string `json:"nombre_usuario"`
	FotoURL       string `json:"foto_url"`
}

// ComentarioView es un comentario; el eliminado sale con `autor: null` y `texto` vacío.
type ComentarioView struct {
	ID          string     `json:"id"`
	Autor       *AutorView `json:"autor"`
	PadreID     *string    `json:"padre_id"`
	Texto       string     `json:"texto"`
	Estado      string     `json:"estado"`
	Propio      bool       `json:"propio"`
	PuedeBorrar bool       `json:"puede_borrar"`
	CreadoEn    time.Time  `json:"creado_en"`
}

// HiloView es un comentario de primer nivel con sus respuestas, de la más vieja a la más nueva.
type HiloView struct {
	ComentarioView
	Respuestas []ComentarioView `json:"respuestas"`
}

// PersonaView es una persona en «quién reaccionó» (como la búsqueda de nv-35) con la relación
// vista por quien pregunta (`yo`, `amigo`, `solicitud_enviada`, `solicitud_recibida` o
// `ninguna`) y sus amigos en común.
type PersonaView struct {
	ID            string `json:"id"`
	Nombre        string `json:"nombre"`
	NombreUsuario string `json:"nombre_usuario"`
	FotoURL       string `json:"foto_url"`
	Relacion      string `json:"relacion"`
	AmigosEnComun int    `json:"amigos_en_comun"`
	MeGusta       bool   `json:"me_gusta"`
	Va            bool   `json:"va"`
}

// TotalesView son los números de las pestañas («Todos · 11 · Me gusta · 8 · Van · 4»).
type TotalesView struct {
	Todos   int64 `json:"todos"`
	MeGusta int64 `json:"me_gusta"`
	Van     int64 `json:"van"`
}

// ReaccionesResponse es GET /{momentos|planes}/{id}/reacciones.
type ReaccionesResponse struct {
	Pestana  string        `json:"pestana"`
	Totales  TotalesView   `json:"totales"`
	Personas []PersonaView `json:"personas"`
}

package postales

import (
	"time"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
)

// CrearPostalRequest es el cuerpo de POST /postales. `imagen_id` es una imagen propia de nv-45.
type CrearPostalRequest struct {
	ImagenID    string `json:"imagen_id" validate:"required,len=32,hexadecimal"`
	Estilo      string `json:"estilo" validate:"required,oneof=clasica polaroid pasaporte"`
	Texto       string `json:"texto" validate:"omitempty,max=80"`
	PlanID      string `json:"plan_id" validate:"omitempty,objectid"`
	NegocioID   string `json:"negocio_id" validate:"omitempty,objectid"`
	Visibilidad string `json:"visibilidad" validate:"required,oneof=privada amigos publica"`
}

// EditarPostalRequest es el cuerpo de PATCH /postales/{id}: solo cambia lo que trae. Cambiar el
// estilo es subir otra imagen y mandar su `imagen_id` (la app dibuja).
type EditarPostalRequest struct {
	ImagenID    *string `json:"imagen_id" validate:"omitempty,len=32,hexadecimal"`
	Estilo      *string `json:"estilo" validate:"omitempty,oneof=clasica polaroid pasaporte"`
	Texto       *string `json:"texto" validate:"omitempty,max=80"`
	Visibilidad *string `json:"visibilidad" validate:"omitempty,oneof=privada amigos publica"`
}

// AlbumQuery es GET /postales.
type AlbumQuery struct {
	PlanID    string `query:"plan_id" validate:"omitempty,objectid"`
	NegocioID string `query:"negocio_id" validate:"omitempty,objectid"`
	Estilo    string `query:"estilo" validate:"omitempty,oneof=clasica polaroid pasaporte"`
	Mes       string `query:"mes" validate:"omitempty,datetime=2006-01"`
	Cursor    string `query:"cursor" validate:"omitempty,max=300,cursor"`
	Limite    int    `query:"limite" validate:"min=1,max=50"`
}

// ListaQuery es la galería pública y el feed.
type ListaQuery struct {
	Cursor string `query:"cursor" validate:"omitempty,max=300,cursor"`
	Limite int    `query:"limite" validate:"min=1,max=50"`
}

// RecuerdosQuery es GET /recuerdos.
type RecuerdosQuery struct {
	Mes    string `query:"mes" validate:"omitempty,datetime=2006-01"`
	Cursor string `query:"cursor" validate:"omitempty,max=300,cursor"`
	Limite int    `query:"limite" validate:"min=1,max=50"`
}

// PanelQuery es GET /negocios/{id}/momentos.
type PanelQuery struct {
	Destacados bool   `query:"destacados"`
	Cursor     string `query:"cursor" validate:"omitempty,max=300,cursor"`
	Limite     int    `query:"limite" validate:"min=1,max=50"`
}

// NewAlbumQuery, NewListaQuery, NewRecuerdosQuery y NewPanelQuery traen el límite por defecto.
func NewAlbumQuery() AlbumQuery { return AlbumQuery{Limite: pagination.DefaultLimit} }

// NewListaQuery trae el límite por defecto.
func NewListaQuery() ListaQuery { return ListaQuery{Limite: pagination.DefaultLimit} }

// NewRecuerdosQuery trae el límite por defecto.
func NewRecuerdosQuery() RecuerdosQuery { return RecuerdosQuery{Limite: pagination.DefaultLimit} }

// NewPanelQuery trae el límite por defecto.
func NewPanelQuery() PanelQuery { return PanelQuery{Limite: pagination.DefaultLimit} }

// RecuerdoRequest es el cuerpo de POST /planes/{id}/recuerdo; la tarjeta la dibuja la app.
type RecuerdoRequest struct {
	TarjetaImagenID string `json:"tarjeta_imagen_id" validate:"omitempty,len=32,hexadecimal"`
}

// MomentoRequest es el cuerpo de POST /momentos.
type MomentoRequest struct {
	PostalID    string   `json:"postal_id" validate:"required,objectid"`
	Texto       string   `json:"texto" validate:"omitempty,max=280"`
	Visibilidad string   `json:"visibilidad" validate:"required,oneof=amigos publica"`
	Etiquetados []string `json:"etiquetados" validate:"omitempty,max=10,unique,dive,objectid"`
	PlanID      string   `json:"plan_id" validate:"omitempty,objectid"`
}

// EditarMomentoRequest es el cuerpo de PATCH /momentos/{id}.
type EditarMomentoRequest struct {
	Texto       *string   `json:"texto" validate:"omitempty,max=280"`
	Etiquetados *[]string `json:"etiquetados" validate:"omitempty,max=10,unique,dive,objectid"`
	Visibilidad *string   `json:"visibilidad" validate:"omitempty,oneof=amigos publica"`
}

// MarcarRequest es el cuerpo de PATCH /negocios/{id}/momentos/{mid}; al menos uno.
type MarcarRequest struct {
	Agradecido *bool `json:"agradecido"`
	Destacado  *bool `json:"destacado"`
}

// AutorView es el autor visible: nombre de usuario y foto; nunca correo, teléfono ni ubicación.
type AutorView struct {
	ID            string `json:"id"`
	NombreUsuario string `json:"nombre_usuario"`
	FotoURL       string `json:"foto_url,omitempty"`
}

// PostalResponse es una postal. `imagen_id` y `plan_id` solo los ve el dueño; `imagen_url`
// solo existe con la imagen publicada.
type PostalResponse struct {
	ID            string     `json:"id"`
	Autor         *AutorView `json:"autor,omitempty"`
	ImagenID      string     `json:"imagen_id,omitempty"`
	ImagenURL     string     `json:"imagen_url,omitempty"`
	Estilo        string     `json:"estilo"`
	Texto         string     `json:"texto"`
	Visibilidad   string     `json:"visibilidad"`
	Estado        string     `json:"estado"`
	PlanID        *string    `json:"plan_id"`
	NegocioID     *string    `json:"negocio_id"`
	Propia        bool       `json:"propia"`
	Version       int        `json:"version"`
	CreadoEn      time.Time  `json:"creado_en"`
	ActualizadoEn time.Time  `json:"actualizado_en"`
}

// PostalCorta es la postal dentro de un momento.
type PostalCorta struct {
	ID        string `json:"id"`
	ImagenURL string `json:"imagen_url,omitempty"`
	Estilo    string `json:"estilo"`
	Texto     string `json:"texto"`
}

// MomentoResponse es un momento. `plan_id` solo lo ve el autor.
type MomentoResponse struct {
	ID              string      `json:"id"`
	Autor           AutorView   `json:"autor"`
	Postal          PostalCorta `json:"postal"`
	NegocioID       *string     `json:"negocio_id"`
	Texto           string      `json:"texto"`
	Visibilidad     string      `json:"visibilidad"`
	Estado          string      `json:"estado"`
	Etiquetados     []string    `json:"etiquetados"`
	PlanID          *string     `json:"plan_id"`
	ReaccionesTotal int         `json:"reacciones_total"`
	Destacado       bool        `json:"destacado_por_negocio"`
	Agradecido      bool        `json:"agradecido_por_negocio"`
	Propio          bool        `json:"propio"`
	Version         int         `json:"version"`
	CreadoEn        time.Time   `json:"creado_en"`
	ActualizadoEn   time.Time   `json:"actualizado_en"`
}

// RecuerdoResponse es un recuerdo propio.
type RecuerdoResponse struct {
	ID              string    `json:"id"`
	PlanID          string    `json:"plan_id"`
	Titulo          string    `json:"titulo"`
	Fecha           time.Time `json:"fecha"`
	Paradas         []Parada  `json:"paradas"`
	PostalesIDs     []string  `json:"postales_ids"`
	GastoTotal      int64     `json:"gasto_total"`
	PuntosGanados   int       `json:"puntos_ganados"`
	TarjetaImagenID string    `json:"tarjeta_imagen_id,omitempty"`
	TarjetaURL      string    `json:"tarjeta_url,omitempty"`
	CreadoEn        time.Time `json:"creado_en"`
	ActualizadoEn   time.Time `json:"actualizado_en"`
}

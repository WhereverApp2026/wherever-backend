package eventos

import (
	"time"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
)

// Coordenadas es `ubicacion{lat, lng}` en el cuerpo.
type Coordenadas struct {
	Lat *float64 `json:"lat" validate:"required,latitude"`
	Lng *float64 `json:"lng" validate:"required,longitude"`
}

// CrearRequest es el cuerpo de POST /eventos. `estado` solo `borrador` o `publicado` (por
// omisión).
type CrearRequest struct {
	Titulo      string       `json:"titulo" validate:"required,max=60"`
	Tipo        string       `json:"tipo" validate:"required,oneof=evento feria tianguis"`
	Descripcion string       `json:"descripcion" validate:"omitempty,max=300"`
	NegocioID   string       `json:"negocio_id" validate:"omitempty,len=24,hexadecimal"`
	Ubicacion   *Coordenadas `json:"ubicacion" validate:"required"`
	Direccion   string       `json:"direccion" validate:"required,max=200"`
	Inicio      string       `json:"inicio" validate:"required,max=40"`
	Fin         string       `json:"fin" validate:"required,max=40"`
	Precio      *int64       `json:"precio" validate:"required,min=0,max=100000000"`
	FotoURL     string       `json:"foto_url" validate:"omitempty,max=500"`
	Estado      string       `json:"estado" validate:"omitempty,oneof=borrador publicado"`
}

// EditarRequest es el cuerpo parcial de PATCH /eventos/{id}; sin `negocio_id` ni `creado_por`.
// `estado: cancelado` cancela.
type EditarRequest struct {
	Titulo      *string      `json:"titulo" validate:"omitempty,max=60"`
	Tipo        *string      `json:"tipo" validate:"omitempty,oneof=evento feria tianguis"`
	Descripcion *string      `json:"descripcion" validate:"omitempty,max=300"`
	Ubicacion   *Coordenadas `json:"ubicacion"`
	Direccion   *string      `json:"direccion" validate:"omitempty,max=200"`
	Inicio      *string      `json:"inicio" validate:"omitempty,max=40"`
	Fin         *string      `json:"fin" validate:"omitempty,max=40"`
	Precio      *int64       `json:"precio" validate:"omitempty,min=0,max=100000000"`
	FotoURL     *string      `json:"foto_url" validate:"omitempty,max=500"`
	Estado      *string      `json:"estado" validate:"omitempty,oneof=borrador publicado cancelado"`
}

// ListaQuery es GET /eventos.
type ListaQuery struct {
	Lat    *float64 `query:"lat" validate:"required,latitude"`
	Lng    *float64 `query:"lng" validate:"required,longitude"`
	RadioM int      `query:"radio_m" validate:"min=1,max=50000"`
	Dias   int      `query:"dias" validate:"min=1,max=30"`
	Tipo   string   `query:"tipo" validate:"omitempty,oneof=evento feria tianguis"`
	Cursor string   `query:"cursor" validate:"omitempty,max=300,cursor"`
	Limit  int      `query:"limite" validate:"min=1,max=50"`
}

// NuevaListaQuery trae los valores por omisión: 10 km, 7 días y 20 por página.
func NuevaListaQuery() ListaQuery {
	return ListaQuery{RadioM: RadioDefecto, Dias: DiasDefecto, Limit: pagination.DefaultLimit}
}

// UbicacionView es `ubicacion{lat, lng}` en la respuesta.
type UbicacionView struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// EventoView es un evento en la respuesta. No lleva `creado_por`.
type EventoView struct {
	ID            string        `json:"id"`
	Titulo        string        `json:"titulo"`
	Tipo          string        `json:"tipo"`
	Descripcion   string        `json:"descripcion,omitempty"`
	NegocioID     string        `json:"negocio_id,omitempty"`
	Ubicacion     UbicacionView `json:"ubicacion"`
	Direccion     string        `json:"direccion"`
	Inicio        time.Time     `json:"inicio"`
	Fin           time.Time     `json:"fin"`
	Precio        int64         `json:"precio"`
	FotoURL       string        `json:"foto_url,omitempty"`
	Estado        string        `json:"estado"`
	EnRevision    bool          `json:"en_revision,omitempty"`
	DistanciaM    *float64      `json:"distancia_m,omitempty"`
	Version       int           `json:"version"`
	CreadoEn      time.Time     `json:"creado_en"`
	ActualizadoEn time.Time     `json:"actualizado_en"`
}

func nuevaVista(e Evento) EventoView {
	out := EventoView{
		ID: e.ID.Hex(), Titulo: e.Titulo, Tipo: e.Tipo, Descripcion: e.Descripcion, Direccion: e.Direccion,
		Inicio: e.Inicio, Fin: e.Fin, Precio: e.Precio, FotoURL: e.FotoURL, Estado: e.Estado, EnRevision: e.EnRevision,
		Version: e.Version, CreadoEn: e.CreadoEn, ActualizadoEn: e.ActualizadoEn,
	}
	if len(e.Ubicacion.Coordinates) == 2 {
		out.Ubicacion = UbicacionView{Lat: e.Ubicacion.Coordinates[1], Lng: e.Ubicacion.Coordinates[0]}
	}
	if e.NegocioID != nil {
		out.NegocioID = e.NegocioID.Hex()
	}
	return out
}

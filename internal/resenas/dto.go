package resenas

import (
	"time"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
)

// CheckinRequest es el cuerpo de POST /checkins. Con `ubicacion` llegan lat, lng y precisión;
// con `qr_negocio`, el código del mostrador. La ubicación nunca se guarda.
type CheckinRequest struct {
	NegocioID  string   `json:"negocio_id" validate:"required,objectid"`
	Metodo     string   `json:"metodo" validate:"required,oneof=ubicacion qr_negocio"`
	Lat        *float64 `json:"lat" validate:"required_if=Metodo ubicacion,omitempty,min=-90,max=90"`
	Lng        *float64 `json:"lng" validate:"required_if=Metodo ubicacion,omitempty,min=-180,max=180"`
	PrecisionM *int     `json:"precision_m" validate:"required_if=Metodo ubicacion,omitempty,min=1,max=10000"`
	Codigo     string   `json:"codigo" validate:"required_if=Metodo qr_negocio,omitempty,max=64"`
	PlanID     string   `json:"plan_id" validate:"omitempty,objectid"`
}

// AspectosRequest son los cinco aspectos, todos de 1 a 5.
type AspectosRequest struct {
	Sabor         int `json:"sabor" validate:"required,min=1,max=5"`
	Atencion      int `json:"atencion" validate:"required,min=1,max=5"`
	Ambiente      int `json:"ambiente" validate:"required,min=1,max=5"`
	Espera        int `json:"espera" validate:"required,min=1,max=5"`
	PrecioCalidad int `json:"precio_calidad" validate:"required,min=1,max=5"`
}

func (a AspectosRequest) modelo() Aspectos { return Aspectos(a) }

// CrearRequest es el cuerpo de POST /resenas.
type CrearRequest struct {
	CheckinID    string          `json:"checkin_id" validate:"required,objectid"`
	Calificacion int             `json:"calificacion" validate:"required,min=1,max=5"`
	Aspectos     AspectosRequest `json:"aspectos" validate:"required"`
	Chips        []string        `json:"chips" validate:"omitempty,max=5,unique"`
	Comentario   string          `json:"comentario" validate:"omitempty,max=500"`
	Fotos        []string        `json:"fotos" validate:"omitempty,max=3,unique,dive,required,max=64"`
}

// EditarRequest es el cuerpo de PATCH /resenas/{id}: solo cambia lo que trae.
type EditarRequest struct {
	Calificacion *int             `json:"calificacion" validate:"omitempty,min=1,max=5"`
	Aspectos     *AspectosRequest `json:"aspectos"`
	Chips        *[]string        `json:"chips" validate:"omitempty,max=5,unique"`
	Comentario   *string          `json:"comentario" validate:"omitempty,max=500"`
	Fotos        *[]string        `json:"fotos" validate:"omitempty,max=3,unique,dive,required,max=64"`
}

// UtilRequest es el cuerpo de PUT /resenas/{id}/util: la intención, para que reintentar no
// cuente dos veces.
type UtilRequest struct {
	Util *bool `json:"util" validate:"required"`
}

// ReporteRequest es el cuerpo de POST /resenas/{id}/reportes.
type ReporteRequest struct {
	Motivo string `json:"motivo" validate:"required,oneof=ofensiva falsa spam datos_personales otro"`
}

// RespuestaRequest es el cuerpo de PUT /negocios/{id}/resenas/{rid}/respuesta.
type RespuestaRequest struct {
	Texto string `json:"texto" validate:"required,min=1,max=500"`
}

// ListaQuery es GET /checkins y GET /resenas/mias.
type ListaQuery struct {
	Cursor string `query:"cursor" validate:"omitempty,max=300,cursor"`
	Limite int    `query:"limite" validate:"min=1,max=50"`
}

// NewListaQuery trae el límite por defecto.
func NewListaQuery() ListaQuery { return ListaQuery{Limite: pagination.DefaultLimit} }

// PublicaQuery es GET /lugares/{id}/resenas.
type PublicaQuery struct {
	Estrellas int    `query:"estrellas" validate:"omitempty,min=1,max=5"`
	Aspecto   string `query:"aspecto" validate:"omitempty,oneof=sabor atencion ambiente espera precio_calidad"`
	ConFotos  bool   `query:"con_fotos"`
	Q         string `query:"q" validate:"omitempty,max=100"`
	Orden     string `query:"orden" validate:"oneof=utiles recientes"`
	Cursor    string `query:"cursor" validate:"omitempty,max=300,cursor"`
	Limite    int    `query:"limite" validate:"min=1,max=50"`
}

// NewPublicaQuery ordena por útiles con el límite por defecto.
func NewPublicaQuery() PublicaQuery {
	return PublicaQuery{Orden: "utiles", Limite: pagination.DefaultLimit}
}

// ResumenQuery es GET /negocios/{id}/resenas/resumen.
type ResumenQuery struct {
	Periodo string `query:"periodo" validate:"oneof=mes trimestre"`
}

// CheckinResponse es un check-in propio.
type CheckinResponse struct {
	ID         string    `json:"id"`
	NegocioID  string    `json:"negocio_id"`
	Metodo     string    `json:"metodo"`
	Valido     bool      `json:"valido"`
	DistanciaM *int      `json:"distancia_m"`
	Dia        string    `json:"dia"`
	ResenaID   *string   `json:"resena_id"`
	CreadoEn   time.Time `json:"creado_en"`
}

// PendienteResponse es una visita de «Por reseñar».
type PendienteResponse struct {
	CheckinID     string    `json:"checkin_id"`
	NegocioID     string    `json:"negocio_id"`
	NegocioNombre string    `json:"negocio_nombre"`
	Dia           string    `json:"dia"`
	VenceEn       time.Time `json:"vence_en"`
}

// PendientesResponse es la lista de «Por reseñar».
type PendientesResponse struct {
	Items []PendienteResponse `json:"items"`
}

// RespuestaView es la respuesta del negocio tal como se muestra.
type RespuestaView struct {
	Texto  string    `json:"texto"`
	Fecha  time.Time `json:"fecha"`
	Estado string    `json:"estado,omitempty"`
}

// ResenaResponse es una reseña vista por su autor.
type ResenaResponse struct {
	ID               string         `json:"id"`
	NegocioID        string         `json:"negocio_id"`
	CheckinID        string         `json:"checkin_id"`
	Calificacion     int            `json:"calificacion"`
	Aspectos         Aspectos       `json:"aspectos"`
	Chips            []string       `json:"chips"`
	Comentario       string         `json:"comentario"`
	Fotos            []Foto         `json:"fotos"`
	Estado           string         `json:"estado"`
	Util             int            `json:"util"`
	RespuestaNegocio *RespuestaView `json:"respuesta_negocio"`
	Editable         bool           `json:"editable"`
	Version          int            `json:"version"`
	CreadoEn         time.Time      `json:"creado_en"`
	ActualizadoEn    time.Time      `json:"actualizado_en"`
}

// AutorView es el autor público: nombre de usuario y foto; nunca correo ni teléfono.
type AutorView struct {
	NombreUsuario string `json:"nombre_usuario"`
	FotoURL       string `json:"foto_url,omitempty"`
}

// PublicaResponse es una reseña en la lista pública: sin `checkin_id` ni datos de contacto.
type PublicaResponse struct {
	ID               string         `json:"id"`
	Autor            AutorView      `json:"autor"`
	Calificacion     int            `json:"calificacion"`
	Aspectos         Aspectos       `json:"aspectos"`
	Chips            []string       `json:"chips"`
	Comentario       string         `json:"comentario"`
	Fotos            []Foto         `json:"fotos"`
	Util             int            `json:"util"`
	RespuestaNegocio *RespuestaView `json:"respuesta_negocio"`
	CreadoEn         time.Time      `json:"creado_en"`
}

// NegocioResenaResponse es una reseña vista por el dueño del negocio tras responder.
type NegocioResenaResponse struct {
	ID               string         `json:"id"`
	RespuestaNegocio *RespuestaView `json:"respuesta_negocio"`
}

// UtilResponse es el resultado de marcar o desmarcar «me fue útil».
type UtilResponse struct {
	Util  bool `json:"util"`
	Total int  `json:"total"`
}

// ReporteResponse confirma el reporte.
type ReporteResponse struct {
	Reportada bool `json:"reportada"`
}

// SugerenciaResponse es la respuesta sugerida por la IA.
type SugerenciaResponse struct {
	Texto string `json:"texto"`
}

// Promedios son el promedio general y por aspecto de un periodo.
type Promedios struct {
	Total    int64              `json:"total"`
	Promedio float64            `json:"promedio"`
	Aspectos map[string]float64 `json:"aspectos"`
}

// ResumenResponse es el resumen por aspecto del negocio (P-50).
type ResumenResponse struct {
	Periodo      string           `json:"periodo"`
	Desde        time.Time        `json:"desde"`
	Actual       Promedios        `json:"actual"`
	Anterior     Promedios        `json:"anterior"`
	Distribucion map[string]int64 `json:"distribucion"`
}

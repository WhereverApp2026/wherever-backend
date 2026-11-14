package cupones

import (
	"time"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
)

// CrearRequest es el cuerpo de POST /negocios/{id}/cupones. Con `relampago` el servidor fija
// la vigencia (ahora a ahora + 3 h) e ignora la que llegue.
type CrearRequest struct {
	Titulo         string `json:"titulo" validate:"required,min=1,max=40"`
	Tipo           string `json:"tipo" validate:"required,oneof=porcentaje monto dos_por_uno regalo"`
	Valor          *int64 `json:"valor"`
	Condiciones    string `json:"condiciones" validate:"omitempty,max=200"`
	VigenciaInicio string `json:"vigencia_inicio" validate:"required_without=Relampago,omitempty,datetime=2006-01-02T15:04:05Z07:00"`
	VigenciaFin    string `json:"vigencia_fin" validate:"required_without=Relampago,omitempty,datetime=2006-01-02T15:04:05Z07:00"`
	Relampago      bool   `json:"relampago"`
	DiasValidos    []int  `json:"dias_validos" validate:"omitempty,max=7,unique,dive,min=0,max=6"`
	HorasValidas   *Horas `json:"horas_validas" validate:"omitempty"`
	LimiteUsos     int    `json:"limite_usos" validate:"omitempty,min=1,max=100000"`
}

// EditarRequest es el cuerpo de PATCH /negocios/{id}/cupones/{cid}: solo cambia lo que trae.
type EditarRequest struct {
	Titulo         *string `json:"titulo" validate:"omitempty,min=1,max=40"`
	Tipo           *string `json:"tipo" validate:"omitempty,oneof=porcentaje monto dos_por_uno regalo"`
	Valor          *int64  `json:"valor"`
	Condiciones    *string `json:"condiciones" validate:"omitempty,max=200"`
	VigenciaInicio *string `json:"vigencia_inicio" validate:"omitempty,datetime=2006-01-02T15:04:05Z07:00"`
	VigenciaFin    *string `json:"vigencia_fin" validate:"omitempty,datetime=2006-01-02T15:04:05Z07:00"`
	DiasValidos    *[]int  `json:"dias_validos" validate:"omitempty,max=7,unique,dive,min=0,max=6"`
	HorasValidas   *Horas  `json:"horas_validas"`
	LimiteUsos     *int    `json:"limite_usos" validate:"omitempty,min=0,max=100000"`
	Estado         *string `json:"estado" validate:"omitempty,oneof=activo pausado"`
}

// ListaNegocioQuery es GET /negocios/{id}/cupones.
type ListaNegocioQuery struct {
	Estado string `query:"estado" validate:"omitempty,oneof=programado activo pausado agotado vencido"`
	Cursor string `query:"cursor" validate:"omitempty,max=300,cursor"`
	Limite int    `query:"limite" validate:"min=1,max=50"`
}

// NewListaNegocioQuery trae el límite por defecto.
func NewListaNegocioQuery() ListaNegocioQuery {
	return ListaNegocioQuery{Limite: pagination.DefaultLimit}
}

// EstadisticasQuery es GET /negocios/{id}/cupones/{cid}/estadisticas (días UTC, ≤ 90).
type EstadisticasQuery struct {
	Desde string `query:"desde" validate:"omitempty,datetime=2006-01-02"`
	Hasta string `query:"hasta" validate:"omitempty,datetime=2006-01-02"`
}

// SugerirRequest es el cuerpo de POST /negocios/{id}/cupones/sugerir-titulo.
type SugerirRequest struct {
	Tipo        string `json:"tipo" validate:"required,oneof=porcentaje monto dos_por_uno regalo"`
	Valor       *int64 `json:"valor"`
	Descripcion string `json:"descripcion" validate:"omitempty,max=200"`
}

// ValidarRequest es el cuerpo de POST /negocios/{id}/cupones/validar.
type ValidarRequest struct {
	Codigo string `json:"codigo" validate:"required,max=12"`
}

// CercaQuery es GET /cupones/cerca.
type CercaQuery struct {
	Lat       *float64 `query:"lat" validate:"required,latitude"`
	Lng       *float64 `query:"lng" validate:"required,longitude"`
	RadioKm   float64  `query:"radio_km" validate:"min=0.1,max=20"`
	Categoria string   `query:"categoria" validate:"omitempty,max=40"`
	Cursor    string   `query:"cursor" validate:"omitempty,max=300,cursor"`
	Limite    int      `query:"limite" validate:"min=1,max=50"`
}

// NewCercaQuery trae los valores por defecto (5 km, 20).
func NewCercaQuery() CercaQuery { return CercaQuery{RadioKm: 5, Limite: pagination.DefaultLimit} }

// MisQuery es GET /mis-cupones.
type MisQuery struct {
	Filtro string `query:"filtro" validate:"omitempty,oneof=vigentes por_vencer usados"`
	Q      string `query:"q" validate:"omitempty,max=60"`
	Cursor string `query:"cursor" validate:"omitempty,max=300,cursor"`
	Limite int    `query:"limite" validate:"min=1,max=50"`
}

// NewMisQuery trae los valores por defecto (vigentes, 20).
func NewMisQuery() MisQuery { return MisQuery{Filtro: "vigentes", Limite: pagination.DefaultLimit} }

// CuponResponse es el cupón para el dueño (con contadores) y para el público (sin ellos).
type CuponResponse struct {
	ID             string    `json:"id"`
	NegocioID      string    `json:"negocio_id"`
	NegocioNombre  string    `json:"negocio_nombre"`
	Titulo         string    `json:"titulo"`
	Tipo           string    `json:"tipo"`
	Valor          *int64    `json:"valor"`
	Condiciones    string    `json:"condiciones"`
	VigenciaInicio time.Time `json:"vigencia_inicio"`
	VigenciaFin    time.Time `json:"vigencia_fin"`
	Relampago      bool      `json:"relampago"`
	DiasValidos    []int     `json:"dias_validos"`
	HorasValidas   *Horas    `json:"horas_validas"`
	Categoria      string    `json:"categoria"`
	Estado         string    `json:"estado"`
	LimiteUsos     *int      `json:"limite_usos,omitempty"`
	UsosActuales   *int      `json:"usos_actuales,omitempty"`
	Vistas         *int      `json:"vistas,omitempty"`
	Guardados      *int      `json:"guardados,omitempty"`
	Version        *int      `json:"version,omitempty"`
	DistanciaM     *float64  `json:"distancia_m,omitempty"`
}

// CarteraResponse es un cupón guardado.
type CarteraResponse struct {
	ID                string     `json:"id"`
	CuponID           string     `json:"cupon_id"`
	NegocioID         string     `json:"negocio_id"`
	Titulo            string     `json:"titulo"`
	Tipo              string     `json:"tipo"`
	Valor             *int64     `json:"valor"`
	NegocioNombre     string     `json:"negocio_nombre"`
	Estado            string     `json:"estado"`
	VenceEn           time.Time  `json:"vence_en"`
	SegundosRestantes *int64     `json:"segundos_restantes,omitempty"`
	CanjeadoEn        *time.Time `json:"canjeado_en,omitempty"`
	CreadoEn          time.Time  `json:"creado_en"`
}

// CanjeResponse es el código de un solo uso y el texto del QR.
type CanjeResponse struct {
	Codigo   string    `json:"codigo"`
	QR       string    `json:"qr"`
	ExpiraEn time.Time `json:"expira_en"`
}

// ValidarResponse es el verde (200) o el rojo (409) del escaneo.
type ValidarResponse struct {
	Valido   bool           `json:"valido"`
	Cupon    *CuponResponse `json:"cupon,omitempty"`
	Repetido bool           `json:"repetido,omitempty"`
	Motivo   string         `json:"motivo,omitempty"`
}

// MetricaResponse es un día de estadísticas.
type MetricaResponse struct {
	Dia       string `json:"dia"`
	Vistas    int    `json:"vistas"`
	Guardados int    `json:"guardados"`
	Canjes    int    `json:"canjes"`
}

// EstadisticasResponse son los días del periodo (con ceros) y los totales.
type EstadisticasResponse struct {
	CuponID string            `json:"cupon_id"`
	Dias    []MetricaResponse `json:"dias"`
	Totales MetricaResponse   `json:"totales"`
}

// SugerenciaResponse es el título sugerido.
type SugerenciaResponse struct {
	Titulo string `json:"titulo"`
}

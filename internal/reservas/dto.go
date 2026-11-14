package reservas

import (
	"time"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
)

// DisponibilidadQuery es GET /lugares/{id}/disponibilidad.
type DisponibilidadQuery struct {
	Fecha    string `query:"fecha" validate:"required,datetime=2006-01-02"`
	Personas int    `query:"personas" validate:"required,min=1,max=20"`
}

// DisponibilidadResponse son los horarios del día con cupo.
type DisponibilidadResponse struct {
	Fecha    string    `json:"fecha"`
	Horarios []Horario `json:"horarios"`
}

// CrearRequest es el cuerpo de POST /reservas.
type CrearRequest struct {
	NegocioID string `json:"negocio_id" validate:"required,objectid"`
	FechaHora string `json:"fecha_hora" validate:"required,datetime=2006-01-02T15:04:05Z07:00"`
	Personas  int    `json:"personas" validate:"required,min=1,max=20"`
	Nota      string `json:"nota" validate:"omitempty,max=140"`
	PlanID    string `json:"plan_id" validate:"omitempty,objectid"`
	ParadaID  string `json:"parada_id" validate:"omitempty,objectid"`
	GrupoID   string `json:"grupo_id" validate:"omitempty,objectid"`
}

// CambiarRequest es el cuerpo de PATCH /reservas/{id}: solo cambia lo que trae.
type CambiarRequest struct {
	FechaHora *string `json:"fecha_hora" validate:"omitempty,datetime=2006-01-02T15:04:05Z07:00"`
	Personas  *int    `json:"personas" validate:"omitempty,min=1,max=20"`
}

// CancelarRequest es el cuerpo (opcional) de POST /reservas/{id}/cancelar.
type CancelarRequest struct {
	Motivo string `json:"motivo" validate:"omitempty,max=140"`
}

// MiasQuery es GET /reservas.
type MiasQuery struct {
	Cuando string `query:"cuando" validate:"omitempty,oneof=proximas pasadas"`
	Estado string `query:"estado" validate:"omitempty,oneof=bloqueada pendiente confirmada asistio no_asistio expirada cancelada"`
	Cursor string `query:"cursor" validate:"omitempty,max=300,cursor"`
	Limite int    `query:"limite" validate:"min=1,max=50"`
}

// NewMiasQuery trae los valores por defecto (próximas, 20).
func NewMiasQuery() MiasQuery { return MiasQuery{Cuando: "proximas", Limite: pagination.DefaultLimit} }

// AgendaQuery es GET /negocios/{id}/reservas.
type AgendaQuery struct {
	Fecha  string `query:"fecha" validate:"omitempty,datetime=2006-01-02"`
	Estado string `query:"estado" validate:"omitempty,oneof=bloqueada pendiente confirmada asistio no_asistio expirada cancelada"`
	Q      string `query:"q" validate:"omitempty,max=60"`
}

// EstadoRequest es el cuerpo de PATCH /negocios/{id}/reservas/{rid}.
type EstadoRequest struct {
	Estado string `json:"estado" validate:"required,oneof=confirmada asistio no_asistio"`
}

// EscanearRequest es el cuerpo de POST /negocios/{id}/reservas/escanear.
type EscanearRequest struct {
	Codigo string `json:"codigo" validate:"required,len=6,alphanum"`
}

// BloqueoRequest es el cuerpo de POST /negocios/{id}/bloqueos.
type BloqueoRequest struct {
	Desde  string `json:"desde" validate:"required,datetime=2006-01-02T15:04:05Z07:00"`
	Hasta  string `json:"hasta" validate:"required,datetime=2006-01-02T15:04:05Z07:00"`
	Motivo string `json:"motivo" validate:"omitempty,oneof=evento_privado lleno otro"`
}

// ReservaResponse es una reserva vista por la persona o por el negocio.
type ReservaResponse struct {
	ID                string     `json:"id"`
	Codigo            string     `json:"codigo"`
	NegocioID         string     `json:"negocio_id"`
	PlanID            string     `json:"plan_id,omitempty"`
	ParadaID          string     `json:"parada_id,omitempty"`
	GrupoID           string     `json:"grupo_id,omitempty"`
	FechaHora         time.Time  `json:"fecha_hora"`
	Personas          int        `json:"personas"`
	Nota              string     `json:"nota,omitempty"`
	Estado            string     `json:"estado"`
	BloqueoExpiraEn   *time.Time `json:"bloqueo_expira_en,omitempty"`
	Anticipo          Anticipo   `json:"anticipo"`
	Politica          Politica   `json:"politica_cancelacion"`
	ReembolsoCentavos int64      `json:"reembolso_centavos"`
	PuedeCambiar      bool       `json:"puede_cambiar"`
	PuedeCancelar     bool       `json:"puede_cancelar"`
	Version           int        `json:"version"`
	CreadoEn          time.Time  `json:"creado_en"`
}

// ReembolsoResponse es el cálculo previo a cancelar.
type ReembolsoResponse struct {
	ReembolsoCentavos int64    `json:"reembolso_centavos"`
	Porcentaje        int      `json:"porcentaje"`
	Politica          Politica `json:"politica"`
}

// AgendaItem es una fila de la agenda del negocio: sin correo ni teléfono del cliente.
type AgendaItem struct {
	ID        string    `json:"id"`
	Codigo    string    `json:"codigo"`
	FechaHora time.Time `json:"fecha_hora"`
	Cliente   string    `json:"cliente"`
	Personas  int       `json:"personas"`
	Nota      string    `json:"nota,omitempty"`
	Anticipo  Anticipo  `json:"anticipo"`
	Estado    string    `json:"estado"`
	Version   int       `json:"version"`
}

// AgendaResponse es la agenda de un día.
type AgendaResponse struct {
	Fecha string       `json:"fecha"`
	Items []AgendaItem `json:"items"`
}

// BloqueoResponse es un bloqueo de horas creado.
type BloqueoResponse struct {
	ID     string    `json:"id"`
	Desde  time.Time `json:"desde"`
	Hasta  time.Time `json:"hasta"`
	Motivo string    `json:"motivo"`
}

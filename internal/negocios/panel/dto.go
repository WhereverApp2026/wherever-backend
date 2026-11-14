// Package panel es lo que el comerciante ve y edita de su negocio ya aprobado (spec nv-56, hoja
// A5): resumen del día, perfil y horarios, pausa, «Horas y cupo», QR de mostrador, baja y el
// Sello del administrador. Usa `internal/negocios` (modelo, repositorio e interfaces) y el menú;
// ninguno de los dos lo usa a él.
package panel

import (
	"time"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios/alta"
)

// PerfilRequest cambia solo los campos que vienen. `nombre`, `direccion` y `ubicacion` quedan
// en `cambios_pendientes` hasta que el administrador los aplique.
type PerfilRequest struct {
	Nombre         *string                 `json:"nombre" validate:"omitempty,min=3,max=60"`
	Descripcion    *string                 `json:"descripcion" validate:"omitempty,max=300"`
	Categoria      *string                 `json:"categoria" validate:"omitempty,max=30"`
	Direccion      *negocios.Direccion     `json:"direccion"`
	Ubicacion      *alta.Ubicacion         `json:"ubicacion"`
	Horarios       *[]negocios.Horario     `json:"horarios" validate:"omitempty,min=1,max=14"`
	NivelPrecio    *int                    `json:"nivel_precio"`
	PrecioPromedio *int                    `json:"precio_promedio"`
	Servicios      *[]string               `json:"servicios" validate:"omitempty,max=20,dive,min=1,max=40"`
	OpcionesDieta  *[]string               `json:"opciones_dieta"`
	Alergenos      *[]string               `json:"alergenos"`
	Accesibilidad  *negocios.Accesibilidad `json:"accesibilidad"`
	Telefono       *string                 `json:"telefono" validate:"omitempty,max=20"`
}

// PausaRequest pausa o reanuda. `hasta` es opcional (máximo 90 días).
type PausaRequest struct {
	Pausado *bool      `json:"pausado" validate:"required"`
	Hasta   *time.Time `json:"hasta"`
	Motivo  string     `json:"motivo" validate:"max=120"`
}

// AnticipoRequest es el anticipo de una reserva: fijo y por persona en centavos, porcentaje 0–100.
type AnticipoRequest struct {
	Tipo  string `json:"tipo" validate:"required,oneof=fijo porcentaje por_persona"`
	Valor int    `json:"valor" validate:"gte=0,lte=10000000"`
}

// ReservasConfigRequest es «Horas y cupo» (US-078); lo lee nv-59.
type ReservasConfigRequest struct {
	AceptaReservas      *bool           `json:"acepta_reservas" validate:"required"`
	CapacidadPorHorario int             `json:"capacidad_por_horario" validate:"required,min=1,max=500"`
	DuracionTurnoMin    int             `json:"duracion_turno_min" validate:"required,min=15,max=240"`
	Anticipo            AnticipoRequest `json:"anticipo"`
	PoliticaCancelacion string          `json:"politica_cancelacion" validate:"required,oneof=flexible moderada estricta"`
	Confirmacion        string          `json:"confirmacion" validate:"required,oneof=automatica manual"`
	PermiteCambios      *bool           `json:"permite_cambios" validate:"required"`
}

// BajaRequest confirma la baja con el nombre exacto del negocio.
type BajaRequest struct {
	Confirmacion string `json:"confirmacion" validate:"required,max=60"`
}

// SelloRequest es la decisión del administrador sobre el Sello de Excelencia.
type SelloRequest struct {
	Otorgar *bool  `json:"otorgar" validate:"required"`
	Motivo  string `json:"motivo" validate:"required,min=10,max=500"`
}

// PausaResponse es la pausa vigente.
type PausaResponse struct {
	Desde  *time.Time `json:"desde"`
	Hasta  *time.Time `json:"hasta"`
	Motivo string     `json:"motivo"`
}

// PerfilResponse es el negocio como lo edita su dueño en P-52. Nunca lleva el expediente.
type PerfilResponse struct {
	ID                string                   `json:"id"`
	Nombre            string                   `json:"nombre"`
	Categoria         string                   `json:"categoria"`
	Descripcion       string                   `json:"descripcion"`
	Direccion         negocios.Direccion       `json:"direccion"`
	Ubicacion         *alta.Ubicacion          `json:"ubicacion"`
	Telefono          string                   `json:"telefono"`
	Horarios          []negocios.Horario       `json:"horarios"`
	NivelPrecio       int                      `json:"nivel_precio"`
	PrecioPromedio    int                      `json:"precio_promedio"`
	Servicios         []string                 `json:"servicios"`
	OpcionesDieta     []string                 `json:"opciones_dieta"`
	Alergenos         []string                 `json:"alergenos"`
	Accesibilidad     negocios.Accesibilidad   `json:"accesibilidad"`
	Estado            string                   `json:"estado"`
	Pausado           *PausaResponse           `json:"pausado"`
	CambiosPendientes *alta.CambiosResponse    `json:"cambios_pendientes"`
	ReservasConfig    *negocios.ReservasConfig `json:"reservas_config"`
	SelloExcelencia   bool                     `json:"sello_excelencia"`
	Version           int                      `json:"version"`
	ActualizadoEn     time.Time                `json:"actualizado_en"`
}

// ReservasConfigResponse es la configuración guardada con la versión nueva del negocio.
type ReservasConfigResponse struct {
	ReservasConfig negocios.ReservasConfig `json:"reservas_config"`
	Version        int                     `json:"version"`
}

// QRResponse es el contenido del QR de mostrador; la app lo dibuja e imprime.
type QRResponse struct {
	Contenido string `json:"contenido"`
}

// SelloResponse es el estado del Sello tras la decisión.
type SelloResponse struct {
	NegocioID       string    `json:"negocio_id"`
	SelloExcelencia bool      `json:"sello_excelencia"`
	Motivo          string    `json:"motivo"`
	DecididoEn      time.Time `json:"decidido_en"`
	Version         int       `json:"version"`
}

// CuentaResponse es la cuenta de depósito enmascarada (sin titular).
type CuentaResponse struct {
	CLABEUltimos4 string `json:"clabe_ultimos4"`
	Banco         string `json:"banco"`
	Estado        string `json:"estado"`
}

// ProximaResponse es la siguiente reserva del día.
type ProximaResponse struct {
	Hora     time.Time `json:"hora"`
	Personas int       `json:"personas"`
}

// CambiosResumen dice qué campos esperan al administrador.
type CambiosResumen struct {
	Campos    []string  `json:"campos"`
	EnviadoEn time.Time `json:"enviado_en"`
}

// PanelResponse es el resumen del día (P-47). Un campo de una fuente sin registrar o caída va
// en null y la app muestra «—».
type PanelResponse struct {
	NegocioID              string           `json:"negocio_id"`
	Nombre                 string           `json:"nombre"`
	Estado                 string           `json:"estado"`
	Pausado                *PausaResponse   `json:"pausado"`
	CambiosPendientes      *CambiosResumen  `json:"cambios_pendientes"`
	DatosFiscalesCompletos *bool            `json:"datos_fiscales_completos"`
	CuentaDeposito         *CuentaResponse  `json:"cuenta_deposito"`
	ReservasHoy            *int             `json:"reservas_hoy"`
	ProximaReserva         *ProximaResponse `json:"proxima_reserva"`
	CuponesCanjeadosHoy    *int             `json:"cupones_canjeados_hoy"`
	CuponesActivos         *int             `json:"cupones_activos"`
	LlegadasHoy            *int             `json:"llegadas_hoy"`
	CalificacionSemana     *float64         `json:"calificacion_semana"`
	CambioVsSemanaAnterior *float64         `json:"cambio_vs_semana_anterior"`
	ResenasPorResponder    *int             `json:"resenas_por_responder"`
	SelloExcelencia        bool             `json:"sello_excelencia"`
	AvanceSello            AvanceSello      `json:"avance_sello"`
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

func pausaResponse(p *negocios.Pausa) *PausaResponse {
	if p == nil {
		return nil
	}
	return &PausaResponse{Desde: p.Desde, Hasta: p.Hasta, Motivo: p.Motivo}
}

func perfilResponse(b negocios.Business) PerfilResponse {
	out := PerfilResponse{
		ID: b.ID.Hex(), Nombre: b.Name, Categoria: b.Category, Descripcion: b.Description, Direccion: b.Address, Telefono: b.Phone,
		Horarios: nonNil(b.Schedules), NivelPrecio: b.PriceLevel, PrecioPromedio: b.AveragePrice, Servicios: nonNil(b.Services),
		OpcionesDieta: nonNil(b.DietOptions), Alergenos: nonNil(b.Allergens), Accesibilidad: b.Accessibility, Estado: b.State,
		Pausado: pausaResponse(b.Paused), ReservasConfig: b.ReservasConfig, SelloExcelencia: b.ExcellenceSeal, Version: b.Version,
		ActualizadoEn: b.UpdatedAt.UTC(),
	}
	if b.Location.Type == "Point" {
		out.Ubicacion = &alta.Ubicacion{Type: b.Location.Type, Coordinates: b.Location.Coordinates}
	}
	if c := b.CambiosPendientes; c != nil {
		out.CambiosPendientes = &alta.CambiosResponse{Campos: nonNil(c.Campos), Nombre: c.Nombre, Direccion: c.Direccion, EnviadoEn: c.EnviadoEn.UTC()}
		if c.Ubicacion != nil {
			out.CambiosPendientes.Ubicacion = &alta.Ubicacion{Type: c.Ubicacion.Type, Coordinates: c.Ubicacion.Coordinates}
		}
	}
	return out
}

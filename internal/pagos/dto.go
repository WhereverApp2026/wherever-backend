package pagos

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
)

// Ningún DTO de este paquete tiene campos de tarjeta: la tarjeta la captura la PaymentSheet de
// Stripe en la app. Los cuerpos se leen con DecodeEstricto, que rechaza campos desconocidos
// (400): un `numero` o `cvc` nunca llega al service.

// CobrarRequest es el cuerpo de POST /reservas/{id}/pago: vacío (el monto sale de la reserva).
type CobrarRequest struct{}

// PagoResponse lleva el `client_secret` para la PaymentSheet.
type PagoResponse struct {
	PagoID        string `json:"pago_id"`
	ClientSecret  string `json:"client_secret"`
	MontoCentavos int64  `json:"monto_centavos"`
	Moneda        string `json:"moneda"`
}

// EstadoResponse es GET /pagos/{id}/estado.
type EstadoResponse struct {
	Estado        string `json:"estado"`
	ReservaEstado string `json:"reserva_estado"`
}

// CobroResponse es lo que recibe el negocio de una reserva.
type CobroResponse struct {
	Anticipo          int64  `json:"anticipo"`
	Reembolsos        int64  `json:"reembolsos"`
	Comision          int64  `json:"comision"`
	Neto              int64  `json:"neto"`
	Estado            string `json:"estado"`
	LiquidacionSemana string `json:"liquidacion_semana,omitempty"`
}

// ConectarResponse es el enlace de registro de Stripe.
type ConectarResponse struct {
	URL string `json:"url"`
}

// SaldoResponse es el saldo del negocio en centavos.
type SaldoResponse struct {
	Disponible      int64  `json:"disponible"`
	EnEspera        int64  `json:"en_espera"`
	ProximoDeposito string `json:"proximo_deposito"`
}

// LiquidacionesQuery es la paginación de GET /negocios/{id}/liquidaciones.
type LiquidacionesQuery struct {
	Cursor string `query:"cursor" validate:"omitempty,max=512"`
	Limite int    `query:"limite" validate:"min=1,max=50"`
}

// NewLiquidacionesQuery da el límite por defecto.
func NewLiquidacionesQuery() LiquidacionesQuery {
	return LiquidacionesQuery{Limite: pagination.DefaultLimit}
}

// LiquidacionResponse es una liquidación semanal (sin ids de Stripe).
type LiquidacionResponse struct {
	ID             string `json:"id"`
	SemanaInicio   string `json:"semana_inicio"`
	Pagos          int    `json:"pagos"`
	TotalAnticipos int64  `json:"total_anticipos"`
	Comision       int64  `json:"comision"`
	Reembolsos     int64  `json:"reembolsos"`
	Neto           int64  `json:"neto"`
	Estado         string `json:"estado"`
}

func liquidacionResponse(l Liquidacion, loc *time.Location) LiquidacionResponse {
	return LiquidacionResponse{
		ID: l.ID.Hex(), SemanaInicio: fecha(l.SemanaInicio, loc), Pagos: l.Pagos, TotalAnticipos: l.TotalAnticipos,
		Comision: l.Comision, Reembolsos: l.Reembolsos, Neto: l.Neto, Estado: l.Estado,
	}
}

// DecodeEstricto lee un JSON que solo admite los campos del DTO; un cuerpo vacío es `{}`.
func DecodeEstricto(body []byte, dto any) error {
	if len(bytes.TrimSpace(body)) == 0 {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dto); err != nil {
		return err
	}
	if dec.More() {
		return errCuerpo
	}
	return nil
}

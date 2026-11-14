// Package pagos cobra el anticipo de las reservas con Stripe (nv-60, hojas J5 y A5): crea el
// PaymentIntent que la app paga con la PaymentSheet, confirma la reserva con el webhook firmado,
// pide los reembolsos y deposita cada semana a los negocios por Stripe Connect Express. El
// backend nunca recibe el número de tarjeta ni el CVC.
package pagos

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Colecciones (J5 §2 y A5).
const (
	Collection              = "pagos"
	LiquidacionesCollection = "liquidaciones"
)

// Estados de un pago.
const (
	EstadoRequierePago       = "requiere_pago"
	EstadoProcesando         = "procesando"
	EstadoAprobado           = "aprobado"
	EstadoRechazado          = "rechazado"
	EstadoReembolsadoParcial = "reembolsado_parcial"
	EstadoReembolsado        = "reembolsado"
)

// Estados de una liquidación semanal.
const (
	LiquidacionPendiente   = "pendiente"
	LiquidacionTransferida = "transferida"
)

// Constantes de la spec.
const (
	ConceptoAnticipo   = "anticipo_reserva"
	Moneda             = "mxn"
	Conservacion       = 5 // años (Código Fiscal de la Federación)
	MaxEventos         = 20
	ComisionPctInicial = 10 // decisión del revisor (2026-11-14)
	ConsultaProcesando = 30 * time.Second
)

// Tarjeta es lo único que se guarda de la tarjeta (aviso de privacidad).
type Tarjeta struct {
	Marca    string `bson:"marca" json:"marca"`
	Ultimos4 string `bson:"ultimos4" json:"ultimos4"`
}

// Reembolso es una devolución pedida a Stripe.
type Reembolso struct {
	RefundID      string    `bson:"refund_id" json:"-"`
	MontoCentavos int64     `bson:"monto_centavos" json:"monto_centavos"`
	Fecha         time.Time `bson:"fecha" json:"fecha"`
	Motivo        string    `bson:"motivo,omitempty" json:"motivo,omitempty"`
}

// Pago es un documento de `pagos`: solo ids, montos, estado y la marca y últimos 4 de la tarjeta.
type Pago struct {
	ID                bson.ObjectID `bson:"_id"`
	ReservaID         bson.ObjectID `bson:"reserva_id"`
	UsuarioID         bson.ObjectID `bson:"usuario_id"`
	NegocioID         bson.ObjectID `bson:"negocio_id"`
	Concepto          string        `bson:"concepto"`
	PaymentIntentID   string        `bson:"payment_intent_id"`
	MontoCentavos     int64         `bson:"monto_centavos"`
	ComisionCentavos  int64         `bson:"comision_centavos"`
	Moneda            string        `bson:"moneda"`
	Estado            string        `bson:"estado"`
	MotivoRechazo     string        `bson:"motivo_rechazo,omitempty"`
	Tarjeta           *Tarjeta      `bson:"tarjeta,omitempty"`
	Reembolsos        []Reembolso   `bson:"reembolsos"`
	IdempotencyKey    string        `bson:"idempotency_key"`
	FechaReserva      time.Time     `bson:"fecha_reserva"`
	LiquidacionSemana *time.Time    `bson:"liquidacion_semana,omitempty"`
	EventosProcesados []string      `bson:"eventos_procesados"`
	ConservarHasta    time.Time     `bson:"conservar_hasta"`
	CreadoEn          time.Time     `bson:"creado_en"`
	ActualizadoEn     time.Time     `bson:"actualizado_en"`
}

// Reembolsado suma lo devuelto.
func (p Pago) Reembolsado() int64 {
	var total int64
	for _, r := range p.Reembolsos {
		total += r.MontoCentavos
	}
	return min(total, p.MontoCentavos)
}

// Abierto dice si el pago todavía puede cobrarse (se reutiliza su PaymentIntent).
func (p Pago) Abierto() bool { return p.Estado == EstadoRequierePago || p.Estado == EstadoProcesando }

// Liquidacion es un documento de `liquidaciones`: lo que se transfiere a un negocio por semana.
type Liquidacion struct {
	ID               bson.ObjectID `bson:"_id"`
	NegocioID        bson.ObjectID `bson:"negocio_id"`
	SemanaInicio     time.Time     `bson:"semana_inicio"`
	Pagos            int           `bson:"pagos"`
	TotalAnticipos   int64         `bson:"total_anticipos"`
	Comision         int64         `bson:"comision"`
	Reembolsos       int64         `bson:"reembolsos"`
	Neto             int64         `bson:"neto"`
	Estado           string        `bson:"estado"`
	StripeTransferID string        `bson:"stripe_transfer_id,omitempty"`
	ConservarHasta   time.Time     `bson:"conservar_hasta"`
	CreadoEn         time.Time     `bson:"creado_en"`
	ActualizadoEn    time.Time     `bson:"actualizado_en"`
}

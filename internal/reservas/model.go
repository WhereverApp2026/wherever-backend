// Package reservas aparta cupos en los negocios (nv-59, hoja J5): disponibilidad por horario,
// reservas sin sobreventa (candado en Redis), agenda del negocio, bloqueos de horas y reembolso
// según la política. El cobro del anticipo y el reembolso real los entrega nv-60 por los puertos
// Anticipos y Refunder.
package reservas

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Colecciones (J5 §1 y §3).
const (
	Collection         = "reservas"
	BloqueosCollection = "bloqueos_horario"
)

// Estados de una reserva.
const (
	EstadoBloqueada  = "bloqueada"
	EstadoPendiente  = "pendiente"
	EstadoConfirmada = "confirmada"
	EstadoAsistio    = "asistio"
	EstadoNoAsistio  = "no_asistio"
	EstadoExpirada   = "expirada"
	EstadoCancelada  = "cancelada"
)

// Topes de la spec.
const (
	MaxPersonas       = 20
	MaxNota           = 140
	MaxApartados      = 3
	MaxDiasAdelante   = 60
	MaxDiasBloqueo    = 31
	MinHorasCambio    = 2
	ApartadoDuracion  = 10 * time.Minute
	CodigoLongitud    = 6
	MaxAgendaDia      = 500
	CuentaEliminada   = "Cuenta eliminada"
	motivoNegocioBaja = "negocio_dado_de_baja"
)

// Quién hizo un cambio de estado (historial).
const (
	PorCliente = "cliente"
	PorNegocio = "negocio"
	PorSistema = "sistema"
)

// Motivos de un bloqueo de horas.
const (
	MotivoEventoPrivado = "evento_privado"
	MotivoLleno         = "lleno"
	MotivoOtro          = "otro"
)

// Anticipo es lo que se cobra por adelantado, calculado al reservar.
type Anticipo struct {
	MontoCentavos int64  `bson:"monto_centavos" json:"monto_centavos"`
	Tipo          string `bson:"tipo" json:"tipo"`
}

// Politica es la copia de la política de cancelación al reservar: hasta `horas_limite` antes
// se devuelve `porcentaje_antes`; después, `porcentaje_despues`.
type Politica struct {
	Tipo              string `bson:"tipo" json:"tipo"`
	HorasLimite       int    `bson:"horas_limite" json:"horas_limite"`
	PorcentajeAntes   int    `bson:"porcentaje_antes" json:"porcentaje_antes"`
	PorcentajeDespues int    `bson:"porcentaje_despues" json:"porcentaje_despues"`
}

// Cambio es una fila de `historial`.
type Cambio struct {
	Estado string    `bson:"estado"`
	Fecha  time.Time `bson:"fecha"`
	Por    string    `bson:"por"`
}

// Reserva es un documento de `reservas`.
type Reserva struct {
	ID              bson.ObjectID  `bson:"_id"`
	Codigo          string         `bson:"codigo"`
	UsuarioID       bson.ObjectID  `bson:"usuario_id"`
	NegocioID       bson.ObjectID  `bson:"negocio_id"`
	PlanID          *bson.ObjectID `bson:"plan_id,omitempty"`
	ParadaID        *bson.ObjectID `bson:"parada_id,omitempty"`
	GrupoID         *bson.ObjectID `bson:"grupo_id,omitempty"`
	FechaHora       time.Time      `bson:"fecha_hora"`
	Personas        int            `bson:"personas"`
	Nota            string         `bson:"nota,omitempty"`
	Estado          string         `bson:"estado"`
	BloqueoExpiraEn *time.Time     `bson:"bloqueo_expira_en,omitempty"`
	Anticipo        Anticipo       `bson:"anticipo"`
	Politica        Politica       `bson:"politica_cancelacion"`
	PagoID          string         `bson:"pago_id,omitempty"`
	Historial       []Cambio       `bson:"historial"`
	Version         int            `bson:"version"`
	CreadoEn        time.Time      `bson:"creado_en"`
	ActualizadoEn   time.Time      `bson:"actualizado_en"`
}

// Bloqueo es un documento de `bloqueos_horario`: el negocio cierra [Desde, Hasta).
type Bloqueo struct {
	ID        bson.ObjectID `bson:"_id"`
	NegocioID bson.ObjectID `bson:"negocio_id"`
	Desde     time.Time     `bson:"desde"`
	Hasta     time.Time     `bson:"hasta"`
	Motivo    string        `bson:"motivo"`
	CreadoPor bson.ObjectID `bson:"creado_por"`
	CreadoEn  time.Time     `bson:"creado_en"`
}

// activos son los estados que ocupan cupo (la `bloqueada` solo mientras no vence).
var activos = []string{EstadoBloqueada, EstadoPendiente, EstadoConfirmada}

// Ocupa dice si la reserva cuenta contra el cupo en `now`.
func (r Reserva) Ocupa(now time.Time) bool {
	switch r.Estado {
	case EstadoPendiente, EstadoConfirmada:
		return true
	case EstadoBloqueada:
		return r.BloqueoExpiraEn != nil && r.BloqueoExpiraEn.After(now)
	}
	return false
}

// Cancelable dice si la reserva todavía se puede cancelar.
func (r Reserva) Cancelable() bool {
	return r.Estado == EstadoBloqueada || r.Estado == EstadoPendiente || r.Estado == EstadoConfirmada
}

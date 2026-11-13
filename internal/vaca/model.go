// Package vaca registra los gastos compartidos de un grupo y calcula quién le paga a quién
// con el menor número de transferencias (nv-58, hoja J4). Wherever solo calcula: el dinero
// se mueve por el banco de cada persona. Todo en centavos enteros.
package vaca

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Colecciones (J4 §1 y §2).
const (
	ExpensesCollection = "gastos"
	DebtsCollection    = "deudas_vaca"
)

// Topes de la spec.
const (
	MaxExpenses     = 500
	MaxCents        = 10_000_000
	MinSplit        = 2
	MaxSplit        = 12
	MaxDescription  = 60
	DeletedUserName = "Cuenta eliminada"
)

// Conceptos de un gasto.
const (
	ConceptFood      = "comida"
	ConceptDrinks    = "bebidas"
	ConceptTransport = "transporte"
	ConceptEntry     = "entrada"
	ConceptOther     = "otro"
)

// Formas de dividir.
const (
	DivisionEqual  = "igual"
	DivisionCustom = "personalizada"
)

// Estados de una deuda.
const (
	DebtPending = "pendiente"
	DebtPaid    = "pagada"
)

// Share es lo que le toca a una persona en una división personalizada.
type Share struct {
	UserID bson.ObjectID `bson:"usuario_id"`
	Cents  int64         `bson:"monto_centavos"`
}

// Expense es un documento de `gastos`. El borrado es lógico (`eliminado_en`).
type Expense struct {
	ID          bson.ObjectID   `bson:"_id"`
	GroupID     bson.ObjectID   `bson:"grupo_id"`
	PlanID      *bson.ObjectID  `bson:"plan_id,omitempty"`
	Concept     string          `bson:"concepto"`
	Description string          `bson:"descripcion,omitempty"`
	Cents       int64           `bson:"monto_centavos"`
	PaidBy      bson.ObjectID   `bson:"pagado_por"`
	SplitAmong  []bson.ObjectID `bson:"dividir_entre"`
	Division    string          `bson:"division"`
	Custom      []Share         `bson:"montos_personalizados,omitempty"`
	BusinessID  *bson.ObjectID  `bson:"negocio_id,omitempty"`
	ClientID    string          `bson:"id_cliente"`
	CreatedBy   bson.ObjectID   `bson:"creado_por"`
	DeletedAt   *time.Time      `bson:"eliminado_en,omitempty"`
	Version     int             `bson:"version"`
	CreatedAt   time.Time       `bson:"creado_en"`
	UpdatedAt   time.Time       `bson:"actualizado_en"`
}

// Debt es un documento de `deudas_vaca`: una transferencia del corte.
type Debt struct {
	ID         bson.ObjectID `bson:"_id"`
	GroupID    bson.ObjectID `bson:"grupo_id"`
	Debtor     bson.ObjectID `bson:"deudor_id"`
	Creditor   bson.ObjectID `bson:"acreedor_id"`
	Cents      int64         `bson:"monto_centavos"`
	Status     string        `bson:"estado"`
	Reference  string        `bson:"referencia,omitempty"`
	PaidAt     *time.Time    `bson:"pagada_en,omitempty"`
	CutVersion int           `bson:"version_corte"`
	CreatedAt  time.Time     `bson:"creado_en"`
	UpdatedAt  time.Time     `bson:"actualizado_en"`
}

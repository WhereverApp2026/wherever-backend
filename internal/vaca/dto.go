package vaca

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
)

// ShareInput es una fila de `montos_personalizados`.
type ShareInput struct {
	UserID string `json:"usuario_id" validate:"required,objectid"`
	Cents  int64  `json:"monto_centavos" validate:"min=0,max=10000000"`
}

// CreateRequest es el cuerpo de POST /grupos/{id}/gastos.
type CreateRequest struct {
	Cents       int64        `json:"monto_centavos" validate:"required,min=1,max=10000000"`
	Concept     string       `json:"concepto" validate:"required,oneof=comida bebidas transporte entrada otro"`
	Description string       `json:"descripcion" validate:"omitempty,max=60"`
	PaidBy      string       `json:"pagado_por" validate:"required,objectid"`
	SplitAmong  []string     `json:"dividir_entre" validate:"omitempty,min=2,max=12,unique,dive,objectid"`
	Division    string       `json:"division" validate:"required,oneof=igual personalizada"`
	Custom      []ShareInput `json:"montos_personalizados" validate:"omitempty,max=12,dive"`
	PlanID      string       `json:"plan_id" validate:"omitempty,objectid"`
	BusinessID  string       `json:"negocio_id" validate:"omitempty,objectid"`
	ClientID    string       `json:"id_cliente" validate:"required,uuid"`
}

// PatchRequest es el cuerpo de PATCH /gastos/{id}: solo cambia lo que trae.
type PatchRequest struct {
	Cents       *int64        `json:"monto_centavos" validate:"omitempty,min=1,max=10000000"`
	Concept     *string       `json:"concepto" validate:"omitempty,oneof=comida bebidas transporte entrada otro"`
	Description *string       `json:"descripcion" validate:"omitempty,max=60"`
	PaidBy      *string       `json:"pagado_por" validate:"omitempty,objectid"`
	SplitAmong  *[]string     `json:"dividir_entre" validate:"omitempty,min=2,max=12,unique,dive,objectid"`
	Division    *string       `json:"division" validate:"omitempty,oneof=igual personalizada"`
	Custom      *[]ShareInput `json:"montos_personalizados" validate:"omitempty,max=12,dive"`
	Confirm     bool          `json:"confirmar"`
}

// MarkPaidRequest es el cuerpo de PATCH /vaca/deudas/{id}.
type MarkPaidRequest struct {
	Status    string `json:"estado" validate:"required,oneof=pagada"`
	Reference string `json:"referencia" validate:"omitempty,max=60"`
}

// ListQuery es GET /grupos/{id}/gastos.
type ListQuery struct {
	Concept string `query:"concepto" validate:"omitempty,oneof=comida bebidas transporte entrada otro"`
	PaidBy  string `query:"pagado_por" validate:"omitempty,objectid"`
	Cursor  string `query:"cursor" validate:"omitempty,max=300,cursor"`
	Limit   int    `query:"limite" validate:"min=1,max=50"`
}

// NewListQuery trae el límite por defecto (20).
func NewListQuery() ListQuery { return ListQuery{Limit: pagination.DefaultLimit} }

// MyDebtsQuery es GET /vaca/mis-deudas; sin `estado` trae las pendientes.
type MyDebtsQuery struct {
	Status string `query:"estado" validate:"omitempty,oneof=pendiente pagada"`
}

func invalid(field, message string) error {
	return apperr.Validation(message, apperr.FieldError{Field: field, Message: message})
}

func oid(raw string) bson.ObjectID {
	id, _ := bson.ObjectIDFromHex(raw) // ya validado como objectid
	return id
}

func optionalID(raw string) *bson.ObjectID {
	if raw == "" {
		return nil
	}
	id := oid(raw)
	return &id
}

func ids(raw []string) []bson.ObjectID {
	out := make([]bson.ObjectID, 0, len(raw))
	for _, r := range raw {
		out = append(out, oid(r))
	}
	return out
}

func shares(raw []ShareInput) []Share {
	out := make([]Share, 0, len(raw))
	for _, s := range raw {
		out = append(out, Share{UserID: oid(s.UserID), Cents: s.Cents})
	}
	return out
}

// CheckSplit valida las reglas que cruzan campos de un gasto ya armado: 2 a 12 personas sin
// repetir, quién pagó y la división personalizada (una por persona y suma exacta). members
// son los integrantes confirmados del grupo.
func CheckSplit(e Expense, members map[bson.ObjectID]bool) error {
	if !members[e.PaidBy] {
		return invalid("pagado_por", "quien pagó no es integrante confirmado del grupo")
	}
	if len(e.SplitAmong) < MinSplit || len(e.SplitAmong) > MaxSplit {
		return invalid("dividir_entre", "se divide entre 2 y 12 personas")
	}
	seen := map[bson.ObjectID]bool{}
	for _, id := range e.SplitAmong {
		if seen[id] {
			return invalid("dividir_entre", "hay personas repetidas")
		}
		seen[id] = true
		if !members[id] {
			return invalid("dividir_entre", "todas las personas deben ser integrantes confirmados")
		}
	}
	if e.Division != DivisionCustom {
		if len(e.Custom) > 0 {
			return invalid("montos_personalizados", "solo van con división personalizada")
		}
		return nil
	}
	if len(e.Custom) != len(e.SplitAmong) {
		return invalid("montos_personalizados", "va un monto por cada persona de dividir_entre")
	}
	var sum int64
	given := map[bson.ObjectID]bool{}
	for _, s := range e.Custom {
		if !seen[s.UserID] || given[s.UserID] {
			return invalid("montos_personalizados", "va un monto por cada persona de dividir_entre")
		}
		given[s.UserID] = true
		sum += s.Cents
	}
	if sum != e.Cents {
		return invalid("montos_personalizados", "los montos deben sumar exacto el total")
	}
	return nil
}

// Person es una persona del corte con su nombre (o «Cuenta eliminada»).
type Person struct {
	UserID string `json:"usuario_id"`
	Name   string `json:"nombre"`
}

// ShareView es una fila de `montos_personalizados` en la respuesta.
type ShareView struct {
	UserID string `json:"usuario_id"`
	Cents  int64  `json:"monto_centavos"`
}

// ExpenseView es un gasto en la respuesta.
type ExpenseView struct {
	ID          string      `json:"id"`
	GroupID     string      `json:"grupo_id"`
	PlanID      string      `json:"plan_id,omitempty"`
	Concept     string      `json:"concepto"`
	Description string      `json:"descripcion,omitempty"`
	Cents       int64       `json:"monto_centavos"`
	PaidBy      Person      `json:"pagado_por"`
	SplitAmong  []string    `json:"dividir_entre"`
	Division    string      `json:"division"`
	Custom      []ShareView `json:"montos_personalizados,omitempty"`
	BusinessID  string      `json:"negocio_id,omitempty"`
	ClientID    string      `json:"id_cliente"`
	CreatedBy   string      `json:"creado_por"`
	Version     int         `json:"version"`
	CreatedAt   time.Time   `json:"creado_en"`
	UpdatedAt   time.Time   `json:"actualizado_en"`
}

// BalanceView es una persona en el corte.
type BalanceView struct {
	UserID  string `json:"usuario_id"`
	Name    string `json:"nombre"`
	Put     int64  `json:"puso"`
	Share   int64  `json:"le_toca"`
	Balance int64  `json:"saldo"`
}

// DebtView es una deuda en la respuesta.
type DebtView struct {
	ID         string     `json:"id"`
	GroupID    string     `json:"grupo_id"`
	Debtor     Person     `json:"deudor"`
	Creditor   Person     `json:"acreedor"`
	Cents      int64      `json:"monto_centavos"`
	Status     string     `json:"estado"`
	Reference  string     `json:"referencia,omitempty"`
	PaidAt     *time.Time `json:"pagada_en,omitempty"`
	CutVersion int        `json:"version_corte"`
}

// CutView es GET /grupos/{id}/vaca.
type CutView struct {
	Total   int64         `json:"total_centavos"`
	People  []BalanceView `json:"por_persona"`
	Debts   []DebtView    `json:"deudas"`
	Version int           `json:"version_corte"`
}

// ExpenseResult es la respuesta de POST y PATCH de gastos.
type ExpenseResult struct {
	Expense ExpenseView `json:"gasto"`
	Cut     CutView     `json:"corte"`
}

// ExpensePage es GET /grupos/{id}/gastos.
type ExpensePage struct {
	Items []ExpenseView `json:"items"`
	Next  string        `json:"siguiente"`
	Total int64         `json:"total_centavos"`
}

// MyDebtsView es GET /vaca/mis-deudas.
type MyDebtsView struct {
	Owe       []DebtView `json:"debo"`
	OwedToMe  []DebtView `json:"me_deben"`
	TotalOwe  int64      `json:"total_debo"`
	TotalOwed int64      `json:"total_me_deben"`
}

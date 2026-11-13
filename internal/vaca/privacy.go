package vaca

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// ExpensesPrivacy es la parte de `gastos` en ARCO (nv-05). Borrar la cuenta no borra gastos:
// son datos del grupo y el corte de los demás depende de ellos. Los nombres se resuelven al
// responder, así que la cuenta borrada ya sale como «Cuenta eliminada» sin reescribir nada.
type ExpensesPrivacy struct{ repo *Repository }

// DebtsPrivacy es la parte de `deudas_vaca` en ARCO, con la misma regla.
type DebtsPrivacy struct{ repo *Repository }

// NewExpensesPrivacy recibe el repositorio por constructor.
func NewExpensesPrivacy(repo *Repository) *ExpensesPrivacy { return &ExpensesPrivacy{repo: repo} }

// NewDebtsPrivacy recibe el repositorio por constructor.
func NewDebtsPrivacy(repo *Repository) *DebtsPrivacy { return &DebtsPrivacy{repo: repo} }

// Name identifica al módulo en el registro y en el JSON exportado.
func (p *ExpensesPrivacy) Name() string { return ExpensesCollection }

// Name identifica al módulo en el registro y en el JSON exportado.
func (p *DebtsPrivacy) Name() string { return DebtsCollection }

// PurgeHistory no toca la Vaca: "borrar historial" no aplica a datos del grupo.
func (p *ExpensesPrivacy) PurgeHistory(context.Context, bson.ObjectID) error { return nil }

// PurgeAccount conserva los gastos (ver el comentario del tipo).
func (p *ExpensesPrivacy) PurgeAccount(context.Context, bson.ObjectID) error { return nil }

// PurgeHistory no toca la Vaca.
func (p *DebtsPrivacy) PurgeHistory(context.Context, bson.ObjectID) error { return nil }

// PurgeAccount conserva las deudas (ver ExpensesPrivacy).
func (p *DebtsPrivacy) PurgeAccount(context.Context, bson.ObjectID) error { return nil }

type exportedExpense struct {
	ID          string    `json:"id"`
	GroupID     string    `json:"grupo_id"`
	Concept     string    `json:"concepto"`
	Description string    `json:"descripcion,omitempty"`
	Cents       int64     `json:"monto_centavos"`
	IPaid       bool      `json:"lo_pague"`
	ICaptured   bool      `json:"lo_capture"`
	Deleted     bool      `json:"eliminado"`
	CreatedAt   time.Time `json:"creado_en"`
}

// Export devuelve los gastos que la persona pagó o capturó, sin ids de los demás.
func (p *ExpensesPrivacy) Export(ctx context.Context, userID bson.ObjectID) (any, error) {
	rows, err := p.repo.UserExpenses(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]exportedExpense, 0, len(rows))
	for _, e := range rows {
		out = append(out, exportedExpense{
			ID: e.ID.Hex(), GroupID: e.GroupID.Hex(), Concept: e.Concept, Description: e.Description, Cents: e.Cents,
			IPaid: e.PaidBy == userID, ICaptured: e.CreatedBy == userID, Deleted: e.DeletedAt != nil, CreatedAt: e.CreatedAt,
		})
	}
	return out, nil
}

type exportedDebt struct {
	ID        string     `json:"id"`
	GroupID   string     `json:"grupo_id"`
	Role      string     `json:"rol"`
	Cents     int64      `json:"monto_centavos"`
	Status    string     `json:"estado"`
	Reference string     `json:"referencia,omitempty"`
	PaidAt    *time.Time `json:"pagada_en,omitempty"`
	CreatedAt time.Time  `json:"creado_en"`
}

// Export devuelve las deudas donde la persona es deudora o acreedora, con su rol.
func (p *DebtsPrivacy) Export(ctx context.Context, userID bson.ObjectID) (any, error) {
	rows, err := p.repo.UserDebts(ctx, userID, "")
	if err != nil {
		return nil, err
	}
	out := make([]exportedDebt, 0, len(rows))
	for _, d := range rows {
		role := "acreedor"
		if d.Debtor == userID {
			role = "deudor"
		}
		out = append(out, exportedDebt{
			ID: d.ID.Hex(), GroupID: d.GroupID.Hex(), Role: role, Cents: d.Cents, Status: d.Status,
			Reference: d.Reference, PaidAt: d.PaidAt, CreatedAt: d.CreatedAt,
		})
	}
	return out, nil
}

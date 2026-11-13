package vaca

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	platformmongo "github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
)

// Errores del repositorio; el service los traduce.
var (
	ErrNotFound  = errors.New("no encontrado")
	ErrDuplicate = errors.New("id_cliente repetido")
)

// Indexes son los índices de `gastos` y `deudas_vaca`. El único creador es scripts/indexes.
func Indexes() []platformmongo.IndexSet {
	return []platformmongo.IndexSet{
		{Collection: ExpensesCollection, Models: []mongo.IndexModel{
			{Keys: bson.D{{Key: "grupo_id", Value: 1}, {Key: "_id", Value: -1}}, Options: options.Index().SetName("ix_gastos_grupo")},
			{Keys: bson.D{{Key: "grupo_id", Value: 1}, {Key: "pagado_por", Value: 1}, {Key: "_id", Value: -1}}, Options: options.Index().SetName("ix_gastos_pagador")},
			{Keys: bson.D{{Key: "creado_por", Value: 1}, {Key: "id_cliente", Value: 1}}, Options: options.Index().SetName("ux_id_cliente").SetUnique(true)},
		}},
		{Collection: DebtsCollection, Models: []mongo.IndexModel{
			{
				Keys:    bson.D{{Key: "grupo_id", Value: 1}, {Key: "deudor_id", Value: 1}, {Key: "acreedor_id", Value: 1}, {Key: "version_corte", Value: 1}},
				Options: options.Index().SetName("ux_deuda_corte").SetUnique(true),
			},
			{Keys: bson.D{{Key: "grupo_id", Value: 1}, {Key: "estado", Value: 1}}, Options: options.Index().SetName("ix_deudas_grupo")},
			{Keys: bson.D{{Key: "deudor_id", Value: 1}, {Key: "estado", Value: 1}}, Options: options.Index().SetName("ix_mis_deudas")},
			{Keys: bson.D{{Key: "acreedor_id", Value: 1}, {Key: "estado", Value: 1}}, Options: options.Index().SetName("ix_mis_deudas_acreedor")},
		}},
	}
}

// Repository lee y escribe `gastos` y `deudas_vaca`.
type Repository struct {
	expenses *mongo.Collection
	debts    *mongo.Collection
}

// NewRepository recibe la base por constructor.
func NewRepository(db *mongo.Database) *Repository {
	return &Repository{expenses: db.Collection(ExpensesCollection), debts: db.Collection(DebtsCollection)}
}

var live = bson.E{Key: "eliminado_en", Value: nil}

// Insert guarda un gasto; ErrDuplicate si la persona ya usó ese id_cliente (ux_id_cliente).
func (r *Repository) Insert(ctx context.Context, e *Expense) error {
	_, err := r.expenses.InsertOne(ctx, e)
	if mongo.IsDuplicateKeyError(err) {
		return ErrDuplicate
	}
	return err
}

func (r *Repository) findExpense(ctx context.Context, filter bson.D) (Expense, error) {
	var e Expense
	err := r.expenses.FindOne(ctx, filter).Decode(&e)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Expense{}, ErrNotFound
	}
	return e, err
}

// FindByClientID trae el gasto que la persona capturó con ese id_cliente (aunque esté borrado).
func (r *Repository) FindByClientID(ctx context.Context, creator bson.ObjectID, clientID string) (Expense, error) {
	return r.findExpense(ctx, bson.D{{Key: "creado_por", Value: creator}, {Key: "id_cliente", Value: clientID}})
}

// FindExpense trae un gasto vivo.
func (r *Repository) FindExpense(ctx context.Context, id bson.ObjectID) (Expense, error) {
	return r.findExpense(ctx, bson.D{{Key: "_id", Value: id}, live})
}

// CountLive cuenta los gastos vivos del grupo.
func (r *Repository) CountLive(ctx context.Context, groupID bson.ObjectID) (int64, error) {
	return r.expenses.CountDocuments(ctx, bson.D{{Key: "grupo_id", Value: groupID}, live})
}

// LiveExpenses trae todos los gastos vivos del grupo (a lo más MaxExpenses).
func (r *Repository) LiveExpenses(ctx context.Context, groupID bson.ObjectID) ([]Expense, error) {
	return findAll[Expense](ctx, r.expenses, bson.D{{Key: "grupo_id", Value: groupID}, live}, options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}))
}

// ExpenseFilter son los filtros de la lista.
type ExpenseFilter struct {
	Concept string
	PaidBy  *bson.ObjectID
	After   *bson.ObjectID
}

func (f ExpenseFilter) query(groupID bson.ObjectID) bson.D {
	q := bson.D{{Key: "grupo_id", Value: groupID}}
	if f.PaidBy != nil {
		q = append(q, bson.E{Key: "pagado_por", Value: *f.PaidBy})
	}
	q = append(q, live)
	if f.Concept != "" {
		q = append(q, bson.E{Key: "concepto", Value: f.Concept})
	}
	return q
}

// ListExpenses trae hasta limit+1 gastos vivos del grupo, del más nuevo al más viejo.
func (r *Repository) ListExpenses(ctx context.Context, groupID bson.ObjectID, f ExpenseFilter, limit int) ([]Expense, error) {
	q := f.query(groupID)
	if f.After != nil {
		q = append(q, bson.E{Key: "_id", Value: bson.D{{Key: "$lt", Value: *f.After}}})
	}
	return findAll[Expense](ctx, r.expenses, q, options.Find().SetSort(bson.D{{Key: "_id", Value: -1}}).SetLimit(int64(limit+1)))
}

// TotalCents suma los gastos vivos del grupo con los mismos filtros (sin cursor).
func (r *Repository) TotalCents(ctx context.Context, groupID bson.ObjectID, f ExpenseFilter) (int64, error) {
	cur, err := r.expenses.Aggregate(ctx, mongo.Pipeline{
		{{Key: "$match", Value: f.query(groupID)}},
		{{Key: "$group", Value: bson.D{{Key: "_id", Value: nil}, {Key: "total", Value: bson.D{{Key: "$sum", Value: "$monto_centavos"}}}}}},
	})
	if err != nil {
		return 0, err
	}
	var rows []struct {
		Total int64 `bson:"total"`
	}
	if err := cur.All(ctx, &rows); err != nil || len(rows) == 0 {
		return 0, err
	}
	return rows[0].Total, nil
}

// Replace guarda los cambios de un gasto si sigue en `version`; false si otra petición ganó.
func (r *Repository) Replace(ctx context.Context, e *Expense, version int) (bool, error) {
	e.Version = version + 1
	res, err := r.expenses.ReplaceOne(ctx, bson.D{{Key: "_id", Value: e.ID}, {Key: "version", Value: version}, live}, e)
	if err != nil {
		return false, err
	}
	return res.MatchedCount == 1, nil
}

// SoftDelete marca el gasto como borrado si sigue en `version`.
func (r *Repository) SoftDelete(ctx context.Context, id bson.ObjectID, version int, now time.Time) (bool, error) {
	res, err := r.expenses.UpdateOne(ctx, bson.D{{Key: "_id", Value: id}, {Key: "version", Value: version}, live}, bson.D{
		{Key: "$set", Value: bson.D{{Key: "eliminado_en", Value: now}, {Key: "actualizado_en", Value: now}}},
		{Key: "$inc", Value: bson.D{{Key: "version", Value: 1}}},
	})
	if err != nil {
		return false, err
	}
	return res.MatchedCount == 1, nil
}

// GroupDebts trae las deudas del grupo en `status` (vacío = todas).
func (r *Repository) GroupDebts(ctx context.Context, groupID bson.ObjectID, status string) ([]Debt, error) {
	q := bson.D{{Key: "grupo_id", Value: groupID}}
	if status != "" {
		q = append(q, bson.E{Key: "estado", Value: status})
	}
	return findAll[Debt](ctx, r.debts, q, options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}))
}

// LastCutVersion es la versión del corte más reciente del grupo (0 si no hay).
func (r *Repository) LastCutVersion(ctx context.Context, groupID bson.ObjectID) (int, error) {
	var d Debt
	err := r.debts.FindOne(ctx, bson.D{{Key: "grupo_id", Value: groupID}}, options.FindOne().SetSort(bson.D{{Key: "version_corte", Value: -1}})).Decode(&d)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return 0, nil
	}
	return d.CutVersion, err
}

// ReplacePending borra las deudas pendientes del grupo y guarda las del corte nuevo. Corre
// bajo `lock:vaca:{grupo_id}`; si falla a la mitad, el siguiente recálculo lo arregla.
func (r *Repository) ReplacePending(ctx context.Context, groupID bson.ObjectID, fresh []Debt) error {
	if _, err := r.debts.DeleteMany(ctx, bson.D{{Key: "grupo_id", Value: groupID}, {Key: "estado", Value: DebtPending}}); err != nil {
		return err
	}
	if len(fresh) == 0 {
		return nil
	}
	docs := make([]any, 0, len(fresh))
	for i := range fresh {
		docs = append(docs, fresh[i])
	}
	_, err := r.debts.InsertMany(ctx, docs)
	return err
}

// FindDebt trae una deuda.
func (r *Repository) FindDebt(ctx context.Context, id bson.ObjectID) (Debt, error) {
	var d Debt
	err := r.debts.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&d)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Debt{}, ErrNotFound
	}
	return d, err
}

// MarkPaid pasa una deuda de pendiente a pagada; false si ya no estaba pendiente.
func (r *Repository) MarkPaid(ctx context.Context, id bson.ObjectID, reference string, now time.Time) (bool, error) {
	set := bson.D{{Key: "estado", Value: DebtPaid}, {Key: "pagada_en", Value: now}, {Key: "actualizado_en", Value: now}}
	if reference != "" {
		set = append(set, bson.E{Key: "referencia", Value: reference})
	}
	res, err := r.debts.UpdateOne(ctx, bson.D{{Key: "_id", Value: id}, {Key: "estado", Value: DebtPending}}, bson.D{{Key: "$set", Value: set}})
	if err != nil {
		return false, err
	}
	return res.MatchedCount == 1, nil
}

// UserDebts trae las deudas donde userID es deudor o acreedor, en `status` (vacío = todas).
func (r *Repository) UserDebts(ctx context.Context, userID bson.ObjectID, status string) ([]Debt, error) {
	side := func(field string) bson.D {
		q := bson.D{{Key: field, Value: userID}}
		if status != "" {
			q = append(q, bson.E{Key: "estado", Value: status})
		}
		return q
	}
	return findAll[Debt](ctx, r.debts, bson.D{{Key: "$or", Value: bson.A{side("deudor_id"), side("acreedor_id")}}}, options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}))
}

// UserExpenses trae los gastos que userID pagó o capturó, para exportarlos.
func (r *Repository) UserExpenses(ctx context.Context, userID bson.ObjectID) ([]Expense, error) {
	q := bson.D{{Key: "$or", Value: bson.A{bson.D{{Key: "pagado_por", Value: userID}}, bson.D{{Key: "creado_por", Value: userID}}}}}
	return findAll[Expense](ctx, r.expenses, q, options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}))
}

func findAll[T any](ctx context.Context, col *mongo.Collection, filter bson.D, opts *options.FindOptionsBuilder) ([]T, error) {
	cur, err := col.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	out := []T{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

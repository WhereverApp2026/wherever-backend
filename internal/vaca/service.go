package vaca

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/grupos"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// Members es lo que la Vaca lee de grupos (nv-37): el grupo con sus integrantes.
type Members interface {
	FindByID(ctx context.Context, id bson.ObjectID) (grupos.Group, error)
}

// Users trae lo público de las cuentas activas; la que falta sale como «Cuenta eliminada».
type Users interface {
	FindPublicByIDs(ctx context.Context, ids []bson.ObjectID) ([]usuarios.User, error)
}

// Service maneja la Vaca (nv-58). "Gasto agregado" no se notifica hasta nv-40 y nv-41.
type Service struct {
	repo    *Repository
	members Members
	users   Users
	locker  Locker
	cursor  *pagination.Signer
	now     func() time.Time
}

// NewService recibe sus dependencias por constructor; sin cursor la lista falla cerrada.
func NewService(repo *Repository, members Members, users Users, locker Locker, cursor *pagination.Signer) *Service {
	return &Service{repo: repo, members: members, users: users, locker: locker, cursor: cursor, now: time.Now}
}

// Errores públicos: se crean en cada llamada porque la etapa de errores puede anotarlos.
func errGroupNotFound() error   { return apperr.NotFound("grupo no encontrado") }
func errExpenseNotFound() error { return apperr.NotFound("gasto no encontrado") }
func errDebtNotFound() error    { return apperr.NotFound("deuda no encontrada") }
func errStale() error {
	return apperr.Conflict("el gasto cambió; vuelve a leerlo y usa su versión en If-Match")
}

// errStore no lleva el texto de MongoDB ni de Redis: podría repetir ids o nombres.
func errStore() error { return apperr.Unavailable("servicio no disponible") }

func self(subject string) (bson.ObjectID, error) {
	id, err := bson.ObjectIDFromHex(subject)
	if err != nil {
		return bson.ObjectID{}, apperr.Unauthorized("autenticación requerida")
	}
	return id, nil
}

// group trae el grupo si me es integrante confirmado (el organizador lo es); si no, 404.
func (s *Service) group(ctx context.Context, me bson.ObjectID, raw string) (grupos.Group, error) {
	id, err := bson.ObjectIDFromHex(raw)
	if err != nil {
		return grupos.Group{}, errGroupNotFound()
	}
	return s.groupByID(ctx, me, id)
}

func (s *Service) groupByID(ctx context.Context, me, id bson.ObjectID) (grupos.Group, error) {
	g, err := s.members.FindByID(ctx, id)
	if errors.Is(err, grupos.ErrNotFound) {
		return grupos.Group{}, errGroupNotFound()
	}
	if err != nil {
		return grupos.Group{}, errStore()
	}
	if !confirmed(g)[me] {
		return grupos.Group{}, errGroupNotFound()
	}
	return g, nil
}

// confirmed son los integrantes que cuentan para la Vaca: confirmados (incluye al organizador).
func confirmed(g grupos.Group) map[bson.ObjectID]bool {
	out := map[bson.ObjectID]bool{g.OrganizerID: true}
	for _, m := range g.Members {
		if m.Status == grupos.StatusConfirmed {
			out[m.UserID] = true
		}
	}
	return out
}

func sortedIDs(set map[bson.ObjectID]bool) []bson.ObjectID {
	out := make([]bson.ObjectID, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Hex() < out[j].Hex() })
	return out
}

// locked corre fn con el candado del grupo. Redis caído o candado ocupado = 503.
func (s *Service) locked(ctx context.Context, groupID bson.ObjectID, fn func() error) error {
	release, err := s.locker.Lock(ctx, groupID.Hex())
	if err != nil {
		return errStore()
	}
	defer release()
	return fn()
}

// recalc vuelve a calcular el corte: borra las deudas pendientes y escribe las nuevas con
// `version_corte + 1`; las pagadas se conservan. Corre dentro del candado.
func (s *Service) recalc(ctx context.Context, g grupos.Group) error {
	expenses, err := s.repo.LiveExpenses(ctx, g.ID)
	if err != nil {
		return errStore()
	}
	paid, err := s.repo.GroupDebts(ctx, g.ID, DebtPaid)
	if err != nil {
		return errStore()
	}
	version, err := s.repo.LastCutVersion(ctx, g.ID)
	if err != nil {
		return errStore()
	}
	_, transfers := Compute(nil, expenses, paid)
	now := s.now().UTC()
	fresh := make([]Debt, 0, len(transfers))
	for _, t := range transfers {
		fresh = append(fresh, Debt{
			ID: bson.NewObjectID(), GroupID: g.ID, Debtor: t.From, Creditor: t.To, Cents: t.Cents,
			Status: DebtPending, CutVersion: version + 1, CreatedAt: now, UpdatedAt: now,
		})
	}
	if err := s.repo.ReplacePending(ctx, g.ID, fresh); err != nil {
		return errStore()
	}
	return nil
}

// Create registra un gasto (201). Un id_cliente que la persona ya usó responde el gasto
// existente (200, created = false), para que reenviar desde el teléfono sea seguro.
func (s *Service) Create(ctx context.Context, subject, rawGroup string, req CreateRequest) (ExpenseResult, bool, error) {
	me, err := self(subject)
	if err != nil {
		return ExpenseResult{}, false, err
	}
	g, err := s.group(ctx, me, rawGroup)
	if err != nil {
		return ExpenseResult{}, false, err
	}
	if prev, err := s.repo.FindByClientID(ctx, me, req.ClientID); err == nil {
		return s.replay(ctx, me, prev)
	} else if !errors.Is(err, ErrNotFound) {
		return ExpenseResult{}, false, errStore()
	}
	members := confirmed(g)
	now := s.now().UTC()
	e := Expense{
		ID: bson.NewObjectID(), GroupID: g.ID, PlanID: optionalID(req.PlanID), Concept: req.Concept,
		Description: req.Description, Cents: req.Cents, PaidBy: oid(req.PaidBy), SplitAmong: ids(req.SplitAmong),
		Division: req.Division, Custom: shares(req.Custom), BusinessID: optionalID(req.BusinessID),
		ClientID: req.ClientID, CreatedBy: me, Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	if len(e.SplitAmong) == 0 {
		e.SplitAmong = defaultSplit(e, members)
	}
	if len(e.Custom) == 0 {
		e.Custom = nil
	}
	if err := CheckSplit(e, members); err != nil {
		return ExpenseResult{}, false, err
	}
	duplicate := false
	err = s.locked(ctx, g.ID, func() error {
		n, err := s.repo.CountLive(ctx, g.ID)
		if err != nil {
			return errStore()
		}
		if n >= MaxExpenses {
			return invalid("grupo", fmt.Sprintf("el grupo ya tiene %d gastos", MaxExpenses))
		}
		if err := s.repo.Insert(ctx, &e); errors.Is(err, ErrDuplicate) {
			duplicate = true // otra petición con el mismo id_cliente ganó
			return nil
		} else if err != nil {
			return errStore()
		}
		return s.recalc(ctx, g)
	})
	if err != nil {
		return ExpenseResult{}, false, err
	}
	if duplicate {
		prev, err := s.repo.FindByClientID(ctx, me, req.ClientID)
		if err != nil {
			return ExpenseResult{}, false, errStore()
		}
		return s.replay(ctx, me, prev)
	}
	return s.result(ctx, g, e, true)
}

// defaultSplit son todos los integrantes confirmados, o las personas de la división
// personalizada si la trae.
func defaultSplit(e Expense, members map[bson.ObjectID]bool) []bson.ObjectID {
	if e.Division == DivisionCustom && len(e.Custom) > 0 {
		out := make([]bson.ObjectID, 0, len(e.Custom))
		for _, c := range e.Custom {
			out = append(out, c.UserID)
		}
		return out
	}
	return sortedIDs(members)
}

// replay responde un gasto ya registrado con el mismo id_cliente.
func (s *Service) replay(ctx context.Context, me bson.ObjectID, e Expense) (ExpenseResult, bool, error) {
	g, err := s.groupByID(ctx, me, e.GroupID)
	if err != nil {
		return ExpenseResult{}, false, err
	}
	return s.result(ctx, g, e, false)
}

func (s *Service) result(ctx context.Context, g grupos.Group, e Expense, created bool) (ExpenseResult, bool, error) {
	cut, err := s.cut(ctx, g)
	if err != nil {
		return ExpenseResult{}, false, err
	}
	names, err := s.names(ctx, []bson.ObjectID{e.PaidBy})
	if err != nil {
		return ExpenseResult{}, false, err
	}
	return ExpenseResult{Expense: expenseView(e, names), Cut: cut}, created, nil
}

// List devuelve una página de gastos vivos, del más nuevo al más viejo, con el total del
// grupo bajo los mismos filtros.
func (s *Service) List(ctx context.Context, subject, rawGroup string, q ListQuery) (ExpensePage, error) {
	me, err := self(subject)
	if err != nil {
		return ExpensePage{}, err
	}
	g, err := s.group(ctx, me, rawGroup)
	if err != nil {
		return ExpensePage{}, err
	}
	f := ExpenseFilter{Concept: q.Concept, PaidBy: optionalID(q.PaidBy)}
	scope := s.cursor.Scope("gastos", g.ID.Hex(), q.Concept, q.PaidBy)
	c, ok, err := validation.Position(s.cursor, scope, q.Cursor)
	if err != nil {
		return ExpensePage{}, err
	}
	if ok {
		after, err := bson.ObjectIDFromHex(c.ID)
		if err != nil {
			return ExpensePage{}, invalid("cursor", "cursor inválido")
		}
		f.After = &after
	}
	rows, err := s.repo.ListExpenses(ctx, g.ID, f, q.Limit)
	if err != nil {
		return ExpensePage{}, errStore()
	}
	next := ""
	if len(rows) > q.Limit {
		rows = rows[:q.Limit]
		last := rows[len(rows)-1].ID.Hex()
		if next, err = s.cursor.Encode(pagination.Cursor{ID: last, Sort: last, Scope: scope}); err != nil {
			return ExpensePage{}, errStore()
		}
	}
	total, err := s.repo.TotalCents(ctx, g.ID, f)
	if err != nil {
		return ExpensePage{}, errStore()
	}
	payers := make([]bson.ObjectID, 0, len(rows))
	for _, e := range rows {
		payers = append(payers, e.PaidBy)
	}
	names, err := s.names(ctx, payers)
	if err != nil {
		return ExpensePage{}, err
	}
	items := make([]ExpenseView, 0, len(rows))
	for _, e := range rows {
		items = append(items, expenseView(e, names))
	}
	return ExpensePage{Items: items, Next: next, Total: total}, nil
}

// editable trae el gasto y su grupo si puedo editarlo: quien lo capturó o el organizador
// (otro integrante = 403; ajeno = 404).
func (s *Service) editable(ctx context.Context, me bson.ObjectID, raw string) (Expense, grupos.Group, error) {
	id, err := bson.ObjectIDFromHex(raw)
	if err != nil {
		return Expense{}, grupos.Group{}, errExpenseNotFound()
	}
	e, err := s.repo.FindExpense(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return Expense{}, grupos.Group{}, errExpenseNotFound()
	}
	if err != nil {
		return Expense{}, grupos.Group{}, errStore()
	}
	g, err := s.groupByID(ctx, me, e.GroupID)
	if err != nil {
		return Expense{}, grupos.Group{}, errExpenseNotFound()
	}
	if e.CreatedBy != me && g.OrganizerID != me {
		return Expense{}, grupos.Group{}, apperr.Forbidden("solo quien capturó el gasto o el organizador pueden cambiarlo")
	}
	return e, g, nil
}

// paidGuard responde 409 si el grupo ya tiene deudas pagadas y no viene `confirmar`.
func (s *Service) paidGuard(ctx context.Context, groupID bson.ObjectID, confirm bool) error {
	if confirm {
		return nil
	}
	paid, err := s.repo.GroupDebts(ctx, groupID, DebtPaid)
	if err != nil {
		return errStore()
	}
	if len(paid) > 0 {
		return apperr.Conflict(fmt.Sprintf("deudas_pagadas: el grupo tiene %d deudas pagadas; repite con confirmar=true", len(paid)))
	}
	return nil
}

// Patch corrige un gasto con If-Match y recalcula el corte.
func (s *Service) Patch(ctx context.Context, subject, raw string, version int, req PatchRequest) (ExpenseResult, error) {
	me, err := self(subject)
	if err != nil {
		return ExpenseResult{}, err
	}
	e, g, err := s.editable(ctx, me, raw)
	if err != nil {
		return ExpenseResult{}, err
	}
	if e.Version != version {
		return ExpenseResult{}, errStale()
	}
	apply(&e, req)
	if err := CheckSplit(e, confirmed(g)); err != nil {
		return ExpenseResult{}, err
	}
	e.UpdatedAt = s.now().UTC()
	err = s.locked(ctx, g.ID, func() error {
		if err := s.paidGuard(ctx, g.ID, req.Confirm); err != nil {
			return err
		}
		ok, err := s.repo.Replace(ctx, &e, version)
		if err != nil {
			return errStore()
		}
		if !ok {
			return errStale()
		}
		return s.recalc(ctx, g)
	})
	if err != nil {
		return ExpenseResult{}, err
	}
	out, _, err := s.result(ctx, g, e, false)
	return out, err
}

// apply copia al gasto lo que trae el PATCH. Cambiar a división igual borra los montos
// personalizados.
func apply(e *Expense, req PatchRequest) {
	if req.Cents != nil {
		e.Cents = *req.Cents
	}
	if req.Concept != nil {
		e.Concept = *req.Concept
	}
	if req.Description != nil {
		e.Description = *req.Description
	}
	if req.PaidBy != nil {
		e.PaidBy = oid(*req.PaidBy)
	}
	if req.SplitAmong != nil {
		e.SplitAmong = ids(*req.SplitAmong)
	}
	if req.Division != nil {
		e.Division = *req.Division
	}
	if req.Custom != nil {
		e.Custom = shares(*req.Custom)
	}
	if e.Division == DivisionEqual || len(e.Custom) == 0 {
		e.Custom = nil
	}
}

// Delete borra el gasto (lógico) con If-Match y recalcula el corte.
func (s *Service) Delete(ctx context.Context, subject, raw string, version int, confirm bool) error {
	me, err := self(subject)
	if err != nil {
		return err
	}
	e, g, err := s.editable(ctx, me, raw)
	if err != nil {
		return err
	}
	if e.Version != version {
		return errStale()
	}
	return s.locked(ctx, g.ID, func() error {
		if err := s.paidGuard(ctx, g.ID, confirm); err != nil {
			return err
		}
		ok, err := s.repo.SoftDelete(ctx, e.ID, version, s.now().UTC())
		if err != nil {
			return errStore()
		}
		if !ok {
			return errStale()
		}
		return s.recalc(ctx, g)
	})
}

// Cut devuelve el corte del grupo.
func (s *Service) Cut(ctx context.Context, subject, rawGroup string) (CutView, error) {
	me, err := self(subject)
	if err != nil {
		return CutView{}, err
	}
	g, err := s.group(ctx, me, rawGroup)
	if err != nil {
		return CutView{}, err
	}
	return s.cut(ctx, g)
}

func (s *Service) cut(ctx context.Context, g grupos.Group) (CutView, error) {
	expenses, err := s.repo.LiveExpenses(ctx, g.ID)
	if err != nil {
		return CutView{}, errStore()
	}
	debts, err := s.repo.GroupDebts(ctx, g.ID, "")
	if err != nil {
		return CutView{}, errStore()
	}
	var paid []Debt
	version := 0
	for _, d := range debts {
		if d.Status == DebtPaid {
			paid = append(paid, d)
		}
		version = max(version, d.CutVersion)
	}
	balances, _ := Compute(sortedIDs(confirmed(g)), expenses, paid)
	who := make([]bson.ObjectID, 0, len(balances))
	for _, b := range balances {
		who = append(who, b.UserID)
	}
	names, err := s.names(ctx, who)
	if err != nil {
		return CutView{}, err
	}
	out := CutView{People: make([]BalanceView, 0, len(balances)), Debts: make([]DebtView, 0, len(debts)), Version: version}
	for _, e := range expenses {
		out.Total += e.Cents
	}
	for _, b := range balances {
		out.People = append(out.People, BalanceView{UserID: b.UserID.Hex(), Name: names.of(b.UserID), Put: b.Put, Share: b.Share, Balance: b.Balance})
	}
	for _, d := range debts {
		out.Debts = append(out.Debts, debtView(d, names))
	}
	return out, nil
}

// MarkPaid marca una deuda como pagada: solo el deudor o el acreedor (otro integrante = 403;
// ajeno = 404; ya pagada = 409). Recalcula el corte.
func (s *Service) MarkPaid(ctx context.Context, subject, raw string, req MarkPaidRequest) (DebtView, error) {
	me, err := self(subject)
	if err != nil {
		return DebtView{}, err
	}
	id, err := bson.ObjectIDFromHex(raw)
	if err != nil {
		return DebtView{}, errDebtNotFound()
	}
	d, err := s.repo.FindDebt(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return DebtView{}, errDebtNotFound()
	}
	if err != nil {
		return DebtView{}, errStore()
	}
	g, err := s.groupByID(ctx, me, d.GroupID)
	if err != nil {
		return DebtView{}, errDebtNotFound()
	}
	if d.Debtor != me && d.Creditor != me {
		return DebtView{}, apperr.Forbidden("solo el deudor o el acreedor marcan la deuda")
	}
	if d.Status == DebtPaid {
		return DebtView{}, apperr.Conflict("la deuda ya está pagada")
	}
	err = s.locked(ctx, g.ID, func() error {
		ok, err := s.repo.MarkPaid(ctx, d.ID, req.Reference, s.now().UTC())
		if err != nil {
			return errStore()
		}
		if !ok { // ya pagada o el corte cambió mientras tanto
			return apperr.Conflict("la deuda ya no está pendiente")
		}
		return s.recalc(ctx, g)
	})
	if err != nil {
		return DebtView{}, err
	}
	d, err = s.repo.FindDebt(ctx, d.ID)
	if err != nil {
		return DebtView{}, errStore()
	}
	names, err := s.names(ctx, []bson.ObjectID{d.Debtor, d.Creditor})
	if err != nil {
		return DebtView{}, err
	}
	return debtView(d, names), nil
}

// MyDebts reúne lo que debo y lo que me deben en todos mis grupos.
func (s *Service) MyDebts(ctx context.Context, subject string, q MyDebtsQuery) (MyDebtsView, error) {
	me, err := self(subject)
	if err != nil {
		return MyDebtsView{}, err
	}
	status := q.Status
	if status == "" {
		status = DebtPending
	}
	debts, err := s.repo.UserDebts(ctx, me, status)
	if err != nil {
		return MyDebtsView{}, errStore()
	}
	who := make([]bson.ObjectID, 0, 2*len(debts))
	for _, d := range debts {
		who = append(who, d.Debtor, d.Creditor)
	}
	names, err := s.names(ctx, who)
	if err != nil {
		return MyDebtsView{}, err
	}
	out := MyDebtsView{Owe: []DebtView{}, OwedToMe: []DebtView{}}
	for _, d := range debts {
		if d.Debtor == me {
			out.Owe = append(out.Owe, debtView(d, names))
			out.TotalOwe += d.Cents
		} else {
			out.OwedToMe = append(out.OwedToMe, debtView(d, names))
			out.TotalOwed += d.Cents
		}
	}
	return out, nil
}

// nameBook resuelve nombres; una cuenta que ya no está activa sale como «Cuenta eliminada».
type nameBook map[bson.ObjectID]string

func (n nameBook) of(id bson.ObjectID) string {
	if name, ok := n[id]; ok {
		return name
	}
	return DeletedUserName
}

func (n nameBook) person(id bson.ObjectID) Person { return Person{UserID: id.Hex(), Name: n.of(id)} }

func (s *Service) names(ctx context.Context, who []bson.ObjectID) (nameBook, error) {
	out := nameBook{}
	if len(who) == 0 {
		return out, nil
	}
	users, err := s.users.FindPublicByIDs(ctx, who)
	if err != nil {
		return nil, errStore()
	}
	for _, u := range users {
		out[u.ID] = u.Name
	}
	return out, nil
}

func hexOf(id *bson.ObjectID) string {
	if id == nil {
		return ""
	}
	return id.Hex()
}

func expenseView(e Expense, names nameBook) ExpenseView {
	split := make([]string, 0, len(e.SplitAmong))
	for _, id := range e.SplitAmong {
		split = append(split, id.Hex())
	}
	var custom []ShareView
	for _, c := range e.Custom {
		custom = append(custom, ShareView{UserID: c.UserID.Hex(), Cents: c.Cents})
	}
	return ExpenseView{
		ID: e.ID.Hex(), GroupID: e.GroupID.Hex(), PlanID: hexOf(e.PlanID), Concept: e.Concept, Description: e.Description,
		Cents: e.Cents, PaidBy: names.person(e.PaidBy), SplitAmong: split, Division: e.Division, Custom: custom,
		BusinessID: hexOf(e.BusinessID), ClientID: e.ClientID, CreatedBy: e.CreatedBy.Hex(), Version: e.Version,
		CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
	}
}

func debtView(d Debt, names nameBook) DebtView {
	return DebtView{
		ID: d.ID.Hex(), GroupID: d.GroupID.Hex(), Debtor: names.person(d.Debtor), Creditor: names.person(d.Creditor),
		Cents: d.Cents, Status: d.Status, Reference: d.Reference, PaidAt: d.PaidAt, CutVersion: d.CutVersion,
	}
}

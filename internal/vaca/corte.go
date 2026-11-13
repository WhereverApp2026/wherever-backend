package vaca

import (
	"sort"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Balance es la cuenta de una persona en el corte. Saldo positivo = le deben; negativo = debe.
type Balance struct {
	UserID  bson.ObjectID
	Put     int64
	Share   int64
	Balance int64
}

// Transfer es un pago pendiente del corte.
type Transfer struct {
	From  bson.ObjectID
	To    bson.ObjectID
	Cents int64
}

// Shares reparte un gasto entre sus personas. En división igual, el residuo de la división
// entera va a quien pagó si participa; si no, al primero de `dividir_entre` por id.
func Shares(e Expense) map[bson.ObjectID]int64 {
	out := make(map[bson.ObjectID]int64, len(e.SplitAmong))
	if e.Division == DivisionCustom {
		for _, s := range e.Custom {
			out[s.UserID] += s.Cents
		}
		return out
	}
	n := int64(len(e.SplitAmong))
	if n == 0 {
		return out
	}
	base, rest := e.Cents/n, e.Cents%n
	residual := e.SplitAmong[0]
	payerIn := false
	for _, id := range e.SplitAmong {
		out[id] = base
		if id == e.PaidBy {
			payerIn = true
		}
		if id.Hex() < residual.Hex() {
			residual = id
		}
	}
	if payerIn {
		residual = e.PaidBy
	}
	out[residual] += rest
	return out
}

// Compute calcula el corte: saldo de cada persona (lo que pagó − lo que le toca + lo que ya
// transfirió − lo que ya recibió) y las transferencias mínimas. `people` agrega personas sin
// movimientos (salen en cero). Es determinista: empates por id. La suma de saldos es 0 y hay
// a lo más n − 1 transferencias.
func Compute(people []bson.ObjectID, expenses []Expense, paid []Debt) ([]Balance, []Transfer) {
	acc := map[bson.ObjectID]*Balance{}
	get := func(id bson.ObjectID) *Balance {
		b, ok := acc[id]
		if !ok {
			b = &Balance{UserID: id}
			acc[id] = b
		}
		return b
	}
	for _, id := range people {
		get(id)
	}
	for _, e := range expenses {
		get(e.PaidBy).Put += e.Cents
		for id, c := range Shares(e) {
			get(id).Share += c
		}
	}
	for _, d := range paid {
		get(d.Debtor).Balance += d.Cents
		get(d.Creditor).Balance -= d.Cents
	}
	out := make([]Balance, 0, len(acc))
	for _, b := range acc {
		b.Balance += b.Put - b.Share
		out = append(out, *b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UserID.Hex() < out[j].UserID.Hex() })
	return out, settle(out)
}

// settle empareja al que más debe con al que más le deben hasta dejar a uno en cero.
func settle(balances []Balance) []Transfer {
	type entry struct {
		id  bson.ObjectID
		amt int64
	}
	var debtors, creditors []entry
	for _, b := range balances {
		switch {
		case b.Balance < 0:
			debtors = append(debtors, entry{b.UserID, -b.Balance})
		case b.Balance > 0:
			creditors = append(creditors, entry{b.UserID, b.Balance})
		}
	}
	largest := func(es []entry) int {
		best := -1
		for i, e := range es {
			if e.amt == 0 {
				continue
			}
			if best < 0 || e.amt > es[best].amt || (e.amt == es[best].amt && e.id.Hex() < es[best].id.Hex()) {
				best = i
			}
		}
		return best
	}
	var out []Transfer
	for {
		d, c := largest(debtors), largest(creditors)
		if d < 0 || c < 0 {
			return out
		}
		amt := min(debtors[d].amt, creditors[c].amt)
		out = append(out, Transfer{From: debtors[d].id, To: creditors[c].id, Cents: amt})
		debtors[d].amt -= amt
		creditors[c].amt -= amt
	}
}

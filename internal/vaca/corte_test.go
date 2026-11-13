package vaca

import (
	"math/rand/v2"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// people devuelve n ids ordenados, para que los casos de empate sean predecibles.
func people(n int) []bson.ObjectID {
	out := make([]bson.ObjectID, 0, n)
	for range n {
		out = append(out, bson.NewObjectID())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Hex() < out[j].Hex() })
	return out
}

func equal(paidBy bson.ObjectID, cents int64, among ...bson.ObjectID) Expense {
	return Expense{PaidBy: paidBy, Cents: cents, SplitAmong: among, Division: DivisionEqual}
}

func balanceOf(t *testing.T, bs []Balance, id bson.ObjectID) Balance {
	t.Helper()
	for _, b := range bs {
		if b.UserID == id {
			return b
		}
	}
	t.Fatalf("sin saldo para %s", id.Hex())
	return Balance{}
}

func TestShares_ResiduoAQuienPagaOAlPrimeroPorID(t *testing.T) {
	p := people(4)
	got := Shares(equal(p[2], 1000, p[0], p[1], p[2]))
	assert.Equal(t, map[bson.ObjectID]int64{p[0]: 333, p[1]: 333, p[2]: 334}, got, "el centavo va a quien pagó")

	got = Shares(equal(p[3], 1000, p[2], p[1], p[0]))
	assert.Equal(t, map[bson.ObjectID]int64{p[0]: 334, p[1]: 333, p[2]: 333}, got, "quien pagó no participa: al primero por id")

	custom := Expense{PaidBy: p[0], Cents: 500, Division: DivisionCustom, SplitAmong: []bson.ObjectID{p[0], p[1]},
		Custom: []Share{{UserID: p[0], Cents: 100}, {UserID: p[1], Cents: 400}}}
	assert.Equal(t, map[bson.ObjectID]int64{p[0]: 100, p[1]: 400}, Shares(custom))
	assert.Empty(t, Shares(Expense{Division: DivisionEqual}))
}

func TestCompute_Casos(t *testing.T) {
	p := people(3)
	a, b, c := p[0], p[1], p[2]

	t.Run("igual con residuo", func(t *testing.T) {
		bs, tr := Compute(nil, []Expense{equal(a, 1000, a, b, c)}, nil)
		assert.Equal(t, int64(666), balanceOf(t, bs, a).Balance)
		assert.Equal(t, []Transfer{{From: b, To: a, Cents: 333}, {From: c, To: a, Cents: 333}}, tr)
	})

	t.Run("quien pagó fuera de dividir_entre", func(t *testing.T) {
		bs, tr := Compute(nil, []Expense{equal(c, 301, a, b)}, nil)
		assert.Equal(t, int64(151), balanceOf(t, bs, a).Share)
		assert.Equal(t, int64(301), balanceOf(t, bs, c).Balance)
		assert.Equal(t, []Transfer{{From: a, To: c, Cents: 151}, {From: b, To: c, Cents: 150}}, tr)
	})

	t.Run("personalizada", func(t *testing.T) {
		e := Expense{PaidBy: a, Cents: 900, Division: DivisionCustom, SplitAmong: []bson.ObjectID{a, b, c},
			Custom: []Share{{UserID: a, Cents: 100}, {UserID: b, Cents: 200}, {UserID: c, Cents: 600}}}
		_, tr := Compute(nil, []Expense{e}, nil)
		assert.Equal(t, []Transfer{{From: c, To: a, Cents: 600}, {From: b, To: a, Cents: 200}}, tr)
	})

	t.Run("deudas pagadas que invierten el sentido", func(t *testing.T) {
		// b le pagó 500 a a, pero luego el gasto bajó a 600: ahora a le regresa 200 a b.
		paid := []Debt{{Debtor: b, Creditor: a, Cents: 500}}
		bs, tr := Compute(nil, []Expense{equal(a, 600, a, b)}, paid)
		assert.Equal(t, int64(200), balanceOf(t, bs, b).Balance)
		assert.Equal(t, []Transfer{{From: a, To: b, Cents: 200}}, tr)
	})

	t.Run("personas sin movimientos salen en cero", func(t *testing.T) {
		bs, tr := Compute(p, nil, nil)
		require.Len(t, bs, 3)
		assert.Empty(t, tr)
		for _, x := range bs {
			assert.Zero(t, x.Balance)
		}
	})

	t.Run("doce personas", func(t *testing.T) {
		twelve := people(12)
		bs, tr := Compute(nil, []Expense{equal(twelve[5], 1_000_001, twelve...)}, nil)
		assert.Len(t, tr, 11)
		assert.Equal(t, int64(1_000_001-83_338), balanceOf(t, bs, twelve[5]).Balance, "el residuo es de quien pagó")
	})
}

func TestCompute_Determinista(t *testing.T) {
	p := people(4)
	exp := []Expense{equal(p[0], 400, p...), equal(p[1], 400, p...)}
	_, first := Compute(nil, exp, nil)
	for range 20 {
		_, again := Compute(nil, []Expense{exp[1], exp[0]}, nil)
		assert.Equal(t, first, again)
	}
}

// TestCompute_Propiedad: con gastos al azar la suma de saldos es 0, hay a lo más n − 1
// transferencias y aplicarlas deja a todos en cero.
func TestCompute_Propiedad(t *testing.T) {
	rng := rand.New(rand.NewPCG(58, 4)) //nolint:gosec // semilla fija: la propiedad debe ser reproducible, no es criptografía
	for round := range 300 {
		p := people(2 + rng.IntN(11))
		var exp []Expense
		for range 1 + rng.IntN(8) {
			among := append([]bson.ObjectID(nil), p...)
			rng.Shuffle(len(among), func(i, j int) { among[i], among[j] = among[j], among[i] })
			among = among[:2+rng.IntN(len(among)-1)]
			exp = append(exp, equal(p[rng.IntN(len(p))], 1+rng.Int64N(MaxCents), among...))
		}
		var paid []Debt
		if rng.IntN(2) == 0 {
			paid = append(paid, Debt{Debtor: p[0], Creditor: p[1], Cents: 1 + rng.Int64N(5000)})
		}
		bs, tr := Compute(p, exp, paid)
		var sum int64
		left := map[bson.ObjectID]int64{}
		for _, b := range bs {
			sum += b.Balance
			left[b.UserID] = b.Balance
		}
		require.Zero(t, sum, "ronda %d", round)
		require.LessOrEqual(t, len(tr), len(p)-1, "ronda %d", round)
		for _, x := range tr {
			require.Positive(t, x.Cents)
			left[x.From] += x.Cents
			left[x.To] -= x.Cents
		}
		for id, v := range left {
			require.Zero(t, v, "ronda %d: %s queda con %d", round, id.Hex(), v)
		}
	}
}

package vaca

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

const clientID = "2f1c8a4e-1b7d-4c2a-9e3f-5a6b7c8d9e0f"

func hexes(ids ...bson.ObjectID) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, id.Hex())
	}
	return out
}

func TestDTO_Validacion(t *testing.T) {
	v, err := validation.New()
	require.NoError(t, err)
	p := people(13)
	ok := CreateRequest{Cents: 100, Concept: ConceptFood, PaidBy: p[0].Hex(), Division: DivisionEqual, ClientID: clientID}
	with := func(f func(*CreateRequest)) CreateRequest { r := ok; f(&r); return r }
	cents := int64(0)
	bad := "propina"
	cases := []struct {
		name string
		dto  any
		ok   bool
	}{
		{"alta mínima", ok, true},
		{"monto 0", with(func(r *CreateRequest) { r.Cents = 0 }), false},
		{"monto sobre el tope", with(func(r *CreateRequest) { r.Cents = MaxCents + 1 }), false},
		{"concepto fuera de lista", with(func(r *CreateRequest) { r.Concept = "propina" }), false},
		{"descripción larga", with(func(r *CreateRequest) { r.Description = strings.Repeat("a", 61) }), false},
		{"una sola persona", with(func(r *CreateRequest) { r.SplitAmong = hexes(p[0]) }), false},
		{"doce personas", with(func(r *CreateRequest) { r.SplitAmong = hexes(p[:12]...) }), true},
		{"trece personas", with(func(r *CreateRequest) { r.SplitAmong = hexes(p...) }), false},
		{"repetidos", with(func(r *CreateRequest) { r.SplitAmong = hexes(p[0], p[0]) }), false},
		{"id_cliente no UUID", with(func(r *CreateRequest) { r.ClientID = "abc" }), false},
		{"división desconocida", with(func(r *CreateRequest) { r.Division = "mitad" }), false},
		{"patch vacío", PatchRequest{}, true},
		{"patch monto 0", PatchRequest{Cents: &cents}, false},
		{"patch concepto fuera de lista", PatchRequest{Concept: &bad}, false},
		{"marcar pagada", MarkPaidRequest{Status: DebtPaid, Reference: "SPEI 123"}, true},
		{"marcar pendiente", MarkPaidRequest{Status: DebtPending}, false},
		{"lista por defecto", NewListQuery(), true},
		{"lista con filtro inválido", ListQuery{Concept: "x", Limit: 20}, false},
		{"mis deudas pagadas", MyDebtsQuery{Status: DebtPaid}, true},
		{"mis deudas estado raro", MyDebtsQuery{Status: "x"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := v.Struct(tc.dto)
			if tc.ok {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestCheckSplit(t *testing.T) {
	p := people(4)
	members := map[bson.ObjectID]bool{p[0]: true, p[1]: true, p[2]: true}
	base := Expense{PaidBy: p[0], Cents: 300, SplitAmong: []bson.ObjectID{p[0], p[1], p[2]}, Division: DivisionEqual}
	custom := func(c ...Share) Expense {
		e := base
		e.Division = DivisionCustom
		e.Custom = c
		return e
	}
	cases := []struct {
		name  string
		e     Expense
		field string
	}{
		{"igual válida", base, ""},
		{"pagó un no integrante", func() Expense { e := base; e.PaidBy = p[3]; return e }(), "pagado_por"},
		{"divide con un no integrante", func() Expense { e := base; e.SplitAmong = []bson.ObjectID{p[0], p[3]}; return e }(), "dividir_entre"},
		{"una persona", func() Expense { e := base; e.SplitAmong = p[:1]; return e }(), "dividir_entre"},
		{"repetidos", func() Expense { e := base; e.SplitAmong = []bson.ObjectID{p[0], p[0]}; return e }(), "dividir_entre"},
		{"montos con división igual", func() Expense { e := base; e.Custom = []Share{{UserID: p[0], Cents: 300}}; return e }(), "montos_personalizados"},
		{"personalizada válida", custom(Share{p[0], 100}, Share{p[1], 100}, Share{p[2], 100}), ""},
		{"suma distinta", custom(Share{p[0], 100}, Share{p[1], 100}, Share{p[2], 99}), "montos_personalizados"},
		{"falta una persona", custom(Share{p[0], 150}, Share{p[1], 150}), "montos_personalizados"},
		{"persona repetida", custom(Share{p[0], 100}, Share{p[0], 100}, Share{p[2], 100}), "montos_personalizados"},
		{"persona ajena", custom(Share{p[0], 100}, Share{p[1], 100}, Share{p[3], 100}), "montos_personalizados"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckSplit(tc.e, members)
			if tc.field == "" {
				assert.NoError(t, err)
				return
			}
			var ae *apperr.Error
			require.ErrorAs(t, err, &ae)
			require.Len(t, ae.Fields, 1)
			assert.Equal(t, tc.field, ae.Fields[0].Field)
		})
	}
}

func TestApply_CambiarAIgualBorraMontos(t *testing.T) {
	p := people(2)
	e := Expense{Cents: 200, Division: DivisionCustom, SplitAmong: p, Custom: []Share{{p[0], 50}, {p[1], 150}}}
	div, cents := DivisionEqual, int64(400)
	apply(&e, PatchRequest{Division: &div, Cents: &cents})
	assert.Nil(t, e.Custom)
	assert.Equal(t, int64(400), e.Cents)
}

func TestIfMatchYVistas(t *testing.T) {
	id := bson.NewObjectID()
	names := nameBook{}
	v := expenseView(Expense{ID: id, PaidBy: id, SplitAmong: []bson.ObjectID{id}}, names)
	assert.Equal(t, DeletedUserName, v.PaidBy.Name, "sin cuenta sale «Cuenta eliminada»")
	assert.Empty(t, hexOf(nil))
}

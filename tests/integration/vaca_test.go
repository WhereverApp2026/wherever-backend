package integration

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/grupos"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
	"github.com/WhereverApp2026/wherever-backend/internal/vaca"
)

// vacaSetup es un grupo con organizador, dos confirmados y un invitado.
type vacaSetup struct {
	env      *authtest.Env
	app      *fiber.App
	group    grupos.Group
	org      amigo
	a, b     amigo
	invited  amigo
	outsider amigo
}

func newVaca(t *testing.T, prefix string) vacaSetup {
	t.Helper()
	env := authEnv(t)
	env.ClearRateLimits(t)
	s := vacaSetup{env: env, app: env.App(),
		org: seedAmigo(t, env, prefix+"_org"), a: seedAmigo(t, env, prefix+"_a"), b: seedAmigo(t, env, prefix+"_b"),
		invited: seedAmigo(t, env, prefix+"_inv"), outsider: seedAmigo(t, env, prefix+"_ajeno"),
	}
	s.group = seedVacaGroup(t, env, s.org, []amigo{s.a, s.b}, []amigo{s.invited})
	return s
}

func seedVacaGroup(t *testing.T, env *authtest.Env, org amigo, confirmed, invited []amigo) grupos.Group {
	t.Helper()
	now := time.Now().UTC()
	g := grupos.Group{
		ID: bson.NewObjectID(), PlanID: bson.NewObjectID(), OrganizerID: org.user.ID, CreatedAt: now, UpdatedAt: now,
		Members: []grupos.Member{{UserID: org.user.ID, Role: grupos.RoleOrganizer, Status: grupos.StatusConfirmed, Since: now}},
	}
	for _, m := range confirmed {
		g.Members = append(g.Members, grupos.Member{UserID: m.user.ID, Role: grupos.RoleMember, Status: grupos.StatusConfirmed, Since: now})
	}
	for _, m := range invited {
		g.Members = append(g.Members, grupos.Member{UserID: m.user.ID, Role: grupos.RoleMember, Status: grupos.StatusInvited, Since: now})
	}
	g.Occupied = len(g.Members)
	require.NoError(t, grupos.NewRepository(env.Mongo.DB()).Create(context.Background(), &g))
	return g
}

func (s vacaSetup) gastosPath() string { return "/api/v1/grupos/" + s.group.ID.Hex() + "/gastos" }

func gasto(paidBy amigo, cents int64, concept string) map[string]any {
	return map[string]any{"monto_centavos": cents, "concepto": concept, "pagado_por": paidBy.id(), "division": "igual", "id_cliente": newClientID()}
}

// newClientID es un UUID v4 como el que genera el teléfono.
func newClientID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6], b[8] = b[6]&0x0f|0x40, b[8]&0x3f|0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func (s vacaSetup) add(t *testing.T, who amigo, body map[string]any, want int) vaca.ExpenseResult {
	t.Helper()
	got := planCall(t, s.app, http.MethodPost, s.gastosPath(), body, who.token, "")
	require.Equal(t, want, got.status, got.body)
	var out vaca.ExpenseResult
	if want == http.StatusOK || want == http.StatusCreated {
		require.NoError(t, json.Unmarshal([]byte(got.body), &out), got.body)
		assert.Equal(t, `"`+strconv.Itoa(out.Expense.Version)+`"`, got.etag)
	}
	return out
}

func (s vacaSetup) cut(t *testing.T, who amigo) vaca.CutView {
	t.Helper()
	got := planCall(t, s.app, http.MethodGet, "/api/v1/grupos/"+s.group.ID.Hex()+"/vaca", nil, who.token, "")
	require.Equal(t, http.StatusOK, got.status, got.body)
	var out vaca.CutView
	require.NoError(t, json.Unmarshal([]byte(got.body), &out), got.body)
	return out
}

func sumDebts(debts []vaca.DebtView, status string) int64 {
	var total int64
	for _, d := range debts {
		if d.Status == status {
			total += d.Cents
		}
	}
	return total
}

func TestVaca_AltaConCorteEIdClienteRepetido(t *testing.T) {
	s := newVaca(t, "vaca_alta")
	body := gasto(s.org, 1000, vaca.ConceptFood)
	got := s.add(t, s.org, body, http.StatusCreated)
	assert.Len(t, got.Expense.SplitAmong, 3, "por defecto entre los confirmados, sin el invitado")
	assert.Equal(t, int64(1000), got.Cut.Total)
	assert.Len(t, got.Cut.Debts, 2)
	assert.Equal(t, int64(666), sumDebts(got.Cut.Debts, vaca.DebtPending))

	again := s.add(t, s.org, body, http.StatusOK)
	assert.Equal(t, got.Expense.ID, again.Expense.ID)
	n, err := s.env.Mongo.DB().Collection(vaca.ExpensesCollection).CountDocuments(context.Background(), bson.D{{Key: "grupo_id", Value: s.group.ID}})
	require.NoError(t, err)
	assert.Equal(t, int64(1), n, "reenviar no duplica")

	// Invitado como pagador: 422; ajeno y grupo inexistente: 404 igual.
	s.add(t, s.org, gasto(s.invited, 100, vaca.ConceptOther), http.StatusUnprocessableEntity)
	s.add(t, s.invited, gasto(s.invited, 100, vaca.ConceptOther), http.StatusNotFound)
	s.add(t, s.outsider, gasto(s.outsider, 100, vaca.ConceptOther), http.StatusNotFound)
	missing := planCall(t, s.app, http.MethodPost, "/api/v1/grupos/"+bson.NewObjectID().Hex()+"/gastos", gasto(s.org, 100, "otro"), s.org.token, "")
	assert.Equal(t, http.StatusNotFound, missing.status)

	// Personalizada que no suma el total: 422.
	bad := gasto(s.org, 300, vaca.ConceptFood)
	bad["division"] = "personalizada"
	bad["dividir_entre"] = []string{s.org.id(), s.a.id()}
	bad["montos_personalizados"] = []map[string]any{{"usuario_id": s.org.id(), "monto_centavos": 100}, {"usuario_id": s.a.id(), "monto_centavos": 100}}
	s.add(t, s.org, bad, http.StatusUnprocessableEntity)

	// Todos ven el mismo corte.
	assert.Equal(t, s.cut(t, s.org), s.cut(t, s.b))
}

func TestVaca_ListaConFiltrosYCursor(t *testing.T) {
	s := newVaca(t, "vaca_lista")
	s.add(t, s.org, gasto(s.org, 100, vaca.ConceptFood), http.StatusCreated)
	s.add(t, s.org, gasto(s.org, 200, vaca.ConceptFood), http.StatusCreated)
	s.add(t, s.a, gasto(s.a, 400, vaca.ConceptDrinks), http.StatusCreated)

	page := func(query string) vaca.ExpensePage {
		got := planCall(t, s.app, http.MethodGet, s.gastosPath()+query, nil, s.b.token, "")
		require.Equal(t, http.StatusOK, got.status, got.body)
		var out vaca.ExpensePage
		require.NoError(t, json.Unmarshal([]byte(got.body), &out))
		return out
	}
	all := page("")
	assert.Len(t, all.Items, 3)
	assert.Equal(t, int64(700), all.Total)
	food := page("?concepto=comida")
	assert.Len(t, food.Items, 2)
	assert.Equal(t, int64(300), food.Total)
	byA := page("?pagado_por=" + s.a.id())
	require.Len(t, byA.Items, 1)
	assert.Equal(t, int64(400), byA.Items[0].Cents)

	first := page("?limite=2")
	require.Len(t, first.Items, 2)
	require.NotEmpty(t, first.Next)
	second := page("?limite=2&cursor=" + first.Next)
	require.Len(t, second.Items, 1)
	assert.Empty(t, second.Next)
	assert.Equal(t, http.StatusUnprocessableEntity, planCall(t, s.app, http.MethodGet, s.gastosPath()+"?concepto=propina", nil, s.b.token, "").status)
	assert.Equal(t, http.StatusNotFound, planCall(t, s.app, http.MethodGet, s.gastosPath(), nil, s.outsider.token, "").status)
}

func TestVaca_EditarBorrarYDeudasPagadas(t *testing.T) {
	s := newVaca(t, "vaca_editar")
	created := s.add(t, s.a, gasto(s.org, 900, vaca.ConceptFood), http.StatusCreated)
	path := "/api/v1/gastos/" + created.Expense.ID
	change := map[string]any{"monto_centavos": 600}

	assert.Equal(t, http.StatusBadRequest, planCall(t, s.app, http.MethodPatch, path, change, s.a.token, "").status, "sin If-Match")
	assert.Equal(t, http.StatusConflict, planCall(t, s.app, http.MethodPatch, path, change, s.a.token, `"7"`).status, "versión vieja")
	assert.Equal(t, http.StatusForbidden, planCall(t, s.app, http.MethodPatch, path, change, s.b.token, `"1"`).status, "no lo capturó ni organiza")
	assert.Equal(t, http.StatusNotFound, planCall(t, s.app, http.MethodPatch, path, change, s.outsider.token, `"1"`).status)
	got := planCall(t, s.app, http.MethodPatch, path, change, s.a.token, `"1"`)
	require.Equal(t, http.StatusOK, got.status, got.body)
	assert.Equal(t, `"2"`, got.etag)

	// b le paga su parte a org; ya pagada = 409; un tercero no la marca (403).
	cut := s.cut(t, s.b)
	var debt vaca.DebtView
	for _, d := range cut.Debts {
		if d.Debtor.UserID == s.b.id() {
			debt = d
		}
	}
	require.NotEmpty(t, debt.ID)
	debtPath := "/api/v1/vaca/deudas/" + debt.ID
	paid := map[string]string{"estado": "pagada", "referencia": "SPEI 58"}
	assert.Equal(t, http.StatusForbidden, planCall(t, s.app, http.MethodPatch, debtPath, paid, s.a.token, "").status)
	marked := planCall(t, s.app, http.MethodPatch, debtPath, paid, s.b.token, "")
	require.Equal(t, http.StatusOK, marked.status, marked.body)
	assert.Contains(t, marked.body, `"estado":"pagada"`)
	assert.Equal(t, http.StatusConflict, planCall(t, s.app, http.MethodPatch, debtPath, paid, s.org.token, "").status)

	// Con deudas pagadas, editar pide confirmar; con la confirmación aparece la deuda de regreso.
	lower := planCall(t, s.app, http.MethodPatch, path, map[string]any{"monto_centavos": 300}, s.org.token, `"2"`)
	require.Equal(t, http.StatusConflict, lower.status)
	assert.Contains(t, lower.body, "deudas_pagadas")
	lower = planCall(t, s.app, http.MethodPatch, path, map[string]any{"monto_centavos": 300, "confirmar": true}, s.org.token, `"2"`)
	require.Equal(t, http.StatusOK, lower.status, lower.body)
	after := s.cut(t, s.b)
	assert.Positive(t, sumDebts(after.Debts, vaca.DebtPending))
	assert.Equal(t, int64(200), sumDebts(after.Debts, vaca.DebtPaid), "la pagada se conserva")
	var back bool
	for _, d := range after.Debts {
		back = back || (d.Status == vaca.DebtPending && d.Creditor.UserID == s.b.id())
	}
	assert.True(t, back, "b pagó de más: ahora le deben a él")

	// Borrar: sin confirmar = 409; con confirmar = 204; luego ya no existe.
	assert.Equal(t, http.StatusConflict, planCall(t, s.app, http.MethodDelete, path, nil, s.a.token, `"3"`).status)
	assert.Equal(t, http.StatusNoContent, planCall(t, s.app, http.MethodDelete, path+"?confirmar=true", nil, s.a.token, `"3"`).status)
	assert.Equal(t, http.StatusNotFound, planCall(t, s.app, http.MethodDelete, path+"?confirmar=true", nil, s.a.token, `"3"`).status, "ya borrado")
	assert.Equal(t, int64(0), s.cut(t, s.org).Total)
}

func TestVaca_MisDeudasEnDosGrupos(t *testing.T) {
	s := newVaca(t, "vaca_mias")
	other := seedVacaGroup(t, s.env, s.a, []amigo{s.org}, nil)
	s.add(t, s.org, gasto(s.org, 300, vaca.ConceptFood), http.StatusCreated)
	g2 := planCall(t, s.app, http.MethodPost, "/api/v1/grupos/"+other.ID.Hex()+"/gastos", gasto(s.a, 1000, vaca.ConceptTransport), s.a.token, "")
	require.Equal(t, http.StatusCreated, g2.status, g2.body)

	got := planCall(t, s.app, http.MethodGet, "/api/v1/vaca/mis-deudas", nil, s.org.token, "")
	require.Equal(t, http.StatusOK, got.status, got.body)
	var mine vaca.MyDebtsView
	require.NoError(t, json.Unmarshal([]byte(got.body), &mine))
	assert.Equal(t, int64(500), mine.TotalOwe, "le debe la mitad del taxi a a")
	assert.Equal(t, int64(200), mine.TotalOwed, "a y b le deben 100 cada uno")
	require.Len(t, mine.Owe, 1)
	assert.Equal(t, other.ID.Hex(), mine.Owe[0].GroupID)
	assert.Equal(t, http.StatusUnprocessableEntity, planCall(t, s.app, http.MethodGet, "/api/v1/vaca/mis-deudas?estado=x", nil, s.org.token, "").status)
}

// TestVaca_Concurrencia_UnSoloCorte: diez altas a la vez dejan un solo juego de deudas
// pendientes (una version_corte) que cuadra.
func TestVaca_Concurrencia_UnSoloCorte(t *testing.T) {
	s := newVaca(t, "vaca_conc")
	people := []amigo{s.org, s.a, s.b}
	var wg sync.WaitGroup
	codes := make([]int, 10)
	for i := range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			who := people[i%3]
			codes[i] = planCall(t, s.app, http.MethodPost, s.gastosPath(), gasto(who, int64(100*(i+1)), vaca.ConceptOther), who.token, "").status
		}()
	}
	wg.Wait()
	for i, c := range codes {
		assert.Equal(t, http.StatusCreated, c, fmt.Sprintf("alta %d", i))
	}
	cut := s.cut(t, s.org)
	assert.Equal(t, int64(5500), cut.Total)
	versions := map[int]bool{}
	for _, d := range cut.Debts {
		versions[d.CutVersion] = true
	}
	assert.Len(t, versions, 1, "una sola version_corte")
	var sum int64
	for _, p := range cut.People {
		sum += p.Balance
	}
	assert.Zero(t, sum)
	pending, err := s.env.Mongo.DB().Collection(vaca.DebtsCollection).CountDocuments(context.Background(), bson.D{{Key: "grupo_id", Value: s.group.ID}})
	require.NoError(t, err)
	assert.Equal(t, int64(len(cut.Debts)), pending)
}

func TestVaca_BorrarCuentaYExportar(t *testing.T) {
	s := newVaca(t, "vaca_arco")
	created := s.add(t, s.a, gasto(s.a, 900, vaca.ConceptFood), http.StatusCreated)
	repo := vaca.NewRepository(s.env.Mongo.DB())

	expenses, err := vaca.NewExpensesPrivacy(repo).Export(context.Background(), s.a.user.ID)
	require.NoError(t, err)
	raw, err := json.Marshal(expenses)
	require.NoError(t, err)
	assert.Contains(t, string(raw), created.Expense.ID)
	assert.Contains(t, string(raw), `"lo_pague":true`)
	debts, err := vaca.NewDebtsPrivacy(repo).Export(context.Background(), s.org.user.ID)
	require.NoError(t, err)
	raw, err = json.Marshal(debts)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"rol":"deudor"`)
	assert.NotContains(t, string(raw), s.a.id(), "no exporta ids de los demás")

	// Borrar la cuenta conserva la Vaca y muestra «Cuenta eliminada».
	require.NoError(t, vaca.NewExpensesPrivacy(repo).PurgeAccount(context.Background(), s.a.user.ID))
	require.NoError(t, vaca.NewDebtsPrivacy(repo).PurgeAccount(context.Background(), s.a.user.ID))
	_, err = s.env.Mongo.DB().Collection(usuarios.Collection).DeleteOne(context.Background(), bson.D{{Key: "_id", Value: s.a.user.ID}})
	require.NoError(t, err)
	cut := s.cut(t, s.org)
	assert.Equal(t, int64(900), cut.Total)
	var name string
	for _, p := range cut.People {
		if p.UserID == s.a.id() {
			name = p.Name
		}
	}
	assert.Equal(t, vaca.DeletedUserName, name)
}

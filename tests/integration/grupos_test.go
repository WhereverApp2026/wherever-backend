package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/grupos"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

const gruposPath = "/api/v1/grupos"

// gruposSetup es un organizador con un plan propio y la app lista.
type gruposSetup struct {
	env  *authtest.Env
	app  *fiber.App
	org  amigo
	plan planes.Plan
	shop bson.ObjectID
}

func newGrupos(t *testing.T) gruposSetup {
	t.Helper()
	env := negociosEnv(t)
	env.ClearRateLimits(t)
	shop := placeDoc(placeSpec{slug: "grupo-a"})
	_, err := env.Mongo.DB().Collection(negocios.Collection).InsertOne(context.Background(), shop)
	require.NoError(t, err)
	s := gruposSetup{env: env, app: env.App(), org: seedAmigo(t, env, "organiza"), shop: shop.ID}
	s.plan = s.newPlan(t, s.org)
	return s
}

// newPlan crea un plan de who para dentro de una semana.
func (s gruposSetup) newPlan(t *testing.T, who amigo) planes.Plan {
	t.Helper()
	day := time.Now().AddDate(0, 0, 7).Format(planes.DayLayout)
	body := map[string]any{
		"nombre": "Salida", "fecha": day, "personas": 4, "presupuesto": 2000, "movilidad": "a_pie",
		"paradas": []map[string]any{{"negocio_id": s.shop.Hex(), "llega": day + "T12:00:00-06:00", "minutos": 60, "precio": 100}},
	}
	got := planCall(t, s.app, http.MethodPost, "/api/v1/planes", body, who.token, "")
	require.Equal(t, http.StatusCreated, got.status, got.body)
	return got.plan(t)
}

// befriend deja a a y b como amigos aceptados.
func befriend(t *testing.T, env *authtest.Env, a, b amigo) {
	t.Helper()
	now := time.Now().UTC()
	f := amistades.Friendship{
		ID: bson.NewObjectID(), From: a.user.ID, To: b.user.ID, Status: amistades.StatusAccepted,
		Pair: amistades.PairKey(a.user.ID, b.user.ID), CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, amistades.NewRepository(env.Mongo.DB()).Insert(context.Background(), &f))
}

// friends siembra n amigos del organizador.
func (s gruposSetup) friends(t *testing.T, prefix string, n int) []amigo {
	t.Helper()
	out := make([]amigo, 0, n)
	for i := range n {
		f := seedAmigo(t, s.env, fmt.Sprintf("%s_%d", prefix, i))
		befriend(t, s.env, s.org, f)
		out = append(out, f)
	}
	return out
}

func idsOf(people []amigo) []string {
	out := make([]string, 0, len(people))
	for _, p := range people {
		out = append(out, p.id())
	}
	return out
}

func grupoCall(t *testing.T, app *fiber.App, method, path string, body any, token string, want int) grupos.View {
	t.Helper()
	resp := sendJSON(t, app, method, path, body, token)
	raw := readAll(t, resp)
	require.Equal(t, want, resp.StatusCode, raw)
	var v grupos.View
	if want == http.StatusOK || want == http.StatusCreated {
		require.NoError(t, json.Unmarshal([]byte(raw), &v), raw)
		assert.Equal(t, "no-store", resp.Header.Get(fiber.HeaderCacheControl))
	}
	return v
}

func (s gruposSetup) create(t *testing.T, invited []amigo) grupos.View {
	t.Helper()
	return grupoCall(t, s.app, http.MethodPost, gruposPath, map[string]any{"plan_id": s.plan.ID.Hex(), "invitados": idsOf(invited)}, s.org.token, http.StatusCreated)
}

func statusOf(v grupos.View, who amigo) string {
	for _, m := range v.Members {
		if m.UserID == who.id() {
			return m.Status
		}
	}
	return ""
}

func TestGrupos_CrearConInvitadosYVerlo(t *testing.T) {
	s := newGrupos(t)
	fr := s.friends(t, "inv", 2)
	v := s.create(t, fr)
	assert.Equal(t, s.plan.ID.Hex(), v.PlanID)
	assert.Equal(t, grupos.RoleOrganizer, v.MyRole)
	assert.Equal(t, grupos.Seats{Max: 12, Occupied: 3}, v.Seats)
	assert.Equal(t, grupos.StatusInvited, statusOf(v, fr[0]))
	require.NotNil(t, v.Members[1].Person)
	assert.Equal(t, "inv_0", v.Members[1].Person.Username)

	// El invitado lo ve y lo encuentra en su lista; el segundo grupo del plan es 409.
	mine := grupoCall(t, s.app, http.MethodGet, gruposPath+"/"+v.ID, nil, fr[0].token, http.StatusOK)
	assert.Equal(t, grupos.StatusInvited, mine.MyStatus)
	resp := sendJSON(t, s.app, http.MethodGet, gruposPath, nil, fr[0].token)
	body := decode(t, resp)
	assert.Len(t, list(t, body["items"]), 1)
	grupoCall(t, s.app, http.MethodPost, gruposPath, map[string]any{"plan_id": s.plan.ID.Hex()}, s.org.token, http.StatusConflict)
}

func TestGrupos_SoloAmigosActivos(t *testing.T) {
	s := newGrupos(t)
	stranger := seedAmigo(t, s.env, "extrano")
	inactive := s.friends(t, "inactivo", 1)[0]
	_, err := s.env.Users.Update(context.Background(), inactive.user.ID, bson.D{{Key: "estado", Value: usuarios.StatusPendingDeletion}})
	require.NoError(t, err)
	pending := seedAmigo(t, s.env, "pendiente")
	now := time.Now().UTC()
	require.NoError(t, amistades.NewRepository(s.env.Mongo.DB()).Insert(context.Background(), &amistades.Friendship{
		ID: bson.NewObjectID(), From: s.org.user.ID, To: pending.user.ID, Status: amistades.StatusPending,
		Pair: amistades.PairKey(s.org.user.ID, pending.user.ID), CreatedAt: now, UpdatedAt: now,
	}))
	for _, who := range []amigo{stranger, inactive, pending, s.org} {
		grupoCall(t, s.app, http.MethodPost, gruposPath, map[string]any{"plan_id": s.plan.ID.Hex(), "invitados": []string{who.id()}}, s.org.token, http.StatusUnprocessableEntity)
	}
	n, err := s.env.Mongo.DB().Collection(grupos.Collection).CountDocuments(context.Background(), bson.D{})
	require.NoError(t, err)
	assert.Zero(t, n, "si un invitado no vale, no se crea el grupo")
}

func TestGrupos_PlanAjenoYGrupoAjenoSon404(t *testing.T) {
	s := newGrupos(t)
	other := seedAmigo(t, s.env, "otro")
	grupoCall(t, s.app, http.MethodPost, gruposPath, map[string]any{"plan_id": s.plan.ID.Hex()}, other.token, http.StatusNotFound)
	v := s.create(t, nil)
	grupoCall(t, s.app, http.MethodGet, gruposPath+"/"+v.ID, nil, other.token, http.StatusNotFound)
	grupoCall(t, s.app, http.MethodPost, gruposPath+"/"+v.ID+"/integrantes", map[string]any{"usuarios": []string{other.id()}}, other.token, http.StatusNotFound)
}

func TestGrupos_ResponderSalirYSacar(t *testing.T) {
	s := newGrupos(t)
	fr := s.friends(t, "resp", 3)
	v := s.create(t, fr)
	path := gruposPath + "/" + v.ID + "/integrantes"

	// Un integrante no invita ni saca (403).
	grupoCall(t, s.app, http.MethodPost, path, map[string]any{"usuarios": []string{fr[1].id()}}, fr[0].token, http.StatusForbidden)
	grupoCall(t, s.app, http.MethodDelete, path+"/"+fr[1].id(), nil, fr[0].token, http.StatusForbidden)

	got := grupoCall(t, s.app, http.MethodPatch, path+"/yo", map[string]string{"estado": "confirmado"}, fr[0].token, http.StatusOK)
	assert.Equal(t, grupos.StatusConfirmed, got.MyStatus)
	got = grupoCall(t, s.app, http.MethodPatch, path+"/yo", map[string]string{"estado": "declinado"}, fr[1].token, http.StatusOK)
	assert.Equal(t, 3, got.Seats.Occupied, "declinar libera su lugar")

	// fr[0] no ve a quien declinó; el organizador sí, con el resumen.
	seen := grupoCall(t, s.app, http.MethodGet, gruposPath+"/"+v.ID, nil, fr[0].token, http.StatusOK)
	assert.Empty(t, statusOf(seen, fr[1]))
	org := grupoCall(t, s.app, http.MethodGet, gruposPath+"/"+v.ID, nil, s.org.token, http.StatusOK)
	assert.Equal(t, grupos.StatusDeclined, statusOf(org, fr[1]))
	require.NotNil(t, org.Summary.Declined)
	assert.Equal(t, 1, *org.Summary.Declined)

	// Vuelve a confirmar, sale y el organizador saca a otro.
	got = grupoCall(t, s.app, http.MethodPatch, path+"/yo", map[string]string{"estado": "confirmado"}, fr[1].token, http.StatusOK)
	assert.Equal(t, 4, got.Seats.Occupied)
	grupoCall(t, s.app, http.MethodDelete, path+"/yo", nil, fr[1].token, http.StatusNoContent)
	grupoCall(t, s.app, http.MethodGet, gruposPath+"/"+v.ID, nil, fr[1].token, http.StatusNotFound)
	grupoCall(t, s.app, http.MethodDelete, path+"/"+fr[2].id(), nil, s.org.token, http.StatusNoContent)
	grupoCall(t, s.app, http.MethodDelete, path+"/"+fr[2].id(), nil, s.org.token, http.StatusNotFound)
	org = grupoCall(t, s.app, http.MethodGet, gruposPath+"/"+v.ID, nil, s.org.token, http.StatusOK)
	assert.Equal(t, 2, org.Seats.Occupied)

	// El organizador no responde, no sale ni se saca (422).
	grupoCall(t, s.app, http.MethodPatch, path+"/yo", map[string]string{"estado": "declinado"}, s.org.token, http.StatusUnprocessableEntity)
	grupoCall(t, s.app, http.MethodDelete, path+"/yo", nil, s.org.token, http.StatusUnprocessableEntity)
	grupoCall(t, s.app, http.MethodDelete, path+"/"+s.org.id(), nil, s.org.token, http.StatusUnprocessableEntity)
}

func TestGrupos_IntegranteLeeElPlanYNoLoEdita(t *testing.T) {
	s := newGrupos(t)
	fr := s.friends(t, "lector", 2)
	v := s.create(t, fr[:1])
	got := planCall(t, s.app, http.MethodGet, planPath(s.plan, ""), nil, fr[0].token, "")
	require.Equal(t, http.StatusOK, got.status, got.body)
	assert.Equal(t, s.plan.ID, got.plan(t).ID)
	edit := planCall(t, s.app, http.MethodPatch, planPath(s.plan, ""), map[string]any{"nombre": "Mío"}, fr[0].token, `"1"`)
	assert.Equal(t, http.StatusNotFound, edit.status, "solo el dueño edita")
	// Un amigo que no está en el grupo no lo ve; quien declinó tampoco.
	assert.Equal(t, http.StatusNotFound, planCall(t, s.app, http.MethodGet, planPath(s.plan, ""), nil, fr[1].token, "").status)
	grupoCall(t, s.app, http.MethodPatch, gruposPath+"/"+v.ID+"/integrantes/yo", map[string]string{"estado": "declinado"}, fr[0].token, http.StatusOK)
	assert.Equal(t, http.StatusNotFound, planCall(t, s.app, http.MethodGet, planPath(s.plan, ""), nil, fr[0].token, "").status)
}

func TestGrupos_PlanBorradoOcultaElGrupo(t *testing.T) {
	s := newGrupos(t)
	fr := s.friends(t, "borrado", 1)
	v := s.create(t, fr)
	require.Equal(t, http.StatusNoContent, planCall(t, s.app, http.MethodDelete, planPath(s.plan, ""), nil, s.org.token, `"1"`).status)
	grupoCall(t, s.app, http.MethodGet, gruposPath+"/"+v.ID, nil, fr[0].token, http.StatusNotFound)
	body := decode(t, sendJSON(t, s.app, http.MethodGet, gruposPath, nil, fr[0].token))
	assert.Empty(t, list(t, body["items"]))
}

func TestGrupos_InvitacionesDelPlanCreanUnSoloGrupo(t *testing.T) {
	s := newGrupos(t)
	fr := s.friends(t, "plan", 2)
	path := planPath(s.plan, "/invitaciones")
	a := grupoCall(t, s.app, http.MethodPost, path, map[string]any{"usuarios": []string{fr[0].id()}}, s.org.token, http.StatusOK)
	b := grupoCall(t, s.app, http.MethodPost, path, map[string]any{"usuarios": idsOf(fr)}, s.org.token, http.StatusOK)
	assert.Equal(t, a.ID, b.ID)
	assert.Equal(t, 3, b.Seats.Occupied, "re-invitar a quien ya está no ocupa otro lugar")
	other := seedAmigo(t, s.env, "ajeno_plan")
	grupoCall(t, s.app, http.MethodPost, path, map[string]any{"usuarios": []string{fr[0].id()}}, other.token, http.StatusNotFound)
}

func TestGrupos_ListaConCursor(t *testing.T) {
	s := newGrupos(t)
	guest := s.friends(t, "lista", 1)[0]
	for range 3 {
		p := s.newPlan(t, s.org)
		grupoCall(t, s.app, http.MethodPost, gruposPath, map[string]any{"plan_id": p.ID.Hex(), "invitados": []string{guest.id()}}, s.org.token, http.StatusCreated)
	}
	first := decode(t, sendJSON(t, s.app, http.MethodGet, gruposPath+"?limite=2", nil, guest.token))
	assert.Len(t, list(t, first["items"]), 2)
	next := str(t, first["siguiente_cursor"])
	require.NotEmpty(t, next)
	second := decode(t, sendJSON(t, s.app, http.MethodGet, gruposPath+"?limite=2&cursor="+next, nil, guest.token))
	assert.Len(t, list(t, second["items"]), 1)
	// El cursor de otra persona no sirve.
	resp := sendJSON(t, s.app, http.MethodGet, gruposPath+"?limite=2&cursor="+next, nil, s.org.token)
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
}

// 20 invitaciones simultáneas a un grupo con 11: entra exactamente una. Va directo al
// repositorio: por HTTP, el límite de 10 escrituras simultáneas del organizador respondería
// 429 antes de probar el tope.
func TestGrupos_Concurrencia_NuncaMasDeDoce(t *testing.T) {
	s := newGrupos(t)
	v := s.create(t, s.friends(t, "lleno", 10))
	require.Equal(t, 11, v.Seats.Occupied)
	repo := grupos.NewRepository(s.env.Mongo.DB())
	groupID := mustID(t, v.ID)

	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- repo.Invite(context.Background(), groupID, s.org.user.ID, bson.NewObjectID(), time.Now().UTC())
		}()
	}
	wg.Wait()
	close(errs)
	ok := 0
	for err := range errs {
		if err == nil {
			ok++
			continue
		}
		assert.ErrorIs(t, err, grupos.ErrFull)
	}
	assert.Equal(t, 1, ok)
	g, err := repo.FindByID(context.Background(), groupID)
	require.NoError(t, err)
	assert.Equal(t, 12, g.Occupied)
	assert.Len(t, g.Members, 12)

	// Por HTTP también: el grupo lleno responde 409.
	extra := s.friends(t, "sobra", 1)
	grupoCall(t, s.app, http.MethodPost, gruposPath+"/"+v.ID+"/integrantes", map[string]any{"usuarios": idsOf(extra)}, s.org.token, http.StatusConflict)
}

func TestGrupos_BorrarCuentaYExportar(t *testing.T) {
	s := newGrupos(t)
	fr := s.friends(t, "arco", 1)
	v := s.create(t, fr)
	got, err := grupos.NewPrivacy(grupos.NewRepository(s.env.Mongo.DB())).Export(context.Background(), fr[0].user.ID)
	require.NoError(t, err)
	raw, err := json.Marshal(got)
	require.NoError(t, err)
	assert.Contains(t, string(raw), v.ID)
	assert.NotContains(t, string(raw), s.org.id(), "no exporta a los demás integrantes")

	priv := grupos.NewPrivacy(grupos.NewRepository(s.env.Mongo.DB()))
	require.NoError(t, priv.PurgeAccount(context.Background(), fr[0].user.ID))
	org := grupoCall(t, s.app, http.MethodGet, gruposPath+"/"+v.ID, nil, s.org.token, http.StatusOK)
	assert.Equal(t, 1, org.Seats.Occupied)
	require.NoError(t, priv.PurgeAccount(context.Background(), s.org.user.ID))
	n, err := s.env.Mongo.DB().Collection(grupos.Collection).CountDocuments(context.Background(), bson.D{})
	require.NoError(t, err)
	assert.Zero(t, n, "los grupos que organizaba se borran")
}

func mustID(t *testing.T, hex string) bson.ObjectID {
	t.Helper()
	id, err := bson.ObjectIDFromHex(hex)
	require.NoError(t, err)
	return id
}

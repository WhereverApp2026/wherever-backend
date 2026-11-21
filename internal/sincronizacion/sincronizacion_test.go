package sincronizacion

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

// Unitarias de sincronización (nv-46, nv-47): lo que se decide antes de tocar la base. Lo que
// lee o escribe MongoDB se prueba en tests/integration (sync_test.go y pendientes_test.go).

func newValidator(t *testing.T) *validation.Validator {
	t.Helper()
	v, err := validation.New()
	require.NoError(t, err)
	return v
}

func newSigner(t *testing.T) *pagination.Signer {
	t.Helper()
	s, err := pagination.NewSigner([]byte("llave-de-cursor-de-prueba-32bytes!"))
	require.NoError(t, err)
	return s
}

func appErrOf(t *testing.T, err error) *apperr.Error {
	t.Helper()
	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	return appErr
}

func TestDTOs_AccionesValidas(t *testing.T) {
	v := newValidator(t)
	id := bson.NewObjectID().Hex()
	for _, a := range []Action{
		{ClientID: "cliente-0001", Type: ActionPlanCreate},
		{ClientID: "cliente-0002", Type: ActionPlanEdit, PlanID: id, Version: 3},
		{ClientID: "cliente-0003", Type: ActionPlanDelete, PlanID: id},
		{ClientID: "cliente-0004", Type: ActionStopAdd, PlanID: id},
		{ClientID: "cliente-0005", Type: ActionStopEdit, PlanID: id, StopID: id},
		{ClientID: "cliente-0006", Type: ActionStopRemove, PlanID: id, StopID: id},
		{ClientID: "cliente-0007", Type: ActionPrefsEdit},
		{ClientID: "cliente-0008", Type: ActionPrivacyEdit, Version: 1},
	} {
		assert.NoError(t, v.Struct(PendingRequest{Actions: []Action{a}}), a.Type)
	}
	assert.NoError(t, v.Struct(NewChangesQuery()))
}

func TestDTOs_AccionesInvalidasSon422(t *testing.T) {
	v := newValidator(t)
	id := bson.NewObjectID().Hex()
	cases := map[string]PendingRequest{
		"lote vacío":              {},
		"id_cliente corto":        {Actions: []Action{{ClientID: "corto", Type: ActionPlanCreate}}},
		"tipo desconocido":        {Actions: []Action{{ClientID: "cliente-0001", Type: "plan.copiar"}}},
		"editar sin plan_id":      {Actions: []Action{{ClientID: "cliente-0001", Type: ActionPlanEdit}}},
		"plan_id que no es id":    {Actions: []Action{{ClientID: "cliente-0001", Type: ActionPlanDelete, PlanID: "no-es-id"}}},
		"editar parada sin su id": {Actions: []Action{{ClientID: "cliente-0001", Type: ActionStopEdit, PlanID: id}}},
		"quitar parada sin su id": {Actions: []Action{{ClientID: "cliente-0001", Type: ActionStopRemove, PlanID: id}}},
		"versión negativa":        {Actions: []Action{{ClientID: "cliente-0001", Type: ActionPlanEdit, PlanID: id, Version: -1}}},
		"más de 100 acciones":     {Actions: make([]Action, MaxActions+1)},
	}
	for name, req := range cases {
		assert.Equal(t, apperr.KindValidation, appErrOf(t, v.Struct(req)).Kind, name)
	}
	for name, q := range map[string]ChangesQuery{
		"límite 0":           {Limit: 0},
		"límite sobre 200":   {Limit: MaxLimit + 1},
		"desde no es cursor": {Limit: DefaultLimit, From: "no es un cursor"},
	} {
		assert.Equal(t, apperr.KindValidation, appErrOf(t, v.Struct(q)).Kind, name)
	}
}

func TestCursor_IdaYVueltaYSoloDelMismoUsuario(t *testing.T) {
	s := &Service{cursor: newSigner(t)}
	me := bson.NewObjectID()
	issued := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	raw, err := s.encode(me, position{Seq: 42, Profile: 7, Issued: issued})
	require.NoError(t, err)

	got, err := s.decode(me, raw)
	require.NoError(t, err)
	assert.Equal(t, position{Seq: 42, Profile: 7, Issued: issued}, got)

	empty, err := s.decode(me, "")
	require.NoError(t, err)
	assert.Equal(t, position{}, empty, "sin cursor es la descarga completa")

	_, err = s.decode(bson.NewObjectID(), raw)
	assert.Equal(t, apperr.KindValidation, appErrOf(t, err).Kind, "el cursor de otra persona no sirve")
	_, err = s.decode(me, raw+"x")
	assert.Equal(t, apperr.KindValidation, appErrOf(t, err).Kind, "cursor alterado")
}

func TestCursor_OrdenMalFormadoEsInvalido(t *testing.T) {
	signer := newSigner(t)
	s := &Service{cursor: signer}
	me := bson.NewObjectID()
	for _, sort := range []string{"1:2", "a:1:1", "1:b:1", "1:1:c", "-1:1:1"} {
		raw, err := signer.Encode(pagination.Cursor{ID: me.Hex(), Scope: s.scope(me), Sort: sort})
		require.NoError(t, err)
		_, err = s.decode(me, raw)
		assert.Equal(t, apperr.KindValidation, appErrOf(t, err).Kind, sort)
	}
}

// Cuentas inválidas, cursores malos y cursores de más de 30 días fallan antes de leer la base.
func TestChanges_FallaAntesDeLaBase(t *testing.T) {
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	s := &Service{cursor: newSigner(t), now: func() time.Time { return now }}
	me := bson.NewObjectID()
	ctx := context.Background()

	_, err := s.Changes(ctx, "no-es-id", NewChangesQuery())
	assert.Equal(t, apperr.KindUnauthorized, appErrOf(t, err).Kind)

	_, err = s.Changes(ctx, me.Hex(), ChangesQuery{Limit: DefaultLimit, From: "basura"})
	assert.Equal(t, apperr.KindValidation, appErrOf(t, err).Kind)

	old, err := s.encode(me, position{Seq: 5, Profile: 1, Issued: now.Add(-Retention - time.Hour)})
	require.NoError(t, err)
	_, err = s.Changes(ctx, me.Hex(), ChangesQuery{Limit: DefaultLimit, From: old})
	gone := appErrOf(t, err)
	assert.Equal(t, apperr.KindGone, gone.Kind)
	require.Len(t, gone.Fields, 1)
	assert.Equal(t, apperr.FieldError{Field: "desde", Message: "requiere_completa"}, gone.Fields[0])
}

func TestSplit_SeparaPlanesYBorradosYDaElUltimo(t *testing.T) {
	plan := planes.Plan{ID: bson.NewObjectID(), SyncSeq: 11}
	tomb := Tombstone{Kind: "plan", EntityID: bson.NewObjectID(), Seq: 12}
	out, last := split([]change{{seq: 11, plan: &plan}, {seq: 12, tomb: &tomb}}, 10)
	assert.Equal(t, int64(12), last)
	require.Len(t, out.Plans, 1)
	assert.Equal(t, plan.ID, out.Plans[0].ID)
	assert.Equal(t, []DeletedItem{{Kind: "plan", ID: tomb.EntityID.Hex()}}, out.Deleted)

	empty, last := split(nil, 10)
	assert.Equal(t, int64(10), last, "sin cambios el cursor no avanza")
	assert.NotNil(t, empty.Plans, "listas vacías, no null, en el JSON")
	assert.NotNil(t, empty.Deleted)
}

func TestDuplicateOf_DevuelveElResultadoOriginal(t *testing.T) {
	planID := bson.NewObjectID()
	r := duplicateOf(ActionRecord{ClientID: "cliente-0001", Result: ResultApplied, PlanID: &planID})
	assert.Equal(t, ActionResult{ClientID: "cliente-0001", Result: ResultDuplicate, Original: ResultApplied, PlanID: planID.Hex()}, r)

	r = duplicateOf(ActionRecord{ClientID: "cliente-0002", Result: ResultRejected})
	assert.Empty(t, r.PlanID)
	assert.Equal(t, ResultRejected, r.Original)
}

func TestPending_DecodeExigeDatosLegiblesYValidos(t *testing.T) {
	p := &Pending{validator: newValidator(t)}
	var req planes.CreateRequest
	assert.Equal(t, apperr.KindValidation, appErrOf(t, p.decode(Action{}, &req)).Kind, "sin datos")
	assert.Equal(t, apperr.KindBadRequest, appErrOf(t, p.decode(Action{Data: json.RawMessage(`{`)}, &req)).Kind, "JSON roto")
	assert.Equal(t, apperr.KindValidation, appErrOf(t, p.decode(Action{Data: json.RawMessage(`{}`)}, &req)).Kind, "datos que no pasan la validación")
}

// Un error de la petición rechaza esa acción; uno de la base detiene el lote (503).
func TestPending_FailedSeparaRechazoDeFallaDeInfraestructura(t *testing.T) {
	p := &Pending{}
	ctx := context.Background()
	a := Action{ClientID: "cliente-0001", Type: ActionPlanEdit}

	for name, err := range map[string]error{
		"error sin tipo": errors.New("conexión perdida"),
		"no disponible":  apperr.Unavailable("base caída"),
		"interno":        &apperr.Error{Kind: apperr.KindInternal, Message: "falla"},
	} {
		assert.True(t, p.failed(ctx, "", a, err).infraErr, name)
	}

	o := p.failed(ctx, "", a, apperr.Validation("datos inválidos", apperr.FieldError{Field: "nombre", Message: "requerido"}))
	require.False(t, o.infraErr)
	assert.Equal(t, ResultRejected, o.result.Result)
	require.NotNil(t, o.result.Error)
	assert.Equal(t, apperr.KindValidation.Code(), o.result.Error.Code)
	assert.Equal(t, []apperr.FieldError{{Field: "nombre", Message: "requerido"}}, o.result.Error.Fields)

	o = p.failed(ctx, "", a, apperr.NotFound("plan no encontrado"))
	assert.Equal(t, ResultRejected, o.result.Result)
	assert.Equal(t, apperr.KindNotFound.Code(), o.result.Error.Code)
}

func TestPending_ApplyConCuentaInvalidaEs401(t *testing.T) {
	p := &Pending{}
	claims := &middleware.Claims{}
	claims.Subject = "no-es-id"
	_, err := p.Apply(context.Background(), claims, PendingRequest{})
	assert.Equal(t, apperr.KindUnauthorized, appErrOf(t, err).Kind)
}

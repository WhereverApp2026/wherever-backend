package planes

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

func validCreate(stops int) CreateRequest {
	r := CreateRequest{Nombre: "Domingo", Fecha: "2026-11-15", Personas: 2, Movilidad: MobilityWalk}
	for i := 0; i < stops; i++ {
		r.Paradas = append(r.Paradas, StopRequest{
			NegocioID: bson.NewObjectID().Hex(), Llega: time.Date(2026, 11, 15, 16+i, 0, 0, 0, time.UTC).Format(StampLayout), Minutos: 45, Precio: 100,
		})
	}
	return r
}

func newValidator(t *testing.T) *validation.Validator {
	t.Helper()
	v, err := validation.New()
	require.NoError(t, err)
	return v
}

func assertField(t *testing.T, err error, field string) {
	t.Helper()
	var ae *apperr.Error
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, apperr.KindValidation, ae.Kind)
	if field != "" {
		require.NotEmpty(t, ae.Fields)
		assert.Contains(t, ae.Fields[0].Field, field)
	}
}

func TestPlanes_ValidacionCrear(t *testing.T) {
	v := newValidator(t)
	require.NoError(t, v.Struct(validCreate(6)))

	cases := map[string]func(*CreateRequest){
		"siete paradas":   func(r *CreateRequest) { *r = validCreate(7) },
		"sin paradas":     func(r *CreateRequest) { r.Paradas = nil },
		"movilidad":       func(r *CreateRequest) { r.Movilidad = "cohete" },
		"fecha con hora":  func(r *CreateRequest) { r.Fecha = "2026-11-15T10:00:00Z" },
		"sin nombre":      func(r *CreateRequest) { r.Nombre = "" },
		"personas cero":   func(r *CreateRequest) { r.Personas = 0 },
		"origen raro":     func(r *CreateRequest) { r.Origen = "magia" },
		"negocio_id malo": func(r *CreateRequest) { r.Paradas[0].NegocioID = "x" },
		"llega sin zona":  func(r *CreateRequest) { r.Paradas[0].Llega = "2026-11-15T16:00:00" },
		"minutos cero":    func(r *CreateRequest) { r.Paradas[0].Minutos = 0 },
		"precio negativo": func(r *CreateRequest) { r.Paradas[0].Precio = -1 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			r := validCreate(1)
			mutate(&r)
			assertField(t, v.Struct(r), "")
		})
	}
}

func TestPlanes_ValidacionEdiciones(t *testing.T) {
	v := newValidator(t)
	empty := ""
	zero := 0
	assertField(t, v.Struct(PatchRequest{Nombre: &empty}), "")
	assertField(t, v.Struct(PatchRequest{Personas: &zero}), "")
	assertField(t, v.Struct(PatchStopRequest{Minutos: &zero}), "")
	assertField(t, v.Struct(OrderRequest{Orden: []string{"no-es-id"}}), "")
	assertField(t, v.Struct(CloneRequest{}), "")
	require.NoError(t, v.Struct(PatchRequest{}))
	require.NoError(t, v.Struct(CloneRequest{Fecha: "2026-12-01"}))

	q := NewListQuery()
	require.NoError(t, v.Struct(q))
	q.Pestana = "grupales"
	assertField(t, v.Struct(q), "")
}

func TestPlanes_ToPlanYFechaDeMexico(t *testing.T) {
	owner := bson.NewObjectID()
	p, err := validCreate(2).toPlan(owner)
	require.NoError(t, err)
	assert.Equal(t, owner, p.OwnerID)
	assert.Equal(t, OriginManual, p.Origen)
	assert.Len(t, p.Paradas, 2)
	assert.Equal(t, "2026-11-15T06:00:00Z", p.Fecha.UTC().Format(time.RFC3339), "medianoche de México")

	bad := validCreate(1)
	bad.Fecha = "2026-02-30"
	_, err = bad.toPlan(owner)
	assertField(t, err, "fecha")
}

func TestPlanes_PatchMueveParadasConLaFecha(t *testing.T) {
	p, err := validCreate(2).toPlan(bson.NewObjectID())
	require.NoError(t, err)
	first := p.Paradas[0].Llega
	day, name, people := "2026-11-18", "Otro", 4
	require.NoError(t, PatchRequest{Fecha: &day, Nombre: &name, Personas: &people}.apply(&p))
	assert.Equal(t, first.AddDate(0, 0, 3), p.Paradas[0].Llega)
	assert.Equal(t, "2026-11-18", p.Fecha.In(negocios.MexicoCity()).Format(DayLayout))
	assert.Equal(t, "Otro", p.Nombre)
	assert.Equal(t, 4, p.Personas)
	assert.Equal(t, MobilityWalk, p.Movilidad, "lo que no viene no cambia")

	bad := "2026-13-01"
	assertField(t, PatchRequest{Fecha: &bad}.apply(&p), "fecha")
}

func TestPlanes_PatchParada(t *testing.T) {
	s := Parada{Minutos: 30, Precio: 10}
	at, minutes, note := "2026-11-15T20:00:00-06:00", 90, "terraza"
	require.NoError(t, PatchStopRequest{Llega: &at, Minutos: &minutes, Nota: &note}.apply(&s))
	assert.Equal(t, time.Date(2026, 11, 16, 2, 0, 0, 0, time.UTC), s.Llega)
	assert.Equal(t, 90, s.Minutos)
	assert.Equal(t, 10, s.Precio)
	assert.Equal(t, "terraza", s.Nota)
}

func TestPlanes_ReordenarRepartaLasHoras(t *testing.T) {
	p, err := validCreate(3).toPlan(bson.NewObjectID())
	require.NoError(t, err)
	a, b, c := p.Paradas[0], p.Paradas[1], p.Paradas[2]
	require.NoError(t, reorder(p.Paradas, []string{c.ID.Hex(), a.ID.Hex(), b.ID.Hex()}))
	assert.Equal(t, []bson.ObjectID{c.ID, a.ID, b.ID}, []bson.ObjectID{p.Paradas[0].ID, p.Paradas[1].ID, p.Paradas[2].ID})
	assert.Equal(t, a.Llega, p.Paradas[0].Llega, "la primera hora sigue siendo la primera")
	assert.Equal(t, c.Llega, p.Paradas[2].Llega)

	assertField(t, reorder(p.Paradas, []string{a.ID.Hex(), a.ID.Hex(), b.ID.Hex()}), "orden")
	assertField(t, reorder(p.Paradas, []string{a.ID.Hex()}), "orden")
}

func TestPlanes_CursorDeOtraPestana(t *testing.T) {
	signer, err := pagination.NewSigner([]byte("solo_para_pruebas_cursor_no_usar_en_entornos_0789"))
	require.NoError(t, err)
	id := bson.NewObjectID()
	owner := bson.NewObjectID().Hex()
	past, upcoming := signer.Scope("planes", owner, TabPast), signer.Scope("planes", owner, TabUpcoming)
	raw, err := signer.Encode(pagination.Cursor{ID: id.Hex(), Sort: TabPast + ":2026-11-01T06:00:00Z", Scope: past})
	require.NoError(t, err)
	pos, err := position(signer, past, TabPast, raw)
	require.NoError(t, err)
	assert.Equal(t, id, pos.ID)

	_, err = position(signer, upcoming, TabUpcoming, raw)
	assertField(t, err, "cursor")
	_, err = position(signer, signer.Scope("planes", bson.NewObjectID().Hex(), TabPast), TabPast, raw)
	assertField(t, err, "cursor") // cursor de otra cuenta
	bad, err := signer.Encode(pagination.Cursor{ID: id.Hex(), Sort: TabPast + ":ayer", Scope: past})
	require.NoError(t, err)
	_, err = position(signer, past, TabPast, bad)
	assertField(t, err, "cursor")
	pos, err = position(signer, past, TabPast, "")
	require.NoError(t, err)
	assert.Nil(t, pos)
}

func TestPlanes_PestanasPorDiaDeMexico(t *testing.T) {
	s := &Service{now: func() time.Time { return time.Date(2026, 11, 16, 3, 0, 0, 0, time.UTC) }} // 15-nov 21:00 en México
	today := time.Date(2026, 11, 15, 6, 0, 0, 0, time.UTC)
	assert.True(t, s.tab(TabUpcoming).Since.Equal(today))
	assert.False(t, s.tab(TabUpcoming).Desc)
	assert.True(t, s.tab(TabPast).Before.Equal(today))
	assert.True(t, s.tab(TabPast).Desc)
	assert.True(t, s.tab(TabSaved).Since.IsZero())
}

func TestPlanes_ErroresDeCalculoSon422(t *testing.T) {
	for _, err := range []error{ErrStopCount, ErrMobility, ErrPeople, &StopError{Reason: ReasonClosed}} {
		assertField(t, planError(err), "")
	}
	var ae *apperr.Error
	require.ErrorAs(t, storeError(ErrVersionConflict), &ae)
	assert.Equal(t, apperr.KindConflict, ae.Kind)
	require.ErrorAs(t, storeError(ErrNotFound), &ae)
	assert.Equal(t, apperr.KindNotFound, ae.Kind)
}

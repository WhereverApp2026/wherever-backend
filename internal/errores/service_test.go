package errores

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/platform/logger"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

func TestScrub_SinDatosPersonales(t *testing.T) {
	jwt := "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
	in := "pago falló para ana.lopez@correo.mx tel +52 55 1234 5678 tarjeta 4111-1111-1111-1111 " +
		"token " + jwt + " uri mongodb://user:secreta@host/db en paso 3 de 12"
	out := Scrub(in)
	for _, secret := range []string{"ana.lopez", "@correo.mx", "1234 5678", "4111", "eyJ", "secreta"} {
		assert.NotContains(t, out, secret)
	}
	assert.Contains(t, out, logger.Redacted)
	assert.Contains(t, out, "en paso 3 de 12", "números cortos se quedan")
}

func TestFingerprint_EstableYSeparaModulos(t *testing.T) {
	a := Fingerprint("pagos", Scrub("falló para ana@x.mx"))
	b := Fingerprint("pagos", Scrub("falló para beto@y.mx"))
	assert.Equal(t, a, b, "el mismo error con otro correo cae en el mismo grupo")
	assert.Len(t, a, 64)
	assert.NotEqual(t, a, Fingerprint("sincronizacion", Scrub("falló para ana@x.mx")))
}

func TestValidacion_Reporte(t *testing.T) {
	v, err := validation.New()
	require.NoError(t, err)
	ok := ReportRequest{Module: "pagos", Message: "x", Version: "1.4.0-beta.1+77", TraceID: "abc-123"}
	require.NoError(t, v.Struct(ok))

	bad := []ReportRequest{
		{Module: "Pagos", Message: "x", Version: "1.0.0"},
		{Module: "p", Message: "x", Version: "1.0.0"},
		{Module: "pagos", Message: "", Version: "1.0.0"},
		{Module: "pagos", Message: strings.Repeat("x", 501), Version: "1.0.0"},
		{Module: "pagos", Message: "x", Version: "1.0"},
		{Module: "pagos", Message: "x", Version: "1.0.0-" + strings.Repeat("a", 20)},
		{Module: "pagos", Message: "x", Version: "1.0.0", TraceID: "con espacio"},
		{Module: "pagos", Message: "x", Version: "1.0.0", TraceID: strings.Repeat("a", 65)},
	}
	for _, r := range bad {
		var appErr *apperr.Error
		require.ErrorAs(t, v.Struct(r), &appErr, "%+v", r)
		assert.Equal(t, apperr.KindValidation, appErr.Kind)
	}
}

func TestValidacion_Lista(t *testing.T) {
	v, err := validation.New()
	require.NoError(t, err)
	q := NewListQuery()
	q.Since = "2026-11-01T00:00:00Z"
	require.NoError(t, v.Struct(q))
	for _, mod := range []func(*ListQuery){
		func(q *ListQuery) { q.Since = "2026-11-01" },
		func(q *ListQuery) { q.Limit = 101 },
		func(q *ListQuery) { q.Limit = 0 },
		func(q *ListQuery) { q.Module = "A" },
	} {
		q := NewListQuery()
		mod(&q)
		require.Error(t, v.Struct(q), "%+v", q)
	}
}

// recordingStore registra lo que el service pidió; no simula la base (eso es Integration).
type recordingStore struct {
	got Report
	f   Filter
	err error
}

func (s *recordingStore) Upsert(_ context.Context, r Report) error { s.got = r; return s.err }

func (s *recordingStore) List(_ context.Context, f Filter, _ int) ([]Report, error) {
	s.f = f
	return nil, s.err
}

func newTestService(t *testing.T, st Store) *Service {
	t.Helper()
	signer, err := pagination.NewSigner([]byte("llave-de-cursor-de-prueba-32bytes!"))
	require.NoError(t, err)
	svc, err := NewService(st, signer)
	require.NoError(t, err)
	return svc
}

func TestReport_GuardaFiltradoYAgrupa(t *testing.T) {
	st := &recordingStore{}
	svc := newTestService(t, st)
	require.NoError(t, svc.Report(context.Background(), ReportRequest{Module: "pagos", Message: " falló ana@x.mx ", Version: "1.0.0"}))
	assert.Equal(t, OriginApp, st.got.Origin)
	assert.NotContains(t, st.got.Message, "ana@x.mx")
	assert.Equal(t, Fingerprint("pagos", st.got.Message), st.got.Fingerprint)
	assert.False(t, st.got.LastAt.IsZero())
}

func TestService_Errores(t *testing.T) {
	_, err := NewService(nil, nil)
	require.Error(t, err)

	svc := newTestService(t, &recordingStore{err: errors.New("mongo caído")})
	var appErr *apperr.Error
	require.ErrorAs(t, svc.Report(context.Background(), ReportRequest{Module: "pagos", Message: "x", Version: "1.0.0"}), &appErr)
	assert.Equal(t, apperr.KindUnavailable, appErr.Kind)
	_, err = svc.List(context.Background(), NewListQuery())
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.KindUnavailable, appErr.Kind)

	q := NewListQuery()
	q.Cursor = "no-firmado"
	_, err = newTestService(t, &recordingStore{}).List(context.Background(), q)
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.KindValidation, appErr.Kind)
}

func TestList_FiltrosLleganAlRepositorio(t *testing.T) {
	st := &recordingStore{}
	q := NewListQuery()
	q.Module, q.Since = "pagos", "2026-11-01T06:00:00-06:00"
	out, err := newTestService(t, st).List(context.Background(), q)
	require.NoError(t, err)
	assert.Equal(t, "pagos", st.f.Module)
	require.NotNil(t, st.f.Since)
	assert.Equal(t, "2026-11-01T12:00:00Z", st.f.Since.Format("2006-01-02T15:04:05Z07:00"))
	assert.NotNil(t, out.Items)
}

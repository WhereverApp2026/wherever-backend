package personas

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

func TestParseQuery(t *testing.T) {
	cases := []struct {
		in   string
		kind Kind
		want string
	}{
		{"ana", KindUsername, "ana"},
		{"  @Ana_L ", KindUsername, "ana_l"},
		{"abc123", KindUsername, "abc123"},
		{"+52 55 1234-5678", KindPhone, "+525512345678"},
		{"(55) 1234.5678", KindPhone, "+525512345678"},
		{"5215512345678", KindPhone, "+5215512345678"},
		{"+14155550123", KindPhone, "+14155550123"},
	}
	for _, c := range cases {
		q, err := ParseQuery(c.in)
		require.NoError(t, err, c.in)
		assert.Equal(t, Query{Kind: c.kind, Value: c.want}, q, c.in)
	}
}

func TestParseQuery_Invalid422(t *testing.T) {
	for _, in := range []string{"", "  ", "ab", "con espacio", "ñandú", "+12", "+0123456789", "a_muy_largo_para_un_nombre_de_usuario"} {
		_, err := ParseQuery(in)
		var appErr *apperr.Error
		require.ErrorAs(t, err, &appErr, in)
		assert.Equal(t, apperr.KindValidation, appErr.Kind, in)
	}
}

// fakeUsers registra qué consulta hizo el service; no simula la base (eso es Integration).
type fakeUsers struct {
	prefix, hash string
	exclude      bson.ObjectID
	err          error
}

func (f *fakeUsers) SearchByUsernamePrefix(_ context.Context, prefix string, exclude bson.ObjectID, _ int64) ([]usuarios.User, error) {
	f.prefix, f.exclude = prefix, exclude
	return nil, f.err
}

func (f *fakeUsers) FindByPhoneSearchable(_ context.Context, hash string, exclude bson.ObjectID) (usuarios.User, bool, error) {
	f.hash, f.exclude = hash, exclude
	return usuarios.User{}, false, f.err
}

func TestSearch_UsaHMACYExcluyeAQuienBusca(t *testing.T) {
	key := []byte("llave-de-prueba")
	f := &fakeUsers{}
	svc, err := NewService(f, key, nil)
	require.NoError(t, err)
	me := bson.NewObjectID()

	out, err := svc.Search(context.Background(), me.Hex(), "55 1234 5678")
	require.NoError(t, err)
	assert.Equal(t, usuarios.HashPhone(key, "+525512345678"), f.hash)
	assert.Equal(t, me, f.exclude)
	assert.NotNil(t, out.Results, "lista vacía, nunca null")

	_, err = svc.Search(context.Background(), me.Hex(), "Ana")
	require.NoError(t, err)
	assert.Equal(t, "ana", f.prefix)
}

func TestSearch_Errores(t *testing.T) {
	_, err := NewService(nil, []byte("k"), nil)
	require.Error(t, err)
	_, err = NewService(&fakeUsers{}, nil, nil)
	require.Error(t, err)

	svc, err := NewService(&fakeUsers{err: errors.New("mongo caído")}, []byte("k"), nil)
	require.NoError(t, err)
	var appErr *apperr.Error
	_, err = svc.Search(context.Background(), "no-es-id", "ana")
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.KindUnauthorized, appErr.Kind)
	_, err = svc.Search(context.Background(), bson.NewObjectID().Hex(), "ana")
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.KindUnavailable, appErr.Kind)
}

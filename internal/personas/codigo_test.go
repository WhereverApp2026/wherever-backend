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

func kindOf(t *testing.T, err error) apperr.Kind {
	t.Helper()
	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	return appErr.Kind
}

func TestParseFriendCode(t *testing.T) {
	code, err := ParseFriendCode(" ab23cd45 ")
	require.NoError(t, err)
	assert.Equal(t, "AB23CD45", code, "minúsculas aceptadas")

	for _, in := range []string{"", "AB23CD4", "AB23CD456", "AB23CD40", "AB23CDI5", "AB23CD-5", "ÁB23CD45"} {
		_, err := ParseFriendCode(in)
		assert.Equal(t, apperr.KindValidation, kindOf(t, err), in)
	}
}

func TestRelationOf(t *testing.T) {
	cases := []struct {
		status string
		mine   bool
		want   string
		ok     bool
	}{
		{"", false, RelationNone, true},
		{"aceptada", true, RelationFriend, true},
		{"pendiente", true, RelationSent, true},
		{"pendiente", false, RelationReceived, true},
		{"bloqueada", true, "", false},
		{"bloqueada", false, "", false},
	}
	for _, c := range cases {
		rel, ok := relationOf(c.status, c.mine)
		assert.Equal(t, c.ok, ok, c.status)
		assert.Equal(t, c.want, rel, c.status)
	}
}

func codeService(t *testing.T, u *fakeUsers, b fakeBlocks) *Service {
	t.Helper()
	svc, err := NewService(u, b, []byte("k"), nil)
	require.NoError(t, err)
	return svc
}

func TestByFriendCode_EncontradoConRelacion(t *testing.T) {
	other := usuarios.User{ID: bson.NewObjectID(), Name: "Ana", Username: "ana", Email: "ana@x.mx", Status: usuarios.StatusActive}
	f := &fakeUsers{byCode: other}
	out, err := codeService(t, f, fakeBlocks{status: "pendiente", mine: false}).ByFriendCode(context.Background(), bson.NewObjectID().Hex(), "ab23cd45")
	require.NoError(t, err)
	assert.Equal(t, "AB23CD45", f.code)
	assert.Equal(t, CodeResponse{Person: Person{ID: other.ID.Hex(), Name: "Ana", Username: "ana"}, Relation: RelationReceived}, out)
}

func TestByFriendCode_404Uniforme(t *testing.T) {
	me := bson.NewObjectID()
	active := usuarios.User{ID: bson.NewObjectID(), Status: usuarios.StatusActive}
	cases := map[string]struct {
		u *fakeUsers
		b fakeBlocks
	}{
		"inexistente": {&fakeUsers{}, fakeBlocks{}},
		"propio":      {&fakeUsers{byCode: usuarios.User{ID: me, Status: usuarios.StatusActive}}, fakeBlocks{}},
		"pendiente":   {&fakeUsers{byCode: usuarios.User{ID: bson.NewObjectID(), Status: usuarios.StatusPendingDeletion}}, fakeBlocks{}},
		"bloqueado":   {&fakeUsers{byCode: usuarios.User{ID: bson.NewObjectID(), Status: usuarios.StatusBlocked}}, fakeBlocks{}},
		"bloqueé":     {&fakeUsers{byCode: active}, fakeBlocks{status: "bloqueada", mine: true}},
		"me bloqueó":  {&fakeUsers{byCode: active}, fakeBlocks{status: "bloqueada", mine: false}},
	}
	var first string
	for name, c := range cases {
		_, err := codeService(t, c.u, c.b).ByFriendCode(context.Background(), me.Hex(), "AB23CD45")
		assert.Equal(t, apperr.KindNotFound, kindOf(t, err), name)
		if first == "" {
			first = err.Error()
		}
		assert.Equal(t, first, err.Error(), "mismo mensaje: %s", name)
	}
}

func TestByFriendCode_Errores(t *testing.T) {
	svc := codeService(t, &fakeUsers{}, fakeBlocks{})
	_, err := svc.ByFriendCode(context.Background(), "no-es-id", "AB23CD45")
	assert.Equal(t, apperr.KindUnauthorized, kindOf(t, err))
	_, err = svc.ByFriendCode(context.Background(), bson.NewObjectID().Hex(), "malo")
	assert.Equal(t, apperr.KindValidation, kindOf(t, err))

	down := codeService(t, &fakeUsers{err: errors.New("mongo caído")}, fakeBlocks{})
	_, err = down.ByFriendCode(context.Background(), bson.NewObjectID().Hex(), "AB23CD45")
	assert.Equal(t, apperr.KindUnavailable, kindOf(t, err))

	active := usuarios.User{ID: bson.NewObjectID(), Status: usuarios.StatusActive}
	relDown := codeService(t, &fakeUsers{byCode: active}, fakeBlocks{err: errors.New("mongo caído")})
	_, err = relDown.ByFriendCode(context.Background(), bson.NewObjectID().Hex(), "AB23CD45")
	assert.Equal(t, apperr.KindUnavailable, kindOf(t, err))
}

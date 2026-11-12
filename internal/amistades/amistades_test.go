package amistades

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

func TestPairKey_EsElMismoEnAmbosSentidos(t *testing.T) {
	a, b := bson.NewObjectID(), bson.NewObjectID()
	assert.Equal(t, PairKey(a, b), PairKey(b, a))
	assert.NotEqual(t, PairKey(a, b), PairKey(a, bson.NewObjectID()))
}

func TestFriendship_OtherYDireccion(t *testing.T) {
	me, other := bson.NewObjectID(), bson.NewObjectID()
	f := Friendship{ID: bson.NewObjectID(), From: me, To: other, Status: StatusPending}
	assert.Equal(t, other, f.Other(me))
	assert.Equal(t, me, f.Other(other))

	p := usuarios.User{ID: other, Name: "Ana", Username: "ana"}
	assert.Equal(t, DirectionSent, newItem(f, me, p).Direction)
	assert.Equal(t, DirectionReceived, newItem(f, other, p).Direction)
	assert.Equal(t, "ana", newItem(f, me, p).Person.Username)
}

func TestMatches_SinAcentosNiMayusculas(t *testing.T) {
	p := usuarios.User{Name: "José Pérez", Username: "pepe_99"}
	for _, needle := range []string{"", "jose", "perez", "pepe"} {
		assert.True(t, matches(p, needle), needle)
	}
	assert.False(t, matches(p, "ana"))
}

func TestDTOs_Validacion(t *testing.T) {
	v, err := validation.New()
	require.NoError(t, err)
	assert.NoError(t, v.Struct(SendRequest{UserID: bson.NewObjectID().Hex()}))
	assert.NoError(t, v.Struct(RespondRequest{Action: ActionReject}))
	assert.NoError(t, v.Struct(ListQuery{Status: string(KindReceived), Query: "ana"}))
	for _, dto := range []any{
		SendRequest{}, SendRequest{UserID: "no-es-id"},
		RespondRequest{}, RespondRequest{Action: "ignorar"},
		ListQuery{Status: "bloqueadas"},
	} {
		var appErr *apperr.Error
		require.ErrorAs(t, v.Struct(dto), &appErr, dto)
		assert.Equal(t, apperr.KindValidation, appErr.Kind, dto)
	}
}

func TestService_EntradasInvalidas(t *testing.T) {
	_, err := NewService(nil, nil, nil)
	require.Error(t, err)

	// Sin base: estas rutas fallan antes de consultarla.
	svc := &Service{}
	ctx := context.Background()
	var appErr *apperr.Error
	_, err = svc.Send(ctx, "no-es-id", SendRequest{})
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.KindUnauthorized, appErr.Kind)

	me := bson.NewObjectID().Hex()
	_, err = svc.Send(ctx, me, SendRequest{UserID: me})
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.KindValidation, appErr.Kind, "no a uno mismo")

	for _, call := range []func() error{
		func() error { return svc.Remove(ctx, me, "x") },
		func() error { return svc.Block(ctx, me, "x") },
		func() error { _, err := svc.Respond(ctx, me, "x", RespondRequest{Action: ActionAccept}); return err },
	} {
		require.ErrorAs(t, call(), &appErr)
		assert.Equal(t, apperr.KindNotFound, appErr.Kind)
	}
}

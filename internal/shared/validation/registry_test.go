package validation

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

type demoDTO struct {
	Name   string `json:"nombre" validate:"required"`
	Amount int    `json:"monto_centavos" validate:"min=1"`
	Code   string `json:"codigo" validate:"omitempty,len=4"`
	Hidden string `json:"-" validate:"omitempty,max=2"`
}

func newValidator(t *testing.T) *Validator {
	t.Helper()
	v, err := New()
	require.NoError(t, err)
	return v
}

func fieldsOf(t *testing.T, err error) map[string]string {
	t.Helper()
	var appErr *apperr.Error
	require.True(t, errors.As(err, &appErr))
	require.Equal(t, apperr.KindValidation, appErr.Kind)
	out := map[string]string{}
	for _, f := range appErr.Fields {
		out[f.Field] = f.Message
	}
	return out
}

func TestStruct_ValidDTO(t *testing.T) {
	assert.NoError(t, newValidator(t).Struct(demoDTO{Name: "Ana", Amount: 100}))
}

func TestStruct_UsesPublicFieldNamesAndSpanishMessages(t *testing.T) {
	err := newValidator(t).Struct(demoDTO{Code: "x", Hidden: "abc"})

	assert.Equal(t, map[string]string{
		"nombre":         "es obligatorio",
		"monto_centavos": "debe ser al menos 1",
		"codigo":         "no es válido",
		"Hidden":         "debe ser como máximo 2",
	}, fieldsOf(t, err))
}

func TestStruct_NonStructIsBadRequest(t *testing.T) {
	err := newValidator(t).Struct(42)

	var appErr *apperr.Error
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, apperr.KindBadRequest, appErr.Kind)
}

func TestPaginationQuery_DefaultsAndPosition(t *testing.T) {
	q := NewPaginationQuery()
	require.NoError(t, newValidator(t).Struct(q))
	assert.Equal(t, 20, q.Limit)

	_, ok, err := q.Position()
	require.NoError(t, err)
	assert.False(t, ok)
}

type passwordDTO struct {
	Password string `json:"password" validate:"password"`
	Email    string `json:"correo" validate:"omitempty,email"`
}

func TestPasswordRule_EightCharsWithLetterAndNumber(t *testing.T) {
	v := newValidator(t)
	for _, ok := range []string{"abcdefg1", "Contraseña9", "1234567a"} {
		assert.NoError(t, v.Struct(passwordDTO{Password: ok}), ok)
	}
	for _, bad := range []string{"", "abc1", "abcdefgh", "12345678", strings.Repeat("a1", 65)} {
		fields := fieldsOf(t, v.Struct(passwordDTO{Password: bad}))
		assert.Contains(t, fields["password"], "una letra y un número", bad)
	}
}

func TestEmailRule_HasSpanishMessage(t *testing.T) {
	fields := fieldsOf(t, newValidator(t).Struct(passwordDTO{Password: "abcdefg1", Email: "no-es-correo"}))

	assert.Equal(t, map[string]string{"correo": "no es un correo válido"}, fields)
}

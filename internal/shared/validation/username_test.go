package validation

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Unit/NombreUsuario (nv-34): formato, longitud, mayúsculas y reservados.
type usernameDTO struct {
	Username string `json:"nombre_usuario" validate:"omitempty,username"`
}

func usernameErr(t *testing.T, name string) string {
	t.Helper()
	return fieldsOf(t, newValidator(t).Struct(usernameDTO{Username: name}))["nombre_usuario"]
}

func TestUsername_AcceptsLowercaseDigitsUnderscore(t *testing.T) {
	v := newValidator(t)
	for _, ok := range []string{"ana", "ana_23", "_x_", "a1b2c3", "taquero_2026"} {
		assert.NoError(t, v.Struct(usernameDTO{Username: ok}), ok)
	}
}

func TestUsername_RejectsOtherCharacters(t *testing.T) {
	for _, bad := range []string{"ana.lopez", "ana-lopez", "ana lopez", "año_23", "ana@x", "ana!"} {
		assert.Contains(t, usernameErr(t, bad), "guion bajo", bad)
	}
}

func TestUsername_LengthThreeToTwenty(t *testing.T) {
	v := newValidator(t)
	assert.NoError(t, v.Struct(usernameDTO{Username: strings.Repeat("a", 3)}))
	assert.NoError(t, v.Struct(usernameDTO{Username: strings.Repeat("a", 20)}))
	assert.NotEmpty(t, usernameErr(t, "ab"))
	assert.NotEmpty(t, usernameErr(t, strings.Repeat("a", 21)))
}

// Solo minúsculas: el índice único sobre el valor guardado ya es único sin importar mayúsculas.
func TestUsername_RejectsUppercase(t *testing.T) {
	for _, bad := range []string{"Ana", "ANA_23", "anA"} {
		assert.Contains(t, usernameErr(t, bad), "minúsculas", bad)
	}
}

func TestUsername_RejectsReserved(t *testing.T) {
	for _, bad := range []string{"admin", "administrador", "wherever", "soporte", "moderador"} {
		assert.Contains(t, usernameErr(t, bad), "reservado", bad)
	}
}

func TestUsername_ReservedOnlyExactMatch(t *testing.T) {
	v := newValidator(t)
	for _, ok := range []string{"admin_ana", "wherever_fan", "soporte1"} {
		assert.NoError(t, v.Struct(usernameDTO{Username: ok}), ok)
	}
}

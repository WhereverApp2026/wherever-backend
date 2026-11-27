package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

// nv-91: el código es de exactamente 6 dígitos; lo demás es 422 antes de gastar un intento.
func TestVerifyEmailRequest_Codigo(t *testing.T) {
	v, err := validation.New()
	require.NoError(t, err)
	for _, ok := range []string{"000000", "123456", "999999"} {
		assert.NoError(t, v.Struct(VerifyEmailRequest{Code: ok}), ok)
	}
	for _, bad := range []string{"", "12345", "1234567", "12a456", "+12345", "-12345", "12 456", "１２３４５６", "1.2345"} {
		var appErr *apperr.Error
		require.ErrorAs(t, v.Struct(VerifyEmailRequest{Code: bad}), &appErr, bad)
		assert.Equal(t, apperr.KindValidation, appErr.Kind, bad)
	}
}

// nv-94: restablecer pide correo, código de 6 dígitos y contraseña nueva; sin alguno es 422.
func TestResetRequest_CorreoYCodigo(t *testing.T) {
	v, err := validation.New()
	require.NoError(t, err)
	ok := ResetRequest{Email: "ana@correo.com", Code: "042917", NewPassword: "NuevaClave9"}
	require.NoError(t, v.Struct(ok))
	for name, bad := range map[string]ResetRequest{
		"sin correo":       {Code: "042917", NewPassword: "NuevaClave9"},
		"correo inválido":  {Email: "ana", Code: "042917", NewPassword: "NuevaClave9"},
		"sin código":       {Email: "ana@correo.com", NewPassword: "NuevaClave9"},
		"código corto":     {Email: "ana@correo.com", Code: "04291", NewPassword: "NuevaClave9"},
		"código con letra": {Email: "ana@correo.com", Code: "04291a", NewPassword: "NuevaClave9"},
		"contraseña débil": {Email: "ana@correo.com", Code: "042917", NewPassword: "corta"},
	} {
		var appErr *apperr.Error
		require.ErrorAs(t, v.Struct(bad), &appErr, name)
		assert.Equal(t, apperr.KindValidation, appErr.Kind, name)
	}
}

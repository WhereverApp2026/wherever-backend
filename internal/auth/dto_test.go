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

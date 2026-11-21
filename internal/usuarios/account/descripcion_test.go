package account

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

func TestLimpiarDescripcion_UnaLineaSinControles(t *testing.T) {
	for raw, want := range map[string]string{
		"  Me gustan los tacos  ":           "Me gustan los tacos",
		"Línea uno\nLínea dos\r\n\tfin":       "Línea uno Línea dos fin",
		"oculto\u200btexto\u202e":            "oculto texto",
		"\x00\x07":                            "",
		"Café ☕ y mezcal 🌵, siempre en Tula": "Café ☕ y mezcal 🌵, siempre en Tula",
	} {
		assert.Equal(t, want, LimpiarDescripcion(raw), "%q", raw)
	}
}

func TestDescripcion_Validacion(t *testing.T) {
	v, err := validation.New()
	require.NoError(t, err)
	ok := strings.Repeat("ñ", 160)
	assert.NoError(t, v.Struct(PatchProfileRequest{Description: &ok}), "160 caracteres con acentos caben")
	vacia := ""
	assert.NoError(t, v.Struct(PatchProfileRequest{Description: &vacia}), "vacía la borra")
	larga := strings.Repeat("a", 161)
	var appErr *apperr.Error
	require.ErrorAs(t, v.Struct(PatchProfileRequest{Description: &larga}), &appErr)
	assert.Equal(t, apperr.KindValidation, appErr.Kind)
}

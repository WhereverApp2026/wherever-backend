package account

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

func dtoFactories() []func() any {
	return []func() any{
		func() any { return &RegisterRequest{} },
		func() any { return &PatchProfileRequest{} },
		func() any { return &ChangeEmailRequest{} },
		func() any { return &TastesRequest{} },
		func() any { return &PreferencesRequest{} },
		func() any { return &PrivacyRequest{} },
		func() any { return &PhoneRequest{} },
		func() any { return &ModeRequest{} },
		func() any { return &DeleteRequest{} },
		func() any { return &DownloadExportRequest{} },
	}
}

// FuzzDTOs: cualquier JSON que entre a un DTO se valida sin pánico y con un error de contrato.
func FuzzDTOs(f *testing.F) {
	f.Add(uint8(0), []byte(`{"nombre":"Ana","fecha_nacimiento":"1999-04-12","correo":"a@b.co","password":"Wherever2026","acepta_terminos":true,"version_terminos":"1"}`))
	f.Add(uint8(0), []byte(`{"fecha_nacimiento":"9999-99-99"}`))
	f.Add(uint8(1), []byte(`{"foto_url":"javascript:alert(1)","ciudad_id":"x"}`))
	f.Add(uint8(3), []byte(`{"categorias":["a","a","a"],"presupuesto_min":-1,"presupuesto_max":-2}`))
	f.Add(uint8(3), []byte(`{"categorias":null,"presupuesto_min":1e300}`))
	f.Add(uint8(4), []byte(`{"horario_silencioso":{"desde":"25:99","hasta":""},"notificaciones":{"reservas":false}}`))
	f.Add(uint8(6), []byte(`{"telefono":"+0"}`))
	f.Add(uint8(7), []byte(`{"modo":"root"}`))

	val, err := validation.New()
	require.NoError(f, err)
	dtos := dtoFactories()
	f.Fuzz(func(t *testing.T, which uint8, body []byte) {
		dto := dtos[int(which)%len(dtos)]()
		if json.Unmarshal(body, dto) != nil {
			return
		}
		verr := val.Struct(dto)
		if verr == nil {
			return
		}
		var appErr *apperr.Error
		require.True(t, errors.As(verr, &appErr), "error fuera de contrato: %v", verr)
		assert.Equal(t, apperr.KindValidation, appErr.Kind)
	})
}

// FuzzNormalizePhone: normalizar dos veces da lo mismo y no quedan separadores.
func FuzzNormalizePhone(f *testing.F) {
	for _, seed := range []string{"+52 (55) 1234-5678", "", "   ", "+", "١٢٣", "\xff", "+52.55.1234"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		out := NormalizePhone(raw)
		assert.Equal(t, out, NormalizePhone(out))
		hasSeparator := strings.ContainsFunc(out, func(r rune) bool { return unicode.IsSpace(r) || strings.ContainsRune("-.()", r) })
		assert.False(t, hasSeparator, "quedó un separador: %q", out)
	})
}

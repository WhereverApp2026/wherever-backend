package ciudades

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

func TestAutocomplete_OneLetterIs422WithoutTouchingTheDatabase(t *testing.T) {
	_, err := NewService(nil).Autocomplete(context.Background(), " É ", DefaultLimit)

	var appErr *apperr.Error
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, apperr.KindValidation, appErr.Kind)
	assert.Equal(t, "q", appErr.Fields[0].Field)
}

func TestParseCSV_ReadsHeaderAndRows(t *testing.T) {
	rows, err := parseRows(strings.NewReader("nombre,estado,latitud,longitud,con_cobertura\nTula de Allende,Hidalgo,20.05,-99.34,true\n"), "csv")

	require.NoError(t, err)
	assert.Equal(t, []ImportRow{{Name: "Tula de Allende", State: "Hidalgo", Latitude: 20.05, Longitude: -99.34, Covered: true}}, rows)
}

func TestParseRows_RejectsBadInput(t *testing.T) {
	for name, tc := range map[string]struct{ body, format string }{
		"encabezado distinto": {"ciudad,estado,lat,lon,cob\n", "csv"},
		"latitud no numérica": {"nombre,estado,latitud,longitud,con_cobertura\nA,B,norte,1,true\n", "csv"},
		"formato desconocido": {"", "xml"},
		"json roto":           {"[{", "json"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := parseRows(strings.NewReader(tc.body), tc.format)
			assert.Error(t, err)
		})
	}
}

func TestValidateRow(t *testing.T) {
	assert.NoError(t, validateRow(ImportRow{Name: "Guadalajara", State: "Jalisco", Latitude: 20.67, Longitude: -103.35}))
	assert.Error(t, validateRow(ImportRow{Name: " ", State: "Jalisco"}))
	assert.Error(t, validateRow(ImportRow{Name: "X", State: "Y", Latitude: 91}))
}

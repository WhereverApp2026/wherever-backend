package ciudades

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// FuzzParseRows: un archivo de catálogo roto da error, nunca pánico, y lo aceptado es válido o se marca.
func FuzzParseRows(f *testing.F) {
	csvOK := "nombre,estado,latitud,longitud,con_cobertura\nGuadalajara,Jalisco,20.6,-103.3,true\n"
	f.Add(csvOK, "csv")
	f.Add(`[{"nombre":"Guadalajara","estado":"Jalisco","latitud":20.6,"longitud":-103.3,"con_cobertura":true}]`, "json")
	f.Add("nombre,estado\nx\n", "csv")
	f.Add("nombre,estado,latitud,longitud,con_cobertura\na,b,NaN,Inf,maybe\n", "csv")
	f.Add("nombre,estado,latitud,longitud,con_cobertura\n\"a,b,1,2,true\n", "csv")
	f.Add("{", "json")
	f.Add("null", "json")
	f.Add("", "xml")
	f.Fuzz(func(t *testing.T, content, format string) {
		rows, err := parseRows(strings.NewReader(content), format)
		if err != nil {
			assert.Empty(t, rows)
		}
		for _, row := range rows {
			_ = validateRow(row)
		}
	})
}

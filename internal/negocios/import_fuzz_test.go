package negocios

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// FuzzParseCSV: cualquier entrada produce un error de archivo o filas válidas; nunca un
// pánico. Las filas que sobreviven a validateRows cumplen validateRow.
func FuzzParseCSV(f *testing.F) {
	for _, seed := range []string{
		"",
		"slug,nombre\n",
		strings.Join(CSVHeader, ",") + "\n",
		csvBody(validRowValues()),
		"\xEF\xBB\xBF" + csvBody(validRowValues()),
		strings.Join(CSVHeader, ",") + "\n" + strings.Repeat("a", 1000) + "\n",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		rows, _, err := Parse(strings.NewReader(input), "csv")
		if err != nil {
			return
		}
		valid, _ := validateRows(rows)
		for _, item := range valid {
			assert.Empty(t, validateRow(item.row))
		}
	})
}

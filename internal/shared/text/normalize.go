// Package text normaliza texto para búsqueda (constitución §5: campos `*_normalizado` en
// minúsculas y sin acentos).
package text

import (
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// Normalize deja el texto en minúsculas, sin acentos y con espacios simples: "  Tula de
// Allende " → "tula de allende", "Ñuñoa" → "nunoa".
func Normalize(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	out, _, err := transform.String(t, s)
	if err != nil {
		out = s
	}
	return strings.Join(strings.Fields(strings.ToLower(out)), " ")
}

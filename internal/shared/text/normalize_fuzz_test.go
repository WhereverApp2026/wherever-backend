package text

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// FuzzNormalize: cualquier texto queda sin espacios de más; la búsqueda de ciudades depende de esto.
func FuzzNormalize(f *testing.F) {
	for _, seed := range []string{"  Tula de  Allende ", "Ñuñoa", "", "İstanbul", "\xff\xfe", "a\x00b", "́", "\t\n\r ", strings.Repeat("é", 1000)} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		out := Normalize(s)
		assert.Equal(t, strings.TrimSpace(out), out)
		assert.NotContains(t, out, "  ")
		assert.Equal(t, strings.Join(strings.Fields(out), " "), out)
	})
}

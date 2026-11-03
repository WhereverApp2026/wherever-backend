package text

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalize_LowercaseWithoutAccentsAndSpaces(t *testing.T) {
	for in, want := range map[string]string{
		"Tula de Allende":       "tula de allende",
		"  MÉRIDA  ":            "merida",
		"Ñuñoa":                 "nunoa",
		"São   Paulo":           "sao paulo",
		"Zihuatanejo de Azueta": "zihuatanejo de azueta",
		"":                      "",
	} {
		assert.Equal(t, want, Normalize(in), in)
	}
}

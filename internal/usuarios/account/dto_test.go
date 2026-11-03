package account

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizePhone_ToE164(t *testing.T) {
	for in, want := range map[string]string{
		"+52 (55) 1234-5678": "+525512345678",
		" +52.55.1234.5678 ": "+525512345678",
		"+5215512345678":     "+5215512345678",
		"55 1234 5678":       "5512345678",
	} {
		assert.Equal(t, want, NormalizePhone(in), in)
	}
}

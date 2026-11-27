package auth

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

var sixDigits = regexp.MustCompile(`^[0-9]{6}$`)

func TestNewCode_SixDigits(t *testing.T) {
	seen := map[string]bool{}
	for range 200 {
		code, err := NewCode()
		require.NoError(t, err)
		assert.Regexp(t, sixDigits, code)
		seen[code] = true
	}
	assert.Greater(t, len(seen), 190, "los códigos no se repiten seguido")
}

func TestCodeHasher_NeverStoresTheCode(t *testing.T) {
	h := NewCodeHasher([]byte("llave-de-prueba-de-32-bytes-0000"))
	id := bson.NewObjectID()
	hash := h.Hash(id, "123456")
	assert.Len(t, hash, 64)
	assert.NotContains(t, hash, "123456")
	assert.True(t, h.Matches(id, "123456", hash))
	assert.False(t, h.Matches(id, "123457", hash))
	assert.False(t, h.Matches(bson.NewObjectID(), "123456", hash), "el hash es del documento")
	assert.NotEqual(t, hash, h.Hash(bson.NewObjectID(), "123456"), "mismos dígitos, otro documento: sin choque en ux_token_hash")
	other := NewCodeHasher([]byte("otra-llave-de-prueba-de-32-bytes"))
	assert.False(t, other.Matches(id, "123456", hash), "sin la llave no se puede comprobar")
	assert.NotEqual(t, HashOpaque("123456"), hash, "no es un SHA-256 simple")
}

// nv-94: restablecer tiene un tope diario más bajo que la verificación.
func TestCodesPerDay(t *testing.T) {
	assert.Equal(t, int64(MaxResetCodesPerDay), codesPerDay("recuperar_password"))
	assert.Equal(t, int64(MaxCodesPerDay), codesPerDay("verificar_correo"))
}

package auth

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

func newHasher(t *testing.T) *Hasher {
	t.Helper()
	h, err := NewHasher()
	require.NoError(t, err)
	return h
}

func TestHasher_VerifiesArgon2id(t *testing.T) {
	h := newHasher(t)
	hash, err := h.Hash("Secreta123")
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(hash, "$argon2id$v=19$m=65536,t=3,p=2$"))
	assert.True(t, h.Verify("Secreta123", hash))
	assert.False(t, h.Verify("Secreta124", hash))
	assert.False(t, h.Verify("Secreta123", "$argon2id$roto"))
	assert.False(t, h.Verify("Secreta123", ""))
}

func TestHasher_SaltMakesEachHashDifferent(t *testing.T) {
	h := newHasher(t)
	a, err := h.Hash("Secreta123")
	require.NoError(t, err)
	b, err := h.Hash("Secreta123")
	require.NoError(t, err)

	assert.NotEqual(t, a, b)
}

func TestPasswordPolicy_EightCharsWithLetterAndNumber(t *testing.T) {
	v, err := validation.New()
	require.NoError(t, err)
	valid := ChangePasswordRequest{CurrentPassword: "x", NewPassword: "abcdefg1"}
	assert.NoError(t, v.Struct(valid))

	for _, weak := range []string{"abc1", "abcdefgh", "12345678"} {
		assert.Error(t, v.Struct(ChangePasswordRequest{CurrentPassword: "x", NewPassword: weak}), weak)
	}
}

// El hash falso usa los mismos parámetros que uno real: verificar contra él cuesta lo mismo.
func TestHasher_DummyHasSameCostAsReal(t *testing.T) {
	h := newHasher(t)
	real, err := h.Hash("Secreta123")
	require.NoError(t, err)

	params := func(encoded string) string { return strings.Join(strings.Split(encoded, "$")[1:4], "$") }

	assert.Equal(t, params(real), params(h.dummy))
	assert.False(t, h.Verify("Secreta123", h.dummy))
}

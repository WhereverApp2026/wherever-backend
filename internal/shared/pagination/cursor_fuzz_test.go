package pagination

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FuzzDecodeCursor: un cursor del cliente nunca provoca pánico, y uno aceptado es reproducible.
func FuzzDecodeCursor(f *testing.F) {
	s, err := NewSigner([]byte(testKey))
	require.NoError(f, err)
	seed, err := s.Encode(Cursor{ID: "6728f0c1a4b2c3d4e5f60718", Sort: "2026-11-03", Scope: "fuzz"})
	require.NoError(f, err)
	f.Add(seed)
	for _, raw := range []string{"", "!!!", "e30", "bnVsbA", "W10.", ".", "eyJfaWQiOjF9.AAAA", "eyJfaWQiOiJ4In0"} {
		f.Add(raw)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		valid := Valid(raw)
		cursor, ok, err := s.Decode("fuzz", raw)
		if err != nil {
			assert.ErrorIs(t, err, ErrInvalidCursor)
			assert.False(t, ok)
			return
		}
		if !ok {
			assert.Empty(t, raw, "solo el cursor vacío es la primera página")
			return
		}
		assert.True(t, valid, "un cursor aceptado tiene forma válida")
		token, err := s.Encode(cursor)
		require.NoError(t, err)
		again, ok, err := s.Decode("fuzz", token)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, cursor, again)
	})
}

package pagination

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FuzzDecodeCursor: un cursor del cliente nunca provoca pánico, y uno aceptado es reproducible.
func FuzzDecodeCursor(f *testing.F) {
	f.Add(Encode(Cursor{ID: "6728f0c1a4b2c3d4e5f60718", Sort: "2026-11-03"}))
	for _, seed := range []string{"", "!!!", "e30", "bnVsbA", "W10", "eyJfaWQiOjF9", "eyJfaWQiOiJ4In0"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		cursor, ok, err := Decode(raw)
		if err != nil {
			assert.ErrorIs(t, err, ErrInvalidCursor)
			assert.False(t, ok)
			return
		}
		if !ok {
			assert.Empty(t, raw, "solo el cursor vacío es la primera página")
			return
		}
		again, ok, err := Decode(Encode(cursor))
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, cursor, again)
	})
}

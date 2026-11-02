package pagination

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sampleID = "65a1f0c2e4b0a1b2c3d4e5f6"

func TestCursor_RoundTrip(t *testing.T) {
	in := Cursor{ID: sampleID, Sort: "2026-11-02T10:00:00Z"}

	out, ok, err := Decode(Encode(in))

	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, in, out)
}

func TestCursor_EmptyIsFirstPage(t *testing.T) {
	c, ok, err := Decode("")

	require.NoError(t, err)
	assert.False(t, ok)
	assert.Equal(t, Cursor{}, c)
}

func TestCursor_TamperedIsInvalid(t *testing.T) {
	enc := base64.RawURLEncoding.EncodeToString
	cases := map[string]string{
		"no es base64":       "%%%",
		"base64 con relleno": base64.URLEncoding.EncodeToString([]byte(`{"_id":"` + sampleID + `","orden":"a"}`)),
		"json roto":          enc([]byte(`{"_id":`)),
		"campo extra":        enc([]byte(`{"_id":"` + sampleID + `","orden":"a","rol":"administrador"}`)),
		"id no es ObjectId":  enc([]byte(`{"_id":"abc","orden":"a"}`)),
		"sin id":             enc([]byte(`{"orden":"a"}`)),
		"basura al final":    enc([]byte(`{"_id":"` + sampleID + `","orden":"a"}{}`)),
		"valor no objeto":    enc([]byte(`"texto"`)),
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			_, ok, err := Decode(raw)
			require.ErrorIs(t, err, ErrInvalidCursor)
			assert.False(t, ok)
		})
	}
}

// Dos elementos con el mismo valor de orden se distinguen por _id: el cursor es
// determinista y conserva el desempate, así la página siguiente no repite ni salta.
func TestCursor_StableOrdering(t *testing.T) {
	a := Cursor{ID: "65a1f0c2e4b0a1b2c3d4e5f6", Sort: "mismo"}
	b := Cursor{ID: "65a1f0c2e4b0a1b2c3d4e5f7", Sort: "mismo"}

	assert.Equal(t, Encode(a), Encode(a))
	assert.NotEqual(t, Encode(a), Encode(b))

	gotA, _, err := Decode(Encode(a))
	require.NoError(t, err)
	gotB, _, err := Decode(Encode(b))
	require.NoError(t, err)
	assert.Equal(t, a.Sort, gotB.Sort)
	assert.Less(t, gotA.ID, gotB.ID)
}

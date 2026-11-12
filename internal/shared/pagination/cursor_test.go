package pagination

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	sampleID = "65a1f0c2e4b0a1b2c3d4e5f6"
	testKey  = "solo_para_pruebas_cursor_no_usar_en_entornos_0789"
	otherKey = "otra_llave_de_pruebas_para_cursores_no_usar_0790"
)

func signer(t testing.TB, key string) *Signer {
	t.Helper()
	s, err := NewSigner([]byte(key))
	require.NoError(t, err)
	return s
}

func sample() Cursor {
	return Cursor{ID: sampleID, Sort: "2026-11-02T10:00:00Z", Scope: "lugares:abc"}
}

func encode(t testing.TB, s *Signer, c Cursor) string {
	t.Helper()
	raw, err := s.Encode(c)
	require.NoError(t, err)
	return raw
}

// signed arma un token con la firma correcta sobre un JSON arbitrario.
func signed(s *Signer, payload string) string {
	enc := base64.RawURLEncoding.EncodeToString
	return enc([]byte(payload)) + "." + enc(s.mac([]byte(payload)))
}

func TestCursor_RoundTrip(t *testing.T) {
	s := signer(t, testKey)

	out, ok, err := s.Decode("lugares:abc", encode(t, s, sample()))

	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, sample(), out)
}

func TestCursor_EmptyIsFirstPage(t *testing.T) {
	c, ok, err := signer(t, testKey).Decode("lugares:abc", "")

	require.NoError(t, err)
	assert.False(t, ok)
	assert.Equal(t, Cursor{}, c)
}

func TestCursor_RejectedTokens(t *testing.T) {
	s := signer(t, testKey)
	good := encode(t, s, sample())
	payload, sum, _ := strings.Cut(good, ".")
	tamperedPayload := base64.RawURLEncoding.EncodeToString(
		[]byte(`{"_id":"` + sampleID + `","orden":"z","ambito":"lugares:abc"}`))
	flipped := []byte(sum)
	flipped[0] ^= 1
	if flipped[0] == '.' || flipped[0] == '=' {
		flipped[0] = 'A'
	}
	enc := base64.RawURLEncoding.EncodeToString
	cases := map[string]string{
		"carga alterada":     tamperedPayload + "." + sum,
		"MAC alterado":       payload + "." + string(flipped),
		"otra llave":         encode(t, signer(t, otherKey), sample()),
		"no es base64":       "%%%.%%%",
		"base64 con relleno": base64.URLEncoding.EncodeToString([]byte(`{"_id":"`+sampleID+`","orden":"a","ambito":"x"}`)) + "." + sum,
		"sin firma":          payload,
		"json roto":          signed(s, `{"_id":`),
		"campo extra":        signed(s, `{"_id":"`+sampleID+`","orden":"a","ambito":"lugares:abc","rol":"administrador"}`),
		"id no es ObjectId":  signed(s, `{"_id":"abc","orden":"a","ambito":"lugares:abc"}`),
		"basura al final":    signed(s, `{"_id":"`+sampleID+`","orden":"a","ambito":"lugares:abc"}{}`),
		"firma corta":        payload + "." + enc([]byte("corta")),
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			_, ok, err := s.Decode("lugares:abc", raw)
			require.ErrorIs(t, err, ErrInvalidCursor)
			assert.False(t, ok)
		})
	}
}

func TestCursor_OtherScopeIsInvalid(t *testing.T) {
	s := signer(t, testKey)

	_, ok, err := s.Decode("planes:xyz", encode(t, s, sample()))

	require.ErrorIs(t, err, ErrInvalidCursor)
	assert.False(t, ok)
}

func TestNewSigner_RejectsShortKey(t *testing.T) {
	_, err := NewSigner([]byte(strings.Repeat("a", MinKeyBytes-1)))
	require.Error(t, err)
}

func TestSigner_EncodeRequiresScopeAndObjectID(t *testing.T) {
	s := signer(t, testKey)
	_, err := s.Encode(Cursor{ID: sampleID, Sort: "a"})
	require.ErrorIs(t, err, ErrInvalidCursor)
	_, err = s.Encode(Cursor{ID: "abc", Sort: "a", Scope: "x"})
	require.ErrorIs(t, err, ErrInvalidCursor)
}

func TestSigner_NilFailsClosed(t *testing.T) {
	var s *Signer
	_, err := s.Encode(sample())
	require.Error(t, err)
	_, ok, err := s.Decode("lugares:abc", "x.y")
	require.ErrorIs(t, err, ErrInvalidCursor)
	assert.False(t, ok)
}

// El ámbito depende de la llave y de cada parte; no deja ver las partes que lo forman.
func TestSigner_Scope(t *testing.T) {
	s := signer(t, testKey)
	a := s.Scope("lugares", "calificacion", "19.43", "-99.13")

	assert.Equal(t, a, s.Scope("lugares", "calificacion", "19.43", "-99.13"))
	assert.NotEqual(t, a, s.Scope("lugares", "calificacion", "19.44", "-99.13"))
	assert.NotEqual(t, a, signer(t, otherKey).Scope("lugares", "calificacion", "19.43", "-99.13"))
	assert.True(t, strings.HasPrefix(a, "lugares:"))
	assert.NotContains(t, a, "19.43")
}

func TestValid_ChecksShapeOnly(t *testing.T) {
	s := signer(t, testKey)
	assert.True(t, Valid(""))
	assert.True(t, Valid(encode(t, s, sample())))
	assert.True(t, Valid(encode(t, signer(t, otherKey), sample())), "la firma la verifica Decode")
	for _, raw := range []string{"no-es-un-cursor", "%%%.%%%", signed(s, `{"_id":"abc","orden":"a","ambito":"x"}`)} {
		assert.False(t, Valid(raw), raw)
	}
}

// Dos elementos con el mismo valor de orden se distinguen por _id: el cursor es
// determinista y conserva el desempate, así la página siguiente no repite ni salta.
func TestCursor_StableOrdering(t *testing.T) {
	s := signer(t, testKey)
	a := Cursor{ID: "65a1f0c2e4b0a1b2c3d4e5f6", Sort: "mismo", Scope: "x"}
	b := Cursor{ID: "65a1f0c2e4b0a1b2c3d4e5f7", Sort: "mismo", Scope: "x"}

	assert.Equal(t, encode(t, s, a), encode(t, s, a))
	assert.NotEqual(t, encode(t, s, a), encode(t, s, b))

	gotA, _, err := s.Decode("x", encode(t, s, a))
	require.NoError(t, err)
	gotB, _, err := s.Decode("x", encode(t, s, b))
	require.NoError(t, err)
	assert.Equal(t, a.Sort, gotB.Sort)
	assert.Less(t, gotA.ID, gotB.ID)
}

package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	fuzzKey = []byte("llave-de-prueba-del-fuzz-con-32-bytes")
	fuzzCfg = JWTConfig{Key: fuzzKey, Issuer: "wherever-api", Audience: "wherever-app"}
)

func fuzzClaims() jwt.MapClaims {
	now := time.Now()
	return jwt.MapClaims{
		"sub": "6728f0c1a4b2c3d4e5f60718", "jti": "jti", "iss": fuzzCfg.Issuer, "aud": fuzzCfg.Audience,
		"iat": now.Add(-time.Minute).Unix(), "exp": now.Add(10 * 365 * 24 * time.Hour).Unix(),
	}
}

func signRaw(header, payload []byte) string {
	input := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, fuzzKey)
	mac.Write([]byte(input))
	return input + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// assertSigned comprueba por su cuenta que un token aceptado sí lleva la firma de la llave.
func assertSigned(t *testing.T, raw string) {
	t.Helper()
	parts := strings.Split(raw, ".")
	require.Len(t, parts, 3, "un token aceptado tiene tres segmentos")
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	require.NoError(t, err)
	mac := hmac.New(sha256.New, fuzzKey)
	mac.Write([]byte(parts[0] + "." + parts[1]))
	assert.True(t, hmac.Equal(sig, mac.Sum(nil)), "se aceptó una firma incorrecta")
	var header struct {
		Alg string `json:"alg"`
	}
	headerRaw, err := base64.RawURLEncoding.DecodeString(parts[0])
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(headerRaw, &header))
	assert.Equal(t, "HS256", header.Alg)
}

// FuzzParseToken: tokens malformados, alg none o firma alterada nunca se aceptan.
func FuzzParseToken(f *testing.F) {
	good, err := jwt.NewWithClaims(jwt.SigningMethodHS256, fuzzClaims()).SignedString(fuzzKey)
	require.NoError(f, err)
	none, err := jwt.NewWithClaims(jwt.SigningMethodNone, fuzzClaims()).SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(f, err)
	other, err := jwt.NewWithClaims(jwt.SigningMethodHS256, fuzzClaims()).SignedString([]byte("otra-llave-de-prueba-con-32-bytes"))
	require.NoError(f, err)
	hs512, err := jwt.NewWithClaims(jwt.SigningMethodHS512, fuzzClaims()).SignedString(fuzzKey)
	require.NoError(f, err)
	for _, seed := range []string{good, none, other, hs512, good + "x", good[:len(good)-1], "", ".", "..", "a.b.c", strings.Repeat("a.", 5000)} {
		f.Add(seed)
	}
	parser := NewAccessParser(fuzzCfg)
	f.Fuzz(func(t *testing.T, raw string) {
		claims, err := parser.Parse(raw)
		if err != nil {
			return
		}
		assertSigned(t, raw)
		assert.NotEmpty(t, claims.Subject)
		assert.NotEmpty(t, claims.ID)
	})
}

// FuzzTokenClaims: con firma válida, claims inesperados tampoco pasan.
func FuzzTokenClaims(f *testing.F) {
	good, err := json.Marshal(fuzzClaims())
	require.NoError(f, err)
	f.Add(good)
	for _, seed := range []string{
		`{}`, `null`, `[]`, `1`, `{"sub":"","jti":""}`,
		`{"sub":"a","jti":"b","iss":"otro","aud":"wherever-app","exp":9999999999,"iat":1}`,
		`{"sub":"a","jti":"b","iss":"wherever-api","aud":"otra","exp":9999999999,"iat":1}`,
		`{"sub":"a","jti":"b","iss":"wherever-api","aud":"wherever-app","exp":1,"iat":1}`,
		`{"sub":"a","jti":"b","iss":"wherever-api","aud":"wherever-app"}`,
		`{"sub":1,"jti":{},"iss":[],"aud":true,"exp":"x"}`,
		`{"sub":"a","jti":"b","iss":"wherever-api","aud":"wherever-app","exp":1e400,"iat":-1e400}`,
	} {
		f.Add([]byte(seed))
	}
	parser := NewAccessParser(fuzzCfg)
	header := []byte(`{"alg":"HS256","typ":"JWT"}`)
	f.Fuzz(func(t *testing.T, payload []byte) {
		claims, err := parser.Parse(signRaw(header, payload))
		if err != nil {
			return
		}
		assert.NotEmpty(t, claims.Subject)
		assert.NotEmpty(t, claims.ID)
		assert.Equal(t, fuzzCfg.Issuer, claims.Issuer)
		assert.Contains(t, []string(claims.Audience), fuzzCfg.Audience)
		require.NotNil(t, claims.ExpiresAt)
		assert.True(t, claims.ExpiresAt.After(time.Now()), "se aceptó un token vencido")
	})
}

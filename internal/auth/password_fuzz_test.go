package auth

import (
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/argon2"
)

// lowHash arma un hash válido con parámetros bajos: no se fuzzea el costo de Argon2id.
func lowHash(password string) string {
	salt := []byte("salsalsalsalsal1")
	key := argon2.IDKey([]byte(password), salt, 1, 16, 1, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=16,t=1,p=1$%s$%s", argon2.Version,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key))
}

// FuzzParseHash: un hash corrupto nunca provoca pánico ni agota la memoria.
func FuzzParseHash(f *testing.F) {
	valid := lowHash("Wherever2026")
	f.Add("Wherever2026", valid)
	f.Add("x", "$argon2id$v=19$m=8,t=0,p=1$c2Fsc2Fsc2Fs$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
	f.Add("x", "$argon2id$v=19$m=8,t=1,p=0$c2Fsc2Fsc2Fs$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
	f.Add("x", "$argon2id$v=19$m=4294967295,t=1,p=1$c2Fsc2Fsc2Fs$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
	f.Add("x", "$argon2id$v=19$m=16,t=1,p=1basura$c2Fsc2Fsc2Fs$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
	f.Add("x", "$argon2i$v=19$m=16,t=1,p=1$c2Fsc2Fsc2Fs$AAAA")
	f.Add("", "")
	f.Add("x", "$$$$$")

	h := &Hasher{}
	f.Fuzz(func(t *testing.T, password, encoded string) {
		parsed, err := parseHash(encoded)
		if err != nil {
			assert.False(t, h.Verify(password, encoded), "un hash inválido nunca verifica")
			return
		}
		assert.True(t, parsed.safeParams())
		assert.Len(t, parsed.key, argonKeyLen)
		// Solo se calcula con parámetros bajos: los topes ya están comprobados arriba.
		if parsed.memory <= 1024 && parsed.iterations <= 2 {
			got := h.Verify(password, encoded)
			if encoded == valid {
				assert.Equal(t, password == "Wherever2026", got)
			}
		}
	})
}

func TestParseHash_RejectsUnsafeParams(t *testing.T) {
	tail := "$c2Fsc2Fsc2Fs$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	for name, params := range map[string]string{
		"sin iteraciones":  "m=16,t=0,p=1",
		"sin hilos":        "m=16,t=1,p=0",
		"memoria enorme":   "m=4294967295,t=1,p=1",
		"sobre el tope":    "m=262145,t=1,p=1",
		"muchas vueltas":   "m=16,t=11,p=1",
		"muchos hilos":     "m=1024,t=1,p=17",
		"memoria baja":     "m=7,t=1,p=1",
		"basura al final":  "m=16,t=1,p=1x",
		"ceros a la izq.":  "m=016,t=1,p=1",
		"espacio al final": "m=16,t=1,p=1 ",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := parseHash("$argon2id$v=19$" + params + tail)
			assert.ErrorIs(t, err, errBadHash)
		})
	}
}

func TestParseHash_AcceptsCurrentParams(t *testing.T) {
	h := &Hasher{}
	encoded, err := h.Hash("Wherever2026")
	assert.NoError(t, err)
	_, err = parseHash(encoded)
	assert.NoError(t, err, "los hashes que ya existen deben seguir verificando")
}

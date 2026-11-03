package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Parámetros de Argon2id (AGENTS.md §3: 64 MB y 3 iteraciones).
const (
	argonMemoryKiB = 64 * 1024
	argonTime      = 3
	argonThreads   = 2
	argonKeyLen    = 32
	argonSaltLen   = 16
)

var errBadHash = errors.New("hash de contraseña con formato inválido")

// Hasher calcula y verifica contraseñas con Argon2id en formato PHC.
type Hasher struct {
	dummy string
}

// NewHasher prepara un hash falso con los mismos parámetros: con un correo inexistente
// se verifica contra él para que la respuesta tarde lo mismo (anti-enumeración).
func NewHasher() (*Hasher, error) {
	secret := make([]byte, argonKeyLen)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	h := &Hasher{}
	dummy, err := h.Hash(base64.RawStdEncoding.EncodeToString(secret))
	if err != nil {
		return nil, err
	}
	h.dummy = dummy
	return h, nil
}

// Hash devuelve `$argon2id$v=19$m=65536,t=3,p=2$<sal>$<hash>`.
func (h *Hasher) Hash(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemoryKiB, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, argonMemoryKiB, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

// Verify compara en tiempo constante. Un hash vacío se compara contra el falso y siempre es false.
func (h *Hasher) Verify(password, encoded string) bool {
	if encoded == "" {
		_, _ = h.verify(password, h.dummy)
		return false
	}
	ok, err := h.verify(password, encoded)
	return err == nil && ok
}

func (h *Hasher) verify(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errBadHash
	}
	var version int
	var memory, iterations uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, errBadHash
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &threads); err != nil {
		return false, errBadHash
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, errBadHash
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(want) != argonKeyLen {
		return false, errBadHash
	}
	got := argon2.IDKey([]byte(password), salt, iterations, memory, threads, argonKeyLen)
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

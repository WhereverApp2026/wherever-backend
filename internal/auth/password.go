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

	// Topes para verificar: holgados para subir los parámetros sin migrar, y lejos de agotar la memoria.
	argonMaxMemoryKiB = 256 * 1024
	argonMaxTime      = 10
	argonMaxThreads   = 16
	argonMinSaltLen   = 8
	argonMaxSaltLen   = 64
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
	parsed, err := parseHash(encoded)
	if err != nil {
		return false, err
	}
	got := argon2.IDKey([]byte(password), parsed.salt, parsed.iterations, parsed.memory, parsed.threads, argonKeyLen)
	return subtle.ConstantTimeCompare(got, parsed.key) == 1, nil
}

// parsedHash es un hash PHC ya validado: sus parámetros son seguros para argon2.IDKey.
type parsedHash struct {
	memory, iterations uint32
	threads            uint8
	salt, key          []byte
}

// parseHash valida el formato y los parámetros. El hash viene de la base, pero un valor
// corrupto no debe tumbar la API: con t=0 o p=0 argon2.IDKey entra en pánico, y con una
// memoria enorme el proceso muere sin que recover pueda atraparlo.
func parseHash(encoded string) (parsedHash, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" || parts[2] != fmt.Sprintf("v=%d", argon2.Version) {
		return parsedHash{}, errBadHash
	}
	out, err := parseParams(parts[3])
	if err != nil {
		return parsedHash{}, err
	}
	if out.salt, err = decodeLen(parts[4], argonMinSaltLen, argonMaxSaltLen); err != nil {
		return parsedHash{}, err
	}
	if out.key, err = decodeLen(parts[5], argonKeyLen, argonKeyLen); err != nil {
		return parsedHash{}, err
	}
	return out, nil
}

func decodeLen(encoded string, minLen, maxLen int) ([]byte, error) {
	raw, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil || len(raw) < minLen || len(raw) > maxLen {
		return nil, errBadHash
	}
	return raw, nil
}

func parseParams(params string) (parsedHash, error) {
	var out parsedHash
	if _, err := fmt.Sscanf(params, "m=%d,t=%d,p=%d", &out.memory, &out.iterations, &out.threads); err != nil {
		return parsedHash{}, errBadHash
	}
	// Sscanf acepta basura al final: se exige la forma canónica.
	if params != fmt.Sprintf("m=%d,t=%d,p=%d", out.memory, out.iterations, out.threads) || !out.safeParams() {
		return parsedHash{}, errBadHash
	}
	return out, nil
}

func (p parsedHash) safeParams() bool {
	return p.iterations >= 1 && p.iterations <= argonMaxTime &&
		p.threads >= 1 && p.threads <= argonMaxThreads &&
		p.memory >= 8*uint32(p.threads) && p.memory <= argonMaxMemoryKiB
}

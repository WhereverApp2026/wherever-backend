// Package pagination define la paginación por cursor de la API (constitución §5):
// `?cursor=&limite=` → PageResult{items, siguiente_cursor, limite}.
//
// El cursor es `base64url(JSON{_id,orden,ambito}) + "." + base64url(HMAC-SHA-256)` (nv-17):
// firmado, no cifrado. Nunca lleva datos personales en `orden` ni en `ambito`.
package pagination

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Límites de `limite` (constitución §5).
const (
	DefaultLimit = 20
	MaxLimit     = 50
	// MinKeyBytes es el tamaño mínimo de CURSOR_HMAC_KEY.
	MinKeyBytes = 32
	// scopeHashBytes recorta el hash del ámbito: basta para distinguir búsquedas.
	scopeHashBytes = 12
)

// ErrInvalidCursor indica un cursor que no se puede decodificar, fue manipulado o es de
// otro ámbito.
var ErrInvalidCursor = errors.New("cursor inválido")

// errNoSigner protege a un service armado sin llave: falla cerrado en lugar de entregar
// un cursor sin firma.
var errNoSigner = errors.New("cursor sin llave")

// objectIDPattern es la forma hexadecimal de un ObjectId de MongoDB (24 caracteres).
var objectIDPattern = regexp.MustCompile(`^[0-9a-f]{24}$`)

// Cursor marca la posición del último elemento entregado. Sort es el valor del campo
// por el que se ordena e ID desempata, así el orden es estable aunque Sort se repita.
// Scope es la lista (y sus filtros o su dueño) a la que pertenece el cursor.
type Cursor struct {
	ID    string `json:"_id"`
	Sort  string `json:"orden"`
	Scope string `json:"ambito"`
}

// Signer firma y verifica cursores con CURSOR_HMAC_KEY. No tiene estado mutable.
type Signer struct {
	key []byte
}

// NewSigner exige una llave de al menos MinKeyBytes.
func NewSigner(key []byte) (*Signer, error) {
	if len(key) < MinKeyBytes {
		return nil, fmt.Errorf("la llave del cursor debe tener al menos %d bytes", MinKeyBytes)
	}
	return &Signer{key: bytes.Clone(key)}, nil
}

func (s *Signer) mac(parts ...[]byte) []byte {
	h := hmac.New(sha256.New, s.key)
	for _, p := range parts {
		h.Write(p)
	}
	return h.Sum(nil)
}

// Scope arma un ámbito `nombre:hash` con un hash con llave de las partes (filtros, dueño,
// pestaña). Con llave, el hash no deja adivinar la ubicación ni el id que lo formaron.
func (s *Signer) Scope(name string, parts ...string) string {
	if s == nil {
		return name
	}
	sum := s.mac([]byte("ambito\x00"+name), []byte("\x00"+strings.Join(parts, "\x00")))
	return name + ":" + base64.RawURLEncoding.EncodeToString(sum[:scopeHashBytes])
}

// Encode firma el cursor; el cliente lo trata como opaco. El ámbito es obligatorio.
func (s *Signer) Encode(c Cursor) (string, error) {
	if s == nil {
		return "", errNoSigner
	}
	if c.Scope == "" || !objectIDPattern.MatchString(c.ID) {
		return "", ErrInvalidCursor
	}
	raw, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	enc := base64.RawURLEncoding.EncodeToString
	return enc(raw) + "." + enc(s.mac(raw)), nil
}

// Decode verifica la firma antes de leer el JSON y exige el ámbito dado. Vacío es la
// primera página y devuelve ok=false sin error.
func (s *Signer) Decode(scope, raw string) (c Cursor, ok bool, err error) {
	if raw == "" {
		return Cursor{}, false, nil
	}
	if s == nil {
		return Cursor{}, false, ErrInvalidCursor
	}
	payload, sum, err := split(raw)
	if err != nil || !hmac.Equal(sum, s.mac(payload)) {
		return Cursor{}, false, ErrInvalidCursor
	}
	c, err = parse(payload)
	if err != nil || c.Scope != scope {
		return Cursor{}, false, ErrInvalidCursor
	}
	return c, true, nil
}

// Valid comprueba solo la forma (Base64 URL sin relleno, JSON estricto, `_id` ObjectId y
// firma de 32 bytes); la firma y el ámbito los verifica Decode. Vacío es válido.
func Valid(raw string) bool {
	if raw == "" {
		return true
	}
	payload, _, err := split(raw)
	if err != nil {
		return false
	}
	_, err = parse(payload)
	return err == nil
}

func split(raw string) (payload, sum []byte, err error) {
	left, right, found := strings.Cut(raw, ".")
	if !found {
		return nil, nil, ErrInvalidCursor
	}
	payload, err = base64.RawURLEncoding.DecodeString(left)
	if err != nil {
		return nil, nil, ErrInvalidCursor
	}
	sum, err = base64.RawURLEncoding.DecodeString(right)
	if err != nil || len(sum) != sha256.Size {
		return nil, nil, ErrInvalidCursor
	}
	return payload, sum, nil
}

func parse(payload []byte) (Cursor, error) {
	var c Cursor
	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil || dec.More() {
		return Cursor{}, ErrInvalidCursor
	}
	if !objectIDPattern.MatchString(c.ID) || c.Scope == "" {
		return Cursor{}, ErrInvalidCursor
	}
	return c, nil
}

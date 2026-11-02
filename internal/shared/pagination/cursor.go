// Package pagination define la paginación por cursor de la API (constitución §5):
// `?cursor=&limite=` → PageResult{items, siguiente_cursor, limite}.
package pagination

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
)

// Límites de `limite` (constitución §5).
const (
	DefaultLimit = 20
	MaxLimit     = 50
)

// ErrInvalidCursor indica un cursor que no se puede decodificar o fue manipulado.
var ErrInvalidCursor = errors.New("cursor inválido")

// objectIDPattern es la forma hexadecimal de un ObjectId de MongoDB (24 caracteres).
var objectIDPattern = regexp.MustCompile(`^[0-9a-f]{24}$`)

// Cursor marca la posición del último elemento entregado. Sort es el valor del campo
// por el que se ordena y ID desempata, así el orden es estable aunque Sort se repita.
type Cursor struct {
	ID   string `json:"_id"`
	Sort string `json:"orden"`
}

// Encode serializa el cursor como Base64 URL sin relleno; el cliente lo trata como opaco.
func Encode(c Cursor) string {
	raw, err := json.Marshal(c)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

// Decode lee un cursor. Vacío es la primera página y devuelve ok=false sin error.
func Decode(s string) (c Cursor, ok bool, err error) {
	if s == "" {
		return Cursor{}, false, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return Cursor{}, false, ErrInvalidCursor
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil || dec.More() {
		return Cursor{}, false, ErrInvalidCursor
	}
	if !objectIDPattern.MatchString(c.ID) {
		return Cursor{}, false, ErrInvalidCursor
	}
	return c, true, nil
}

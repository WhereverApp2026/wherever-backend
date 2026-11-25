package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Código de verificación de correo (nv-91).
const (
	CodeLength      = 6
	CodeTTL         = 15 * time.Minute
	MaxCodeAttempts = 5
	MaxCodesPerDay  = 10
)

// codeKeyLabel separa la llave del código de la firma de los JWT aunque las dos salgan de
// JWT_SECRET: el HMAC con otra etiqueta da una llave distinta.
const codeKeyLabel = "wherever/codigo-correo/v1"

var codeSpace = big.NewInt(1_000_000)

// CodeHasher calcula el HMAC de los códigos; el código nunca se guarda en claro.
type CodeHasher struct {
	key []byte
}

// NewCodeHasher deriva la llave de los códigos del secreto de los JWT.
func NewCodeHasher(secret []byte) CodeHasher {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(codeKeyLabel))
	return CodeHasher{key: mac.Sum(nil)}
}

// Hash firma el id del documento junto con el código: el id evita que dos documentos con los
// mismos dígitos choquen en `ux_token_hash`.
func (h CodeHasher) Hash(id bson.ObjectID, code string) string {
	mac := hmac.New(sha256.New, h.key)
	mac.Write(id[:])
	mac.Write([]byte(code))
	return hex.EncodeToString(mac.Sum(nil))
}

// Matches compara en tiempo constante.
func (h CodeHasher) Matches(id bson.ObjectID, code, stored string) bool {
	return hmac.Equal([]byte(h.Hash(id, code)), []byte(stored))
}

// NewCode devuelve un código uniforme de 6 dígitos (000000–999999).
func NewCode() (string, error) {
	n, err := rand.Int(rand.Reader, codeSpace)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%0*d", CodeLength, n.Int64()), nil
}

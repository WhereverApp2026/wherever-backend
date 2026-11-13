// Package cifrado cifra y descifra campos sensibles con AES-256-GCM y llaves rotables
// (nv-48, US-226). Cada valor queda como `enc:v1:<id_llave>:<nonce>:<texto_cifrado>`
// (base64url sin relleno); el dato asociado es colección, campo e id del documento, así un
// valor copiado a otro lugar no descifra.
package cifrado

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const (
	prefix   = "enc:v1:"
	keyBytes = 32
)

// Errores. ErrNoAbre no dice por qué: llave, dato asociado o texto alterado dan lo mismo.
var (
	ErrFormato  = errors.New("cifrado: valor con formato inválido")
	ErrNoAbre   = errors.New("cifrado: el valor no se pudo descifrar")
	ErrSinLlave = errors.New("cifrado: llave desconocida")
	ErrLlavero  = errors.New("cifrado: DATA_KEYS inválido")
)

var keyIDPattern = regexp.MustCompile(`^[a-z0-9]{1,16}$`)

// Llavero tiene las llaves por id. La activa cifra; todas descifran (rotación). No cambia
// después de cargarse, así que es seguro entre goroutines.
type Llavero struct {
	activa string
	llaves map[string]cipher.AEAD
}

// CargarLlavero lee DATA_KEYS: `id:base64,id:base64,…` con llaves de 32 bytes; la primera es
// la activa. Para rotar se pone la nueva al principio y se deja la vieja detrás.
func CargarLlavero(raw string) (*Llavero, error) {
	l := &Llavero{llaves: map[string]cipher.AEAD{}}
	for _, entry := range strings.Split(strings.TrimSpace(raw), ",") {
		id, b64, ok := strings.Cut(strings.TrimSpace(entry), ":")
		if !ok || !keyIDPattern.MatchString(id) {
			return nil, fmt.Errorf("%w: cada llave es id:base64 (id en minúsculas y números)", ErrLlavero)
		}
		key, err := base64.StdEncoding.DecodeString(b64)
		if err != nil || len(key) != keyBytes {
			return nil, fmt.Errorf("%w: la llave %s debe ser de 32 bytes en base64", ErrLlavero, id)
		}
		if _, dup := l.llaves[id]; dup {
			return nil, fmt.Errorf("%w: la llave %s se repite", ErrLlavero, id)
		}
		block, err := aes.NewCipher(key)
		if err != nil {
			return nil, ErrLlavero
		}
		aead, err := cipher.NewGCM(block)
		if err != nil {
			return nil, ErrLlavero
		}
		l.llaves[id] = aead
		if l.activa == "" {
			l.activa = id
		}
	}
	return l, nil
}

// Activa es el id de la llave que cifra.
func (l *Llavero) Activa() string { return l.activa }

// Lugar ata un valor a su documento: colección, campo e id.
type Lugar struct {
	Coleccion string
	Campo     string
	ID        string
}

func (p Lugar) aad() []byte {
	return []byte(p.Coleccion + "\x00" + p.Campo + "\x00" + p.ID)
}

// Cifrar cifra texto con la llave activa y un nonce aleatorio de 12 bytes.
func (l *Llavero) Cifrar(p Lugar, texto string) (string, error) {
	aead := l.llaves[l.activa]
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := aead.Seal(nil, nonce, []byte(texto), p.aad())
	enc := base64.RawURLEncoding.EncodeToString
	return prefix + l.activa + ":" + enc(nonce) + ":" + enc(sealed), nil
}

// EsCifrado dice si el valor tiene el formato de un valor cifrado.
func EsCifrado(valor string) bool { return strings.HasPrefix(valor, prefix) }

// LlaveDe devuelve el id de la llave con que se cifró el valor (para rotar).
func LlaveDe(valor string) (string, error) {
	id, _, _, err := partes(valor)
	return id, err
}

func partes(valor string) (id string, nonce, sealed []byte, err error) {
	rest, ok := strings.CutPrefix(valor, prefix)
	if !ok {
		return "", nil, nil, ErrFormato
	}
	fields := strings.Split(rest, ":")
	if len(fields) != 3 || !keyIDPattern.MatchString(fields[0]) {
		return "", nil, nil, ErrFormato
	}
	nonce, errN := base64.RawURLEncoding.DecodeString(fields[1])
	sealed, errS := base64.RawURLEncoding.DecodeString(fields[2])
	if errN != nil || errS != nil || len(nonce) != 12 || len(sealed) < 16 {
		return "", nil, nil, ErrFormato
	}
	return fields[0], nonce, sealed, nil
}

// Descifrar abre un valor cifrado para su lugar. Un valor alterado, copiado a otro documento
// o de otra llave falla.
func (l *Llavero) Descifrar(p Lugar, valor string) (string, error) {
	id, nonce, sealed, err := partes(valor)
	if err != nil {
		return "", err
	}
	aead, ok := l.llaves[id]
	if !ok {
		return "", ErrSinLlave
	}
	plain, err := aead.Open(nil, nonce, sealed, p.aad())
	if err != nil {
		return "", ErrNoAbre
	}
	return string(plain), nil
}

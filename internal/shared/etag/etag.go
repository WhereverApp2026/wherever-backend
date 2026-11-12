// Package etag calcula ETags débiles y evalúa If-None-Match (spec nv-16; nv-17 lo reutiliza).
package etag

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

const prefijoDebil = "W/"

// De devuelve el ETag débil de un cuerpo: W/"<32 hex de SHA-256>". Es débil porque la
// compresión puede cambiar los bytes enviados sin cambiar el contenido (RFC 9110).
func De(cuerpo []byte) string {
	suma := sha256.Sum256(cuerpo)
	return prefijoDebil + `"` + hex.EncodeToString(suma[:])[:32] + `"`
}

// Coincide aplica la comparación débil de If-None-Match: acepta `*`, una lista separada por
// comas y valores con o sin `W/`.
func Coincide(ifNoneMatch, actual string) bool {
	ifNoneMatch = strings.TrimSpace(ifNoneMatch)
	if ifNoneMatch == "" || actual == "" {
		return false
	}
	if ifNoneMatch == "*" {
		return true
	}
	opaco := strings.TrimPrefix(actual, prefijoDebil)
	for _, v := range strings.Split(ifNoneMatch, ",") {
		if strings.TrimPrefix(strings.TrimSpace(v), prefijoDebil) == opaco {
			return true
		}
	}
	return false
}

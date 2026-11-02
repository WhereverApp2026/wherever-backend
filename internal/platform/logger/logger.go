// Package logger arma el slog de la API con filtro de secretos (constitución §6).
package logger

import (
	"io"
	"log/slog"
	"regexp"
)

// Redacted reemplaza el valor de cualquier atributo sensible.
const Redacted = "[REDACTADO]"

var sensitiveKey = regexp.MustCompile(`(?i)(password|contrasena|secret|token|authorization|cookie|jwt|key|uri|dsn)`)

// New crea un logger JSON; en dev usa texto legible en consola.
func New(w io.Writer, env string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo, ReplaceAttr: redact}
	if env == "dev" {
		return slog.New(slog.NewTextHandler(w, opts))
	}
	return slog.New(slog.NewJSONHandler(w, opts))
}

// redact oculta el valor de los atributos con nombre sensible.
func redact(_ []string, attr slog.Attr) slog.Attr {
	if sensitiveKey.MatchString(attr.Key) {
		return slog.String(attr.Key, Redacted)
	}
	return attr
}

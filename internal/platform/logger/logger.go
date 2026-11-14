// Package logger arma el slog de la API con filtro de secretos (constitución §6).
package logger

import (
	"io"
	"log/slog"
	"regexp"
)

// Redacted reemplaza el valor de cualquier atributo sensible.
const Redacted = "[REDACTADO]"

var sensitiveKey = regexp.MustCompile(`(?i)(password|contrasena|secret|token|authorization|cookie|jwt|key|uri|dsn|clabe|rfc|razon_social|archivo|signature)`)

// Un texto libre, como el error de una base, puede traer un correo o una URI con contraseña.
var (
	emailInText = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)
	uriUserinfo = regexp.MustCompile(`://[^/\s:@]+:[^/\s@]+@`)
	// Llave de API de Google (Gemini, nv-26), por si un error de terceros la repite.
	googleAPIKey = regexp.MustCompile(`AIza[0-9A-Za-z_-]{35}`)
)

// New crea un logger JSON; en dev usa texto legible en consola.
func New(w io.Writer, env string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo, ReplaceAttr: redact}
	if env == "dev" {
		return slog.New(slog.NewTextHandler(w, opts))
	}
	return slog.New(slog.NewJSONHandler(w, opts))
}

// RedactText oculta correos, llaves de API y credenciales de URI en un texto libre; un correo
// ya enmascarado (a***@x.com) no coincide. También lo usa el reporte de errores de la app (nv-64).
func RedactText(text string) string {
	text = uriUserinfo.ReplaceAllString(text, "://"+Redacted+"@")
	text = googleAPIKey.ReplaceAllString(text, Redacted)
	return emailInText.ReplaceAllString(text, Redacted)
}

// redact oculta el valor de los atributos con nombre sensible.
func redact(_ []string, attr slog.Attr) slog.Attr {
	if sensitiveKey.MatchString(attr.Key) {
		return slog.String(attr.Key, Redacted)
	}
	if attr.Value.Kind() == slog.KindString {
		return slog.String(attr.Key, RedactText(attr.Value.String()))
	}
	return attr
}

package mailer

import (
	"context"
	"log/slog"
	"strings"
)

// Log no envía nada: registra solo el tipo y el destinatario enmascarado, nunca el
// enlace ni el token (constitución §6 y §9: sin secretos ni datos personales en logs).
type Log struct {
	log *slog.Logger
}

// NewLog crea el mailer de desarrollo.
func NewLog(log *slog.Logger) *Log { return &Log{log: log} }

// Send registra el correo que se habría enviado.
func (l *Log) Send(ctx context.Context, msg Message) error {
	l.log.InfoContext(ctx, "correo no enviado: sin proveedor configurado", "tipo", string(msg.Kind), "destinatario", Mask(msg.To))
	return nil
}

// Mask deja la primera letra del usuario y el dominio: "a***@dominio.com".
func Mask(email string) string {
	user, domain, ok := strings.Cut(email, "@")
	if !ok || user == "" {
		return "***"
	}
	return user[:1] + "***@" + domain
}

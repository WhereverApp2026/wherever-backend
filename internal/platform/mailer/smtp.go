package mailer

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// Modos de TLS de SMTP_TLS.
const (
	TLSStartTLS = "starttls"
	TLSImplicit = "tls"
	TLSNone     = "none"
)

// Errores del envío. Nunca llevan la respuesta cruda del servidor (puede traer el
// destinatario u otros datos) ni el contenido del correo.
var (
	ErrHeaderInjection = errors.New("encabezado con salto de línea")
	errInsecureAuth    = errors.New("autenticación SMTP sin TLS")
	errNoStartTLS      = errors.New("el servidor no ofrece STARTTLS")
)

// SMTPConfig son los datos del servidor, resueltos desde config.Config en main.
type SMTPConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	TLSMode  string
	From     string
}

// SMTP entrega un correo por intento con la librería estándar; el proveedor (SES, Resend,
// Brevo, Mailpit…) es solo configuración. Los reintentos y el plazo los pone Retrying.
type SMTP struct {
	cfg   SMTPConfig
	links Links
	log   *slog.Logger
}

// NewSMTP recibe la configuración ya validada.
func NewSMTP(cfg SMTPConfig, links Links, log *slog.Logger) *SMTP {
	return &SMTP{cfg: cfg, links: links, log: log}
}

// Send arma el correo y lo entrega. Registra solo el tipo, el destinatario enmascarado, el
// resultado y la duración.
func (s *SMTP) Send(ctx context.Context, msg Message) error {
	start := time.Now()
	err := s.send(ctx, msg)
	attrs := []any{"tipo", string(msg.Kind), "destinatario", Mask(msg.To), "duracion_ms", time.Since(start).Milliseconds()}
	if err != nil {
		s.log.WarnContext(ctx, "correo no enviado", append(attrs, "motivo", reason(err))...)
		return err
	}
	s.log.InfoContext(ctx, "correo enviado", attrs...)
	return nil
}

func (s *SMTP) send(ctx context.Context, msg Message) error {
	link, err := s.links.URL(msg.Kind, msg.Token)
	if err != nil {
		return err
	}
	rendered, err := Render(msg, link)
	if err != nil {
		return err
	}
	raw, err := BuildMIME(s.cfg.From, msg.To, rendered)
	if err != nil {
		return err
	}
	client, err := s.dial(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()
	if err := s.authenticate(client); err != nil {
		return err
	}
	return deliver(client, s.cfg.From, msg.To, raw)
}

// dial abre la conexión con el plazo del contexto y aplica el modo de TLS.
func (s *SMTP) dial(ctx context.Context) (*smtp.Client, error) {
	addr := net.JoinHostPort(s.cfg.Host, strconv.Itoa(s.cfg.Port))
	tlsCfg := &tls.Config{ServerName: s.cfg.Host, MinVersion: tls.VersionTLS12}
	var conn net.Conn
	var err error
	dialer := &net.Dialer{}
	if s.cfg.TLSMode == TLSImplicit {
		conn, err = (&tls.Dialer{NetDialer: dialer, Config: tlsCfg}).DialContext(ctx, "tcp", addr)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return nil, fmt.Errorf("conexión: %w", err)
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	client, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		_ = conn.Close()
		return nil, errors.New("saludo SMTP rechazado")
	}
	if s.cfg.TLSMode == TLSStartTLS {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			_ = client.Close()
			return nil, errNoStartTLS
		}
		if err := client.StartTLS(tlsCfg); err != nil {
			_ = client.Close()
			return nil, fmt.Errorf("starttls: %w", err)
		}
	}
	return client, nil
}

// authenticate solo autentica si hay usuario, y nunca por un canal sin TLS (PlainAuth lo
// permitiría hacia localhost; aquí no).
func (s *SMTP) authenticate(client *smtp.Client) error {
	auth := authFor(s.cfg)
	if auth == nil {
		return nil
	}
	if _, isTLS := client.TLSConnectionState(); !isTLS {
		return errInsecureAuth
	}
	if err := client.Auth(auth); err != nil {
		return errors.New("autenticación SMTP rechazada")
	}
	return nil
}

// authFor arma PlainAuth solo si hay usuario: Mailpit y servidores sin cuenta no autentican.
func authFor(cfg SMTPConfig) smtp.Auth {
	if cfg.User == "" {
		return nil
	}
	return smtp.PlainAuth("", cfg.User, cfg.Password, cfg.Host)
}

func deliver(client *smtp.Client, from, to string, raw []byte) error {
	if err := client.Mail(from); err != nil {
		return errors.New("remitente rechazado")
	}
	if err := client.Rcpt(to); err != nil {
		return errors.New("destinatario rechazado")
	}
	w, err := client.Data()
	if err != nil {
		return errors.New("DATA rechazado")
	}
	if _, err := w.Write(raw); err != nil {
		return errors.New("envío del cuerpo interrumpido")
	}
	if err := w.Close(); err != nil {
		return errors.New("mensaje rechazado")
	}
	_ = client.Quit() // el servidor ya aceptó el mensaje; un QUIT fallido no lo deshace
	return nil
}

// BuildMIME arma un multipart/alternative (texto y HTML) en quoted-printable. Remitente,
// destinatario y asunto no pueden traer CR/LF (inyección de encabezados).
func BuildMIME(from, to string, r Rendered) ([]byte, error) {
	for _, h := range []string{from, to, r.Subject} {
		if strings.ContainsAny(h, "\r\n") {
			return nil, ErrHeaderInjection
		}
	}
	boundary, err := newBoundary()
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	fmt.Fprintf(&b, "From: %s\r\nTo: %s\r\nSubject: %s\r\n", from, to, mime.QEncoding.Encode("utf-8", r.Subject))
	fmt.Fprintf(&b, "Date: %s\r\nMIME-Version: 1.0\r\n", time.Now().UTC().Format(time.RFC1123Z))
	fmt.Fprintf(&b, "Content-Type: multipart/alternative; boundary=%q\r\n\r\n", boundary)
	for _, part := range []struct{ ctype, body string }{{"text/plain", r.Text}, {"text/html", r.HTML}} {
		fmt.Fprintf(&b, "--%s\r\nContent-Type: %s; charset=utf-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n", boundary, part.ctype)
		qp := quotedprintable.NewWriter(&b)
		if _, err := qp.Write([]byte(part.body)); err != nil {
			return nil, err
		}
		if err := qp.Close(); err != nil {
			return nil, err
		}
		b.WriteString("\r\n")
	}
	fmt.Fprintf(&b, "--%s--\r\n", boundary)
	return b.Bytes(), nil
}

func newBoundary() (string, error) {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "wherever-" + hex.EncodeToString(buf), nil
}

// reason resume el error para el log sin la respuesta del servidor.
func reason(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "tiempo agotado"
	case errors.Is(err, ErrHeaderInjection):
		return "encabezado inválido"
	case errors.Is(err, errInsecureAuth), errors.Is(err, errNoStartTLS):
		return "sin TLS"
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return "tiempo agotado"
		}
		return "sin conexión"
	}
	return "rechazado"
}

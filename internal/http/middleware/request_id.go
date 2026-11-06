package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// HeaderRequestID viaja en la petición y en la respuesta; su valor es el `traza_id`.
const HeaderRequestID = "X-Request-ID"

const (
	localRequestID = "request_id"
	localClientIP  = "client_ip"
	requestIDBytes = 16
)

// Un request_id entrante solo se acepta si es corto y sin caracteres raros.
var validRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{8,64}$`)

// RequestID asigna un identificador por petición (el entrante si es válido).
func RequestID() fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Get(HeaderRequestID)
		if !validRequestID.MatchString(id) {
			id = newRequestID()
		}
		c.Locals(localRequestID, id)
		c.Set(HeaderRequestID, id)
		return c.Next()
	}
}

// RequestIDFrom lee el request_id de la petición; vacío si aún no se asignó.
func RequestIDFrom(c *fiber.Ctx) string {
	id, _ := c.Locals(localRequestID).(string)
	return id
}

func newRequestID() string {
	buf := make([]byte, requestIDBytes)
	_, _ = rand.Read(buf) // crypto/rand.Read no falla en plataformas soportadas
	return hex.EncodeToString(buf)
}

// RequestTimeout pone el plazo máximo (60 s) en el contexto de la petición.
func RequestTimeout(timeout time.Duration) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.UserContext(), timeout)
		defer cancel()
		c.SetUserContext(ctx)
		return c.Next()
	}
}

// ProxyHeaders resuelve la IP del cliente. X-Forwarded-For solo se lee si la conexión viene de un
// proxy de confianza (TRUSTED_PROXIES), y de ella se toma la entrada a `hops` del final: lo que está
// más a la izquierda lo pudo escribir el cliente, y creerlo evadiría los límites por IP (nv-12).
func ProxyHeaders(hops int) fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Locals(localClientIP, resolveClientIP(c, hops))
		return c.Next()
	}
}

// resolveClientIP cae a la dirección de la conexión si la cabecera no alcanza o no es una IP.
func resolveClientIP(c *fiber.Ctx, hops int) string {
	remote := c.Context().RemoteIP().String()
	if !c.IsProxyTrusted() {
		return remote
	}
	entries := forwardedEntries(c)
	if len(entries) < hops {
		return remote
	}
	ip := net.ParseIP(strings.TrimSpace(entries[len(entries)-hops]))
	if ip == nil {
		return remote
	}
	return ip.String()
}

// forwardedEntries une todas las cabeceras X-Forwarded-For en orden, como pide el estándar HTTP.
func forwardedEntries(c *fiber.Ctx) []string {
	var entries []string
	for _, header := range c.Request().Header.PeekAll(fiber.HeaderXForwardedFor) {
		entries = append(entries, strings.Split(string(header), ",")...)
	}
	return entries
}

// ClientIP devuelve la IP resuelta por ProxyHeaders.
func ClientIP(c *fiber.Ctx) string {
	if ip, ok := c.Locals(localClientIP).(string); ok {
		return ip
	}
	return c.IP()
}

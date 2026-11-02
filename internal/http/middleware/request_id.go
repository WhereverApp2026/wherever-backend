package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"regexp"
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

// ProxyHeaders resuelve la IP del cliente. Fiber solo lee X-Forwarded-For de los
// proxies de confianza (TRUSTED_PROXIES); con la lista vacía usa la IP del socket.
func ProxyHeaders() fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Locals(localClientIP, c.IP())
		return c.Next()
	}
}

// ClientIP devuelve la IP resuelta por ProxyHeaders.
func ClientIP(c *fiber.Ctx) string {
	if ip, ok := c.Locals(localClientIP).(string); ok {
		return ip
	}
	return c.IP()
}

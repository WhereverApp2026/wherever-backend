package middleware

import (
	"github.com/gofiber/fiber/v2"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

const (
	corsMethods = "GET, POST, PUT, PATCH, DELETE"
	corsHeaders = "Authorization, Content-Type, If-Match, Idempotency-Key, X-Request-ID"
	corsExpose  = "X-Request-ID, Retry-After"
	corsMaxAge  = "600"
)

// CORS admite un solo origen exacto. Con el origen vacío no se responde ningún
// encabezado CORS y el navegador bloquea las peticiones de otros orígenes.
func CORS(allowedOrigin string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		origin := c.Get(fiber.HeaderOrigin)
		preflight := c.Method() == fiber.MethodOptions && c.Get(fiber.HeaderAccessControlRequestMethod) != ""
		allowed := allowedOrigin != "" && origin == allowedOrigin

		if !allowed {
			if preflight {
				return apperr.Forbidden("origen no permitido")
			}
			return c.Next()
		}
		c.Vary(fiber.HeaderOrigin)
		c.Set(fiber.HeaderAccessControlAllowOrigin, allowedOrigin)
		if !preflight {
			c.Set(fiber.HeaderAccessControlExposeHeaders, corsExpose)
			return c.Next()
		}
		c.Set(fiber.HeaderAccessControlAllowMethods, corsMethods)
		c.Set(fiber.HeaderAccessControlAllowHeaders, corsHeaders)
		c.Set(fiber.HeaderAccessControlMaxAge, corsMaxAge)
		return c.SendStatus(fiber.StatusNoContent)
	}
}

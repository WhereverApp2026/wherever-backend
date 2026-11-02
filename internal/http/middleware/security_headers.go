// Package middleware contiene las etapas del pipeline fijo (constitución §4).
package middleware

import "github.com/gofiber/fiber/v2"

const (
	cspStrict = "default-src 'none'; frame-ancestors 'none'"
	// cspDocs relaja la política solo para Swagger UI en dev (scripts y estilos del CDN fijado).
	cspDocs = "default-src 'none'; script-src https://unpkg.com 'unsafe-inline'; " +
		"style-src https://unpkg.com 'unsafe-inline'; img-src 'self' data: https://unpkg.com; " +
		"connect-src 'self'; frame-ancestors 'none'"
	hstsValue = "max-age=31536000; includeSubDomains"
)

// DocsPaths son las rutas de documentación exentas de la CSP estricta en dev.
var DocsPaths = []string{"/docs", "/openapi.json"}

// SecurityHeaders es la etapa más externa: fija los encabezados antes de seguir,
// así también los llevan las respuestas de error.
func SecurityHeaders(env string) fiber.Handler {
	isDev := env == "dev"
	return func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderXContentTypeOptions, "nosniff")
		c.Set(fiber.HeaderXFrameOptions, "DENY")
		c.Set(fiber.HeaderReferrerPolicy, "strict-origin-when-cross-origin")
		c.Set(fiber.HeaderXXSSProtection, "0")
		c.Set(fiber.HeaderContentSecurityPolicy, cspFor(isDev, c.Path()))
		if !isDev {
			c.Set(fiber.HeaderStrictTransportSecurity, hstsValue)
		}
		return c.Next()
	}
}

func cspFor(isDev bool, path string) string {
	if isDev {
		for _, docs := range DocsPaths {
			if path == docs {
				return cspDocs
			}
		}
	}
	return cspStrict
}

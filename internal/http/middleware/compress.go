package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
)

const (
	authPrefix   = "/api/v1/auth"
	registerPath = "/api/v1/usuarios"
)

// Compress comprime las respuestas según Accept-Encoding: detrás de CloudFront sin caché y sin Caddy
// nadie más lo hace (nv-12). No toca las que llevan tokens, para no exponerlos a ataques de tamaño.
func Compress() fiber.Handler {
	return compress.New(compress.Config{Level: compress.LevelDefault, Next: carriesTokens})
}

func carriesTokens(c *fiber.Ctx) bool {
	path := c.Path()
	if strings.HasPrefix(path, authPrefix) {
		return true
	}
	return c.Method() == fiber.MethodPost && path == registerPath
}

package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
)

const (
	authPrefix   = "/api/v1/auth"
	registerPath = "/api/v1/usuarios"
	// El código del enlace de un plan (nv-38) es una credencial: tampoco se comprime.
	plansPrefix = "/api/v1/planes/"
	shareSuffix = "/compartir"
	// El expediente del negocio (nv-55) se entrega descifrado: ni se comprime ni se guarda.
	businessPrefix = "/api/v1/negocios/"
	adminBusiness  = "/api/v1/admin/negocios/"
	docsSegment    = "/documentos/"
	// El cobro del anticipo (nv-60) devuelve el `client_secret` de Stripe; el webhook tampoco se
	// comprime.
	bookingsPrefix = "/api/v1/reservas/"
	paySuffix      = "/pago"
	// StripeWebhookPath es la ruta del webhook de Stripe (nv-60).
	StripeWebhookPath = "/api/v1/webhooks/stripe"
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
	if c.Method() == fiber.MethodGet {
		return strings.Contains(path, docsSegment) && (strings.HasPrefix(path, businessPrefix) || strings.HasPrefix(path, adminBusiness))
	}
	return c.Method() == fiber.MethodPost && postCarriesTokens(path)
}

// postCarriesTokens: registro, enlace de plan, cobro del anticipo y webhook de Stripe.
func postCarriesTokens(path string) bool {
	return path == registerPath || path == StripeWebhookPath ||
		(strings.HasPrefix(path, plansPrefix) && strings.HasSuffix(path, shareSuffix)) ||
		(strings.HasPrefix(path, bookingsPrefix) && strings.HasSuffix(path, paySuffix))
}

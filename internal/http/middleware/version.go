package middleware

import (
	"github.com/gofiber/fiber/v2"

	"github.com/WhereverApp2026/wherever-backend/internal/platform/versionado"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

// HeaderAppVersion es la cabecera con la versión de la app (nv-16).
const HeaderAppVersion = "X-App-Version"

// maintenanceRetryAfter son los segundos de `Retry-After` en mantenimiento.
const maintenanceRetryAfter = 60

// Version traduce la política de versionado a 426 o 503. Solo actúa sobre `/api/v1/` no
// exentas; el cuerpo sale del manejador uniforme de errores.
func Version(p versionado.Politica) fiber.Handler {
	minima := p.Minima.String()
	return func(c *fiber.Ctx) error {
		switch p.Decidir(c.Path(), c.Get(HeaderAppVersion)) {
		case versionado.Mantenimiento:
			return apperr.Maintenance(maintenanceRetryAfter)
		case versionado.Actualizar:
			return apperr.UpgradeRequired(minima)
		default:
			return c.Next()
		}
	}
}

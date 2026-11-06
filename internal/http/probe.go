package httpapi

import (
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

const slowProbeDelay = 45 * time.Second

// registerProbes monta las sondas de errores; solo fuera de prod (en prod dan 404).
func registerProbes(probe fiber.Router) {
	// Para probar peticiones largas por CloudFront (nv-12); oculta como las demás.
	probe.Get("/lenta", func(c *fiber.Ctx) error {
		select {
		case <-time.After(slowProbeDelay):
			return c.JSON(fiber.Map{"estado": "ok"})
		case <-c.UserContext().Done():
			return apperr.Timeout("sonda lenta agotada")
		}
	})
	probe.Get("/timeout", func(*fiber.Ctx) error {
		return apperr.Timeout("sonda de tiempo agotado")
	})
	probe.Get("/error", func(*fiber.Ctx) error {
		return errors.New("sonda de error interno")
	})
	probe.Get("/forbidden", func(*fiber.Ctx) error {
		return apperr.Forbidden("sonda de acceso prohibido")
	})
	probe.Get("/conflict", func(*fiber.Ctx) error {
		return apperr.Conflict("sonda de conflicto").WithDetail("version esperada 2, recibida 1")
	})
	probe.Get("/invalid", func(*fiber.Ctx) error {
		return apperr.Validation("sonda de validación",
			apperr.FieldError{Field: "nombre", Message: "es obligatorio"})
	})
}

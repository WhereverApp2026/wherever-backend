package inicio

import (
	"github.com/gofiber/fiber/v2"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

// Handler traduce HTTP ↔ service.
type Handler struct {
	svc       *Service
	validator *validation.Validator
}

// NewHandler recibe el service y el validador por constructor.
func NewHandler(svc *Service, v *validation.Validator) *Handler {
	return &Handler{svc: svc, validator: v}
}

// Routes registra GET /api/v1/inicio con sesión opcional: un token inválido sigue siendo 401
// (middleware JWT); sin token, Inicio general. Cumple httpapi.Module.
func (h *Handler) Routes(v1 fiber.Router, _ *middleware.Limiter) {
	v1.Get("/inicio", h.home)
}

func (h *Handler) home(c *fiber.Ctx) error {
	var q Query
	if err := c.QueryParser(&q); err != nil {
		return apperr.BadRequest("parámetros inválidos")
	}
	if err := h.validator.Struct(q); err != nil {
		return err
	}
	subject := ""
	if claims, ok := middleware.ClaimsFrom(c); ok {
		subject = claims.Subject
	}
	out, err := h.svc.Home(c.UserContext(), subject, q)
	if err != nil {
		return err
	}
	// Depende del usuario y de la hora: nunca en caché compartida.
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.JSON(out)
}

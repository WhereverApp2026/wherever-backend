package sincronizacion

import (
	"github.com/gofiber/fiber/v2"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

// Handler traduce HTTP ↔ service. No toca la base ni atrapa pánicos (test de arquitectura).
type Handler struct {
	svc       *Service
	validator *validation.Validator
}

// NewHandler recibe el service y el validador por constructor.
func NewHandler(svc *Service, v *validation.Validator) *Handler {
	return &Handler{svc: svc, validator: v}
}

// Routes registra /api/v1/sync. Cumple httpapi.Module.
func (h *Handler) Routes(v1 fiber.Router, _ *middleware.Limiter) {
	g := v1.Group("/sync", middleware.RequireAuth())
	g.Get("/cambios", h.changes)
}

func (h *Handler) changes(c *fiber.Ctx) error {
	q := NewChangesQuery()
	if err := c.QueryParser(&q); err != nil {
		return apperr.BadRequest("parámetros inválidos")
	}
	if err := h.validator.Struct(q); err != nil {
		return err
	}
	subject := ""
	if cl, ok := middleware.ClaimsFrom(c); ok {
		subject = cl.Subject
	}
	out, err := h.svc.Changes(c.UserContext(), subject, q)
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.JSON(out)
}

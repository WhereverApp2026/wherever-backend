package recomendaciones

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
	active    fiber.Handler
}

// NewHandler recibe el service y el validador por constructor.
func NewHandler(svc *Service, v *validation.Validator) *Handler {
	return &Handler{svc: svc, validator: v}
}

// WithActiveGuard agrega usuarios.RequireActive: con la cuenta en eliminación pendiente
// las rutas responden 403.
func (h *Handler) WithActiveGuard(guard fiber.Handler) *Handler {
	h.active = guard
	return h
}

// Routes registra /api/v1/recomendaciones. Cumple httpapi.Module.
func (h *Handler) Routes(v1 fiber.Router, _ *middleware.Limiter) {
	g := v1.Group("/recomendaciones", middleware.RequireAuth())
	chain := func(handler fiber.Handler) []fiber.Handler {
		if h.active == nil {
			return []fiber.Handler{handler}
		}
		return []fiber.Handler{h.active, handler}
	}
	g.Get("", chain(h.list)...)
	g.Post("/:negocio_id/no-me-interesa", chain(h.notInterested)...)
	g.Delete("/:negocio_id/no-me-interesa", chain(h.undo)...)
}

func subject(c *fiber.Ctx) string {
	if claims, ok := middleware.ClaimsFrom(c); ok {
		return claims.Subject
	}
	return ""
}

func (h *Handler) list(c *fiber.Ctx) error {
	q := Query{Limit: DefaultLimit}
	if err := c.QueryParser(&q); err != nil {
		return apperr.BadRequest("parámetros inválidos")
	}
	if err := h.validator.Struct(q); err != nil {
		return err
	}
	out, err := h.svc.Recommend(c.UserContext(), subject(c), q)
	if err != nil {
		return err
	}
	// Depende del usuario y de la hora: nunca en caché compartida.
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.JSON(out)
}

func (h *Handler) notInterested(c *fiber.Ctx) error {
	if err := h.svc.NotInterested(c.UserContext(), subject(c), c.Params("negocio_id")); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) undo(c *fiber.Ctx) error {
	if err := h.svc.Undo(c.UserContext(), subject(c), c.Params("negocio_id")); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

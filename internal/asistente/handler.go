package asistente

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

// WithActiveGuard agrega usuarios.RequireActive: con eliminación pendiente responde 403.
func (h *Handler) WithActiveGuard(guard fiber.Handler) *Handler {
	h.active = guard
	return h
}

// Routes registra POST /api/v1/planes/generar. Cumple httpapi.Module. Los límites de IA por
// usuario y globales (nv-27/nv-28) se aplican dentro de ia.Guardia al armarse en nv-33.
func (h *Handler) Routes(v1 fiber.Router, _ *middleware.Limiter) {
	chain := []fiber.Handler{middleware.RequireAuth()}
	if h.active != nil {
		chain = append(chain, h.active)
	}
	v1.Post("/planes/generar", append(chain, h.generar)...)
}

func (h *Handler) generar(c *fiber.Ctx) error {
	var req GenerarRequest
	if err := c.BodyParser(&req); err != nil {
		return apperr.BadRequest("cuerpo JSON inválido")
	}
	normalizarFrase(&req)
	if err := h.validator.Struct(req); err != nil {
		return err
	}
	cl, _ := middleware.ClaimsFrom(c)
	out, err := h.svc.Generar(c.UserContext(), cl.Subject, req)
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.JSON(out)
}

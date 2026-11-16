package consejos

import (
	"github.com/gofiber/fiber/v2"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
)

// Handler traduce HTTP ↔ service. No toca la base ni atrapa pánicos (test de arquitectura).
type Handler struct {
	svc    *Service
	active fiber.Handler
}

// NewHandler recibe el service por constructor; la ruta no tiene cuerpo ni query.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// WithActiveGuard agrega usuarios.RequireActive.
func (h *Handler) WithActiveGuard(guard fiber.Handler) *Handler {
	h.active = guard
	return h
}

// Routes registra el consejo del día del dueño. Cumple httpapi.Module.
func (h *Handler) Routes(v1 fiber.Router, _ *middleware.Limiter) {
	chain := []fiber.Handler{middleware.RequireRoles(middleware.RoleMerchant)}
	if h.active != nil {
		chain = append(chain, h.active)
	}
	v1.Get("/negocios/:id/consejo", append(chain, h.consejo)...)
}

func (h *Handler) consejo(c *fiber.Ctx) error {
	cl, _ := middleware.ClaimsFrom(c)
	out, err := h.svc.Consejo(c.UserContext(), cl, c.Params("id"))
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.JSON(out)
}

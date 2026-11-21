package insignias

import (
	"github.com/gofiber/fiber/v2"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

// Handler traduce HTTP ↔ service. No toca la base ni atrapa pánicos (test de arquitectura).
type Handler struct {
	svc    *Service
	active fiber.Handler
}

// NewHandler recibe el service y el guardia de cuenta activa por constructor.
func NewHandler(svc *Service, active fiber.Handler) *Handler {
	return &Handler{svc: svc, active: active}
}

// Routes registra GET /usuarios/yo/insignias. Cumple httpapi.Module. Las de otra persona
// van con su perfil en personas (GET /personas/{id}/insignias).
func (h *Handler) Routes(v1 fiber.Router, _ *middleware.Limiter) {
	chain := []fiber.Handler{middleware.RequireAuth()}
	if h.active != nil {
		chain = append(chain, h.active)
	}
	v1.Get("/usuarios/yo/insignias", append(chain, h.mias)...)
}

func (h *Handler) mias(c *fiber.Ctx) error {
	claims, ok := middleware.ClaimsFrom(c)
	if !ok {
		return apperr.Unauthorized("autenticación requerida")
	}
	out, err := h.svc.Mias(c.UserContext(), claims.Subject)
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.JSON(out)
}

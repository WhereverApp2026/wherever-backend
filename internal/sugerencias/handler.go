package sugerencias

import (
	"github.com/gofiber/fiber/v2"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
)

// Handler traduce HTTP ↔ service.
type Handler struct {
	svc    *Service
	active fiber.Handler
}

// NewHandler recibe el service por constructor.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// WithActiveGuard agrega usuarios.RequireActive: con la cuenta en eliminación pendiente
// las rutas responden 403.
func (h *Handler) WithActiveGuard(guard fiber.Handler) *Handler {
	h.active = guard
	return h
}

// Routes registra GET /amistades/sugerencias y GET /lugares/:id/amigos. Cumple
// httpapi.Module; se monta antes que amistades para no pasar dos veces por su grupo.
func (h *Handler) Routes(v1 fiber.Router, _ *middleware.Limiter) {
	active := h.active
	if active == nil {
		active = func(c *fiber.Ctx) error { return c.Next() }
	}
	v1.Get("/amistades/sugerencias", middleware.RequireAuth(), active, h.suggestions)
	v1.Get("/lugares/:id/amigos", middleware.RequireAuth(), active, h.friendsAt)
}

func subject(c *fiber.Ctx) string {
	if claims, ok := middleware.ClaimsFrom(c); ok {
		return claims.Subject
	}
	return ""
}

func (h *Handler) suggestions(c *fiber.Ctx) error {
	out, err := h.svc.Suggestions(c.UserContext(), subject(c))
	if err != nil {
		return err
	}
	// Depende de quien pregunta: nunca en caché compartida.
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.JSON(out)
}

func (h *Handler) friendsAt(c *fiber.Ctx) error {
	out, err := h.svc.FriendsAt(c.UserContext(), subject(c), c.Params("id"))
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.JSON(out)
}

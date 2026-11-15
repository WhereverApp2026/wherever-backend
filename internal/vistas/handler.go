package vistas

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
	active    fiber.Handler
}

// NewHandler recibe el service y el validador por constructor.
func NewHandler(svc *Service, v *validation.Validator) *Handler {
	return &Handler{svc: svc, validator: v}
}

// WithActiveGuard agrega usuarios.RequireActive: con la cuenta en eliminación pendiente = 403.
func (h *Handler) WithActiveGuard(guard fiber.Handler) *Handler {
	h.active = guard
	return h
}

// Routes registra /lugares/recientes y POST /lugares/{id}/vistas, todo con JWT. Cumple
// httpapi.Module. Va antes que negocios: si no, `/lugares/:id` atiende «recientes». Sin
// política propia: el tope de 60 lugares por hora se cuenta en MongoDB (nv-66).
func (h *Handler) Routes(v1 fiber.Router, _ *middleware.Limiter) {
	chain := func(handler fiber.Handler) []fiber.Handler {
		out := []fiber.Handler{middleware.RequireAuth()}
		if h.active != nil {
			out = append(out, h.active)
		}
		return append(out, handler)
	}
	v1.Get("/lugares/recientes", chain(h.recientes)...)
	v1.Delete("/lugares/recientes", chain(h.borrar)...)
	v1.Post("/lugares/:id/vistas", chain(h.registrar)...)
}

func subject(c *fiber.Ctx) string {
	if claims, ok := middleware.ClaimsFrom(c); ok {
		return claims.Subject
	}
	return ""
}

func (h *Handler) registrar(c *fiber.Ctx) error {
	var req VistaRequest
	if len(c.Body()) > 0 {
		if err := c.BodyParser(&req); err != nil {
			return apperr.BadRequest("cuerpo JSON inválido")
		}
	}
	if err := h.validator.Struct(req); err != nil {
		return err
	}
	if err := h.svc.Registrar(c.UserContext(), subject(c), c.Params("id"), req); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) recientes(c *fiber.Ctx) error {
	q := RecientesQuery{Limit: DefaultLimit}
	if err := c.QueryParser(&q); err != nil {
		return apperr.BadRequest("parámetros inválidos")
	}
	if err := h.validator.Struct(q); err != nil {
		return err
	}
	out, err := h.svc.Recientes(c.UserContext(), subject(c), q)
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.JSON(out)
}

func (h *Handler) borrar(c *fiber.Ctx) error {
	if err := h.svc.Borrar(c.UserContext(), subject(c)); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

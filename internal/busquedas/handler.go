package busquedas

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

// WithActiveGuard agrega usuarios.RequireActive a las búsquedas recientes: con la cuenta en
// eliminación pendiente responden 403.
func (h *Handler) WithActiveGuard(guard fiber.Handler) *Handler {
	h.active = guard
	return h
}

// Routes registra /lugares/autocompletar, /lugares/buscar (auth opcional) y
// /busquedas/recientes (JWT). Cumple httpapi.Module. Va antes que el módulo de negocios: si
// no, `/lugares/:id` atiende «buscar».
func (h *Handler) Routes(v1 fiber.Router, _ *middleware.Limiter) {
	v1.Get("/lugares/autocompletar", h.autocompletar)
	v1.Get("/lugares/buscar", h.buscar)
	g := v1.Group("/busquedas", middleware.RequireAuth())
	chain := func(handler fiber.Handler) []fiber.Handler {
		if h.active == nil {
			return []fiber.Handler{handler}
		}
		return []fiber.Handler{h.active, handler}
	}
	g.Get("/recientes", chain(h.recientes)...)
	g.Delete("/recientes", chain(h.borrar)...)
}

func subject(c *fiber.Ctx) string {
	if claims, ok := middleware.ClaimsFrom(c); ok {
		return claims.Subject
	}
	return ""
}

func (h *Handler) autocompletar(c *fiber.Ctx) error {
	q := AutocompletarQuery{Limite: DefaultAutocompletar}
	if err := c.QueryParser(&q); err != nil {
		return apperr.BadRequest("parámetros inválidos")
	}
	if err := h.validator.Struct(q); err != nil {
		return err
	}
	out, err := h.svc.Autocompletar(c.UserContext(), q.Q, q.Limite)
	if err != nil {
		return err
	}
	// Lo que la persona escribe no se guarda en cachés compartidas.
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.JSON(out)
}

func (h *Handler) buscar(c *fiber.Ctx) error {
	q := BuscarQuery{Limite: DefaultBuscar}
	if err := c.QueryParser(&q); err != nil {
		return apperr.BadRequest("parámetros inválidos")
	}
	if err := h.validator.Struct(q); err != nil {
		return err
	}
	out, err := h.svc.Buscar(c.UserContext(), subject(c), q.Q, q.Limite)
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.JSON(out)
}

func (h *Handler) recientes(c *fiber.Ctx) error {
	out, err := h.svc.Recientes(c.UserContext(), subject(c))
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.JSON(out)
}

func (h *Handler) borrar(c *fiber.Ctx) error {
	if err := h.svc.BorrarRecientes(c.UserContext(), subject(c)); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

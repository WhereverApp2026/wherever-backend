package favoritos

import (
	"strings"

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

// WithActiveGuard agrega usuarios.RequireActive a las cinco rutas: con la cuenta en
// eliminación pendiente responden 403 (spec nv-54).
func (h *Handler) WithActiveGuard(guard fiber.Handler) *Handler {
	h.active = guard
	return h
}

// Routes registra /api/v1/favoritos, solo para el rol consumidor. Cumple httpapi.Module.
// Sin política de límite propia: aplican las globales.
func (h *Handler) Routes(v1 fiber.Router, _ *middleware.Limiter) {
	chain := []fiber.Handler{middleware.RequireRoles(middleware.RoleConsumer)}
	if h.active != nil {
		chain = append(chain, h.active)
	}
	g := v1.Group("/favoritos", chain...)
	g.Get("", h.list)
	g.Get("/ids", h.ids)
	g.Get("/buscar", h.search)
	g.Put("/:tipo/:objetivo_id", h.mark)
	g.Delete("/:tipo/:objetivo_id", h.unmark)
}

func subject(c *fiber.Ctx) string {
	if cl, ok := middleware.ClaimsFrom(c); ok {
		return cl.Subject
	}
	return ""
}

func (h *Handler) query(c *fiber.Ctx, dto any) error {
	if err := c.QueryParser(dto); err != nil {
		return apperr.BadRequest("parámetros inválidos")
	}
	return h.validator.Struct(dto)
}

func (h *Handler) params(c *fiber.Ctx) (TargetParams, error) {
	var p TargetParams
	if err := c.ParamsParser(&p); err != nil {
		return p, apperr.BadRequest("parámetros inválidos")
	}
	return p, h.validator.Struct(p)
}

func send(c *fiber.Ctx, v any, err error) error {
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.JSON(v)
}

func (h *Handler) list(c *fiber.Ctx) error {
	q := NewListQuery()
	if err := h.query(c, &q); err != nil {
		return err
	}
	out, err := h.svc.List(c.UserContext(), subject(c), q)
	return send(c, out, err)
}

func (h *Handler) ids(c *fiber.Ctx) error {
	var q IDsQuery
	if err := h.query(c, &q); err != nil {
		return err
	}
	out, err := h.svc.IDs(c.UserContext(), subject(c), q)
	return send(c, out, err)
}

func (h *Handler) search(c *fiber.Ctx) error {
	var q SearchQuery
	if err := c.QueryParser(&q); err != nil {
		return apperr.BadRequest("parámetros inválidos")
	}
	q.Q = strings.TrimSpace(q.Q)
	if err := h.validator.Struct(q); err != nil {
		return err
	}
	out, err := h.svc.Search(c.UserContext(), subject(c), q)
	return send(c, out, err)
}

func (h *Handler) mark(c *fiber.Ctx) error {
	p, err := h.params(c)
	if err != nil {
		return err
	}
	out, err := h.svc.Mark(c.UserContext(), subject(c), p)
	return send(c, out, err)
}

func (h *Handler) unmark(c *fiber.Ctx) error {
	p, err := h.params(c)
	if err != nil {
		return err
	}
	if err := h.svc.Unmark(c.UserContext(), subject(c), p); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

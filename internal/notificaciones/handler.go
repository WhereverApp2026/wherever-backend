package notificaciones

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

// WithActiveGuard agrega usuarios.RequireActive a las escrituras.
func (h *Handler) WithActiveGuard(guard fiber.Handler) *Handler {
	h.active = guard
	return h
}

// Routes registra /api/v1/dispositivos y /api/v1/notificaciones, todo con JWT. Cumple
// httpapi.Module. `no-leidas` y `leer-todas` van antes que `/:id`.
func (h *Handler) Routes(v1 fiber.Router, _ *middleware.Limiter) {
	d := v1.Group("/dispositivos", middleware.RequireAuth())
	d.Put("", h.write(h.register)...)
	d.Delete("", h.write(h.unregister)...)

	n := v1.Group("/notificaciones", middleware.RequireAuth())
	n.Get("", h.list)
	n.Get("/no-leidas", h.unread)
	n.Post("/leer-todas", h.write(h.readAll)...)
	n.Patch("/:id", h.write(h.mark)...)
	n.Delete("/:id", h.write(h.remove)...)
}

func (h *Handler) write(handler fiber.Handler) []fiber.Handler {
	if h.active == nil {
		return []fiber.Handler{handler}
	}
	return []fiber.Handler{h.active, handler}
}

func (h *Handler) bind(c *fiber.Ctx, dto any) error {
	if err := c.BodyParser(dto); err != nil {
		return apperr.BadRequest("cuerpo JSON inválido")
	}
	return h.validator.Struct(dto)
}

func subject(c *fiber.Ctx) string {
	if cl, ok := middleware.ClaimsFrom(c); ok {
		return cl.Subject
	}
	return ""
}

func noStore(c *fiber.Ctx, out any) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.JSON(out)
}

func (h *Handler) register(c *fiber.Ctx) error {
	var req DeviceRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	if err := h.svc.RegisterDevice(c.UserContext(), subject(c), req); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) unregister(c *fiber.Ctx) error {
	var req DeviceRemoveRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	if err := h.svc.RemoveDevice(c.UserContext(), subject(c), req); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) list(c *fiber.Ctx) error {
	q := NewListQuery()
	if err := c.QueryParser(&q); err != nil {
		return apperr.BadRequest("parámetros inválidos")
	}
	if err := h.validator.Struct(q); err != nil {
		return err
	}
	out, err := h.svc.List(c.UserContext(), subject(c), q)
	if err != nil {
		return err
	}
	return noStore(c, out)
}

func (h *Handler) unread(c *fiber.Ctx) error {
	out, err := h.svc.Unread(c.UserContext(), subject(c))
	if err != nil {
		return err
	}
	return noStore(c, out)
}

func (h *Handler) mark(c *fiber.Ctx) error {
	var req MarkRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Mark(c.UserContext(), subject(c), c.Params("id"), req)
	if err != nil {
		return err
	}
	return noStore(c, out)
}

func (h *Handler) readAll(c *fiber.Ctx) error {
	if err := h.svc.ReadAll(c.UserContext(), subject(c)); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) remove(c *fiber.Ctx) error {
	if err := h.svc.Delete(c.UserContext(), subject(c), c.Params("id")); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

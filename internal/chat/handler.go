package chat

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

// WithActiveGuard agrega usuarios.RequireActive a las escrituras (403 con la cuenta en
// eliminación pendiente); leer el chat sigue permitido, igual que leer el grupo.
func (h *Handler) WithActiveGuard(guard fiber.Handler) *Handler {
	h.active = guard
	return h
}

// Routes registra el chat. Cumple httpapi.Module. Va antes que grupos en la lista de
// módulos: así /grupos/:id/mensajes no pasa por el grupo /grupos. El límite de 30 mensajes
// por minuto se cuenta en MongoDB (service), no en Redis.
func (h *Handler) Routes(v1 fiber.Router, _ *middleware.Limiter) {
	auth := middleware.RequireAuth()
	v1.Get("/grupos/:id/mensajes", auth, h.list)
	v1.Post("/grupos/:id/mensajes", append([]fiber.Handler{auth}, h.write(h.send)...)...)
	v1.Put("/grupos/:id/mensajes/:mid/reacciones", append([]fiber.Handler{auth}, h.write(h.react)...)...)
	v1.Delete("/grupos/:id/mensajes/:mid", append([]fiber.Handler{auth}, h.write(h.remove)...)...)
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

func send(c *fiber.Ctx, status int, out any, err error) error {
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.Status(status).JSON(out)
}

func (h *Handler) list(c *fiber.Ctx) error {
	q := NewListQuery()
	if err := c.QueryParser(&q); err != nil {
		return apperr.BadRequest("parámetros inválidos")
	}
	if err := h.validator.Struct(q); err != nil {
		return err
	}
	out, err := h.svc.List(c.UserContext(), subject(c), c.Params("id"), q)
	return send(c, fiber.StatusOK, out, err)
}

func (h *Handler) send(c *fiber.Ctx) error {
	var req SendRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	v, created, err := h.svc.Send(c.UserContext(), subject(c), c.Params("id"), req)
	status := fiber.StatusOK
	if created {
		status = fiber.StatusCreated
	}
	return send(c, status, v, err)
}

func (h *Handler) react(c *fiber.Ctx) error {
	var req ReactRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	v, err := h.svc.React(c.UserContext(), subject(c), c.Params("id"), c.Params("mid"), req)
	return send(c, fiber.StatusOK, v, err)
}

func (h *Handler) remove(c *fiber.Ctx) error {
	if err := h.svc.Delete(c.UserContext(), subject(c), c.Params("id"), c.Params("mid")); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

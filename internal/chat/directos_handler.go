package chat

import (
	"github.com/gofiber/fiber/v2"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

// DirectHandler traduce HTTP ↔ DirectService (nv-87). No toca la base ni atrapa pánicos.
type DirectHandler struct {
	svc       *DirectService
	validator *validation.Validator
	active    fiber.Handler
}

// NewDirectHandler recibe el service y el validador por constructor.
func NewDirectHandler(svc *DirectService, v *validation.Validator) *DirectHandler {
	return &DirectHandler{svc: svc, validator: v}
}

// WithActiveGuard agrega usuarios.RequireActive a todas las rutas: con la cuenta en
// eliminación pendiente no se abren, leen ni escriben mensajes directos (403).
func (h *DirectHandler) WithActiveGuard(guard fiber.Handler) *DirectHandler {
	h.active = guard
	return h
}

// Routes registra /conversaciones. Cumple httpapi.Module. El límite de 120 mensajes por hora
// se cuenta en MongoDB (service), no en Redis.
func (h *DirectHandler) Routes(v1 fiber.Router, _ *middleware.Limiter) {
	v1.Post("/conversaciones", h.chain(h.open)...)
	v1.Get("/conversaciones", h.chain(h.list)...)
	v1.Get("/conversaciones/:id/mensajes", h.chain(h.messages)...)
	v1.Post("/conversaciones/:id/mensajes", h.chain(h.send)...)
	v1.Delete("/conversaciones/:id/mensajes/:mid", h.chain(h.remove)...)
	v1.Put("/conversaciones/:id/mensajes/:mid/reacciones", h.chain(h.react)...)
	v1.Post("/conversaciones/:id/leido", h.chain(h.read)...)
}

func (h *DirectHandler) chain(handler fiber.Handler) []fiber.Handler {
	out := []fiber.Handler{middleware.RequireAuth()}
	if h.active != nil {
		out = append(out, h.active)
	}
	return append(out, handler)
}

func (h *DirectHandler) bind(c *fiber.Ctx, dto any) error {
	if err := c.BodyParser(dto); err != nil {
		return apperr.BadRequest("cuerpo JSON inválido")
	}
	return h.validator.Struct(dto)
}

func created(c *fiber.Ctx, isNew bool, out any, err error) error {
	status := fiber.StatusOK
	if isNew {
		status = fiber.StatusCreated
	}
	return send(c, status, out, err)
}

func noContent(c *fiber.Ctx, err error) error {
	if err != nil {
		return err
	}
	c.Status(fiber.StatusNoContent)
	return nil
}

func (h *DirectHandler) open(c *fiber.Ctx) error {
	var req OpenRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	v, isNew, err := h.svc.Open(c.UserContext(), subject(c), req)
	return created(c, isNew, v, err)
}

func (h *DirectHandler) list(c *fiber.Ctx) error {
	out, err := h.svc.List(c.UserContext(), subject(c))
	return send(c, fiber.StatusOK, out, err)
}

func (h *DirectHandler) messages(c *fiber.Ctx) error {
	q := NewDirectListQuery()
	if err := c.QueryParser(&q); err != nil {
		return apperr.BadRequest("parámetros inválidos")
	}
	if err := h.validator.Struct(q); err != nil {
		return err
	}
	out, err := h.svc.Messages(c.UserContext(), subject(c), c.Params("id"), q)
	return send(c, fiber.StatusOK, out, err)
}

func (h *DirectHandler) send(c *fiber.Ctx) error {
	var req DirectSendRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	v, isNew, err := h.svc.Send(c.UserContext(), subject(c), c.Params("id"), req)
	return created(c, isNew, v, err)
}

func (h *DirectHandler) react(c *fiber.Ctx) error {
	var req ReactRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	v, err := h.svc.React(c.UserContext(), subject(c), c.Params("id"), c.Params("mid"), req)
	return send(c, fiber.StatusOK, v, err)
}

func (h *DirectHandler) remove(c *fiber.Ctx) error {
	return noContent(c, h.svc.Delete(c.UserContext(), subject(c), c.Params("id"), c.Params("mid")))
}

func (h *DirectHandler) read(c *fiber.Ctx) error {
	return noContent(c, h.svc.MarkRead(c.UserContext(), subject(c), c.Params("id")))
}

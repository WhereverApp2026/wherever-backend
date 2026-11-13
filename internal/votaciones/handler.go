package votaciones

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
// eliminación pendiente).
func (h *Handler) WithActiveGuard(guard fiber.Handler) *Handler {
	h.active = guard
	return h
}

// Routes registra las votaciones. Cumple httpapi.Module. Va antes que grupos en la lista de
// módulos: así /grupos/:id/votaciones no pasa por el grupo /grupos.
func (h *Handler) Routes(v1 fiber.Router, _ *middleware.Limiter) {
	auth := middleware.RequireAuth()
	v1.Post("/grupos/:id/votaciones", append([]fiber.Handler{auth}, h.write(h.open)...)...)
	v1.Get("/grupos/:id/votaciones", auth, h.list)
	g := v1.Group("/votaciones", auth)
	g.Put("/:id/voto", h.write(h.cast)...)
	g.Post("/:id/cerrar", h.write(h.close)...)
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

func (h *Handler) open(c *fiber.Ctx) error {
	var req OpenRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	v, err := h.svc.Open(c.UserContext(), subject(c), c.Params("id"), req)
	return send(c, fiber.StatusCreated, v, err)
}

func (h *Handler) list(c *fiber.Ctx) error {
	out, err := h.svc.List(c.UserContext(), subject(c), c.Params("id"))
	return send(c, fiber.StatusOK, out, err)
}

func (h *Handler) cast(c *fiber.Ctx) error {
	var req CastRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	v, err := h.svc.Cast(c.UserContext(), subject(c), c.Params("id"), req)
	return send(c, fiber.StatusOK, v, err)
}

func (h *Handler) close(c *fiber.Ctx) error {
	v, err := h.svc.Close(c.UserContext(), subject(c), c.Params("id"))
	return send(c, fiber.StatusOK, v, err)
}

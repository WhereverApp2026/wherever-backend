package amistades

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

// Límite por usuario de las solicitudes (nv-36, docs/rate-limit-matrix.md).
const (
	requestPolicy = "amistades_solicitud"
	requestLimit  = 20
	requestWindow = time.Hour
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

// WithActiveGuard agrega usuarios.RequireActive: con la cuenta en eliminación pendiente
// las rutas responden 403.
func (h *Handler) WithActiveGuard(guard fiber.Handler) *Handler {
	h.active = guard
	return h
}

// Routes registra /api/v1/amistades. Cumple httpapi.Module.
func (h *Handler) Routes(v1 fiber.Router, limiter *middleware.Limiter) {
	g := v1.Group("/amistades", middleware.RequireAuth())
	chain := func(handlers ...fiber.Handler) []fiber.Handler {
		if h.active == nil {
			return handlers
		}
		return append([]fiber.Handler{h.active}, handlers...)
	}
	send := []fiber.Handler{h.send}
	if limiter != nil {
		send = append([]fiber.Handler{limiter.WindowUser(requestPolicy, requestLimit, requestWindow)}, send...)
	}
	g.Post("", chain(send...)...)
	g.Get("", chain(h.list)...)
	g.Patch("/:id", chain(h.respond)...)
	g.Delete("/:id", chain(h.remove)...)
	g.Post("/:id/bloquear", chain(h.block)...)
}

func subject(c *fiber.Ctx) string {
	if claims, ok := middleware.ClaimsFrom(c); ok {
		return claims.Subject
	}
	return ""
}

// bind lee el JSON y lo valida: cuerpo ilegible → 400, reglas → 422.
func (h *Handler) bind(c *fiber.Ctx, dto any) error {
	if err := c.BodyParser(dto); err != nil {
		return apperr.BadRequest("cuerpo JSON inválido")
	}
	return h.validator.Struct(dto)
}

func (h *Handler) send(c *fiber.Ctx) error {
	var req SendRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Send(c.UserContext(), subject(c), req)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(out)
}

func (h *Handler) list(c *fiber.Ctx) error {
	var q ListQuery
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
	// Depende de quien pregunta: nunca en caché compartida.
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.JSON(out)
}

func (h *Handler) respond(c *fiber.Ctx) error {
	var req RespondRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Respond(c.UserContext(), subject(c), c.Params("id"), req)
	if err != nil {
		return err
	}
	return c.JSON(out)
}

func (h *Handler) remove(c *fiber.Ctx) error {
	if err := h.svc.Remove(c.UserContext(), subject(c), c.Params("id")); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) block(c *fiber.Ctx) error {
	if err := h.svc.Block(c.UserContext(), subject(c), c.Params("id")); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

package sincronizacion

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

// Handler traduce HTTP ↔ service. No toca la base ni atrapa pánicos (test de arquitectura).
type Handler struct {
	svc       *Service
	pending   *Pending
	validator *validation.Validator
	active    fiber.Handler
}

// Límite de lotes por usuario (nv-47): cada lote cuesta lo mismo en Redis, sin importar
// cuántas acciones traiga.
const (
	pendingPolicy = "sync_pendientes"
	pendingLimit  = 30
	pendingWindow = time.Minute
)

// WithPending agrega POST /sync/pendientes (nv-47) con el guardia de cuenta activa.
func (h *Handler) WithPending(p *Pending, active fiber.Handler) *Handler {
	h.pending, h.active = p, active
	return h
}

// NewHandler recibe el service y el validador por constructor.
func NewHandler(svc *Service, v *validation.Validator) *Handler {
	return &Handler{svc: svc, validator: v}
}

// Routes registra /api/v1/sync. Cumple httpapi.Module.
func (h *Handler) Routes(v1 fiber.Router, limiter *middleware.Limiter) {
	g := v1.Group("/sync", middleware.RequireAuth())
	g.Get("/cambios", h.changes)
	chain := []fiber.Handler{}
	if limiter != nil {
		chain = append(chain, limiter.WindowUser(pendingPolicy, pendingLimit, pendingWindow))
	}
	if h.active != nil {
		chain = append(chain, h.active)
	}
	g.Post("/pendientes", append(chain, h.applyPending)...)
}

func (h *Handler) applyPending(c *fiber.Ctx) error {
	claims, ok := middleware.ClaimsFrom(c)
	if !ok {
		return apperr.Unauthorized("autenticación requerida")
	}
	var req PendingRequest
	if err := c.BodyParser(&req); err != nil {
		return apperr.BadRequest("cuerpo JSON inválido")
	}
	if err := h.validator.Struct(req); err != nil {
		return err
	}
	out, err := h.pending.Apply(c.UserContext(), claims, req)
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.JSON(out)
}

func (h *Handler) changes(c *fiber.Ctx) error {
	q := NewChangesQuery()
	if err := c.QueryParser(&q); err != nil {
		return apperr.BadRequest("parámetros inválidos")
	}
	if err := h.validator.Struct(q); err != nil {
		return err
	}
	subject := ""
	if cl, ok := middleware.ClaimsFrom(c); ok {
		subject = cl.Subject
	}
	out, err := h.svc.Changes(c.UserContext(), subject, q)
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.JSON(out)
}

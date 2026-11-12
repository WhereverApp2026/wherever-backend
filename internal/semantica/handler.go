package semantica

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

// Política de la ruta (docs/rate-limit-matrix.md): por usuario o, sin token, por IP.
const (
	politica      = "semantica"
	limitePorMin  = 30
	ventanaLimite = time.Minute
)

// Handler traduce HTTP ↔ service.
type Handler struct {
	svc       *Service
	validator *validation.Validator
}

// NewHandler recibe el service y el validador por constructor.
func NewHandler(svc *Service, v *validation.Validator) *Handler {
	return &Handler{svc: svc, validator: v}
}

// Routes registra GET /api/v1/lugares/semantica con auth opcional. Cumple httpapi.Module.
// Va antes que el módulo de negocios: si no, `/lugares/:id` atiende «semantica».
func (h *Handler) Routes(v1 fiber.Router, limiter *middleware.Limiter) {
	v1.Get("/lugares/semantica", limiter.WindowUser(politica, limitePorMin, ventanaLimite), h.buscar)
}

func (h *Handler) buscar(c *fiber.Ctx) error {
	var q Query
	if err := c.QueryParser(&q); err != nil {
		return apperr.BadRequest("parámetros inválidos")
	}
	q.Q = strings.TrimSpace(q.Q)
	if err := h.validator.Struct(q); err != nil {
		return err
	}
	out, err := h.svc.Buscar(c.UserContext(), q.Q, q.Limite)
	if err != nil {
		return err
	}
	// La frase es de la persona: ni el navegador ni un proxy guardan la respuesta.
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.JSON(out)
}

package personas

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
)

// Límite por usuario de la búsqueda (nv-35, docs/rate-limit-matrix.md).
const (
	searchPolicy = "personas_buscar"
	searchLimit  = 30
	searchWindow = time.Hour
	// Contra la enumeración de códigos de amigo (nv-64).
	codePolicy = "personas_codigo"
	codeLimit  = 30
	codeWindow = time.Hour
)

// Handler traduce HTTP ↔ service.
type Handler struct {
	svc    *Service
	active fiber.Handler
}

// NewHandler recibe el service por constructor; `q` se valida en ParseQuery.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// WithActiveGuard agrega usuarios.RequireActive: con la cuenta en eliminación pendiente
// la búsqueda responde 403.
func (h *Handler) WithActiveGuard(guard fiber.Handler) *Handler {
	h.active = guard
	return h
}

// Routes registra /api/v1/personas. Cumple httpapi.Module.
func (h *Handler) Routes(v1 fiber.Router, limiter *middleware.Limiter) {
	chain := []fiber.Handler{middleware.RequireAuth()}
	if h.active != nil {
		chain = append(chain, h.active)
	}
	limited := func(policy string, limit int, window time.Duration, handler fiber.Handler) []fiber.Handler {
		out := append([]fiber.Handler{}, chain...)
		if limiter != nil {
			out = append(out, limiter.WindowUser(policy, limit, window))
		}
		return append(out, handler)
	}
	v1.Get("/personas/buscar", limited(searchPolicy, searchLimit, searchWindow, h.search)...)
	v1.Get("/personas/codigo/:codigo", limited(codePolicy, codeLimit, codeWindow, h.byCode)...)
}

func subject(c *fiber.Ctx) string {
	if claims, ok := middleware.ClaimsFrom(c); ok {
		return claims.Subject
	}
	return ""
}

func (h *Handler) byCode(c *fiber.Ctx) error {
	out, err := h.svc.ByFriendCode(c.UserContext(), subject(c), c.Params("codigo"))
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.JSON(out)
}

func (h *Handler) search(c *fiber.Ctx) error {
	out, err := h.svc.Search(c.UserContext(), subject(c), c.Query("q"))
	if err != nil {
		return err
	}
	// Depende de quien busca: nunca en caché compartida.
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.JSON(out)
}

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
	if limiter != nil {
		chain = append(chain, limiter.WindowUser(searchPolicy, searchLimit, searchWindow))
	}
	v1.Get("/personas/buscar", append(chain, h.search)...)
}

func (h *Handler) search(c *fiber.Ctx) error {
	subject := ""
	if claims, ok := middleware.ClaimsFrom(c); ok {
		subject = claims.Subject
	}
	out, err := h.svc.Search(c.UserContext(), subject, c.Query("q"))
	if err != nil {
		return err
	}
	// Depende de quien busca: nunca en caché compartida.
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.JSON(out)
}

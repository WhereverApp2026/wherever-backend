package configuracion

import (
	"github.com/gofiber/fiber/v2"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/etag"
)

// Handler traduce HTTP ↔ service.
type Handler struct {
	svc *Service
}

// NewHandler recibe el service por constructor.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Routes registra /api/v1/configuracion, sin autenticación. Cumple httpapi.Module.
func (h *Handler) Routes(v1 fiber.Router, _ *middleware.Limiter) {
	v1.Get("/configuracion", h.get)
}

func (h *Handler) get(c *fiber.Ctx) error {
	c.Set(fiber.HeaderCacheControl, "no-cache")
	c.Set(fiber.HeaderETag, h.svc.ETag())
	if etag.Coincide(c.Get(fiber.HeaderIfNoneMatch), h.svc.ETag()) {
		return c.SendStatus(fiber.StatusNotModified)
	}
	c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
	return c.Send(h.svc.Cuerpo())
}

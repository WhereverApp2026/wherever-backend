package imagenes

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

// Límites por usuario de las subidas (nv-45, docs/rate-limit-matrix.md): 20 por hora de la
// spec y 50 por día para acotar el costo de Rekognition (US$0.001 por imagen).
const (
	politicaSubida    = "imagenes_subida"
	limiteSubida      = 20
	ventanaSubida     = time.Hour
	politicaSubidaDia = "imagenes_subida_dia"
	limiteSubidaDia   = 50
	ventanaSubidaDia  = 24 * time.Hour
)

// Handler traduce HTTP ↔ service. Con svc nil (sin IMAGENES_BUCKET e IMAGENES_URL) las rutas
// responden 503 después de la autenticación.
type Handler struct {
	svc       *Service
	validator *validation.Validator
	active    fiber.Handler
}

// NewHandler recibe el service (puede ser nil) y el validador por constructor.
func NewHandler(svc *Service, v *validation.Validator) *Handler {
	return &Handler{svc: svc, validator: v}
}

// WithActiveGuard agrega usuarios.RequireActive: con la cuenta en eliminación pendiente
// las rutas responden 403.
func (h *Handler) WithActiveGuard(guard fiber.Handler) *Handler {
	h.active = guard
	return h
}

// Routes registra /api/v1/imagenes. Cumple httpapi.Module.
func (h *Handler) Routes(v1 fiber.Router, limiter *middleware.Limiter) {
	g := v1.Group("/imagenes", middleware.RequireAuth())
	chain := func(handlers ...fiber.Handler) []fiber.Handler {
		pre := []fiber.Handler{h.disponible}
		if h.active != nil {
			pre = append([]fiber.Handler{h.active}, pre...)
		}
		return append(pre, handlers...)
	}
	subir := []fiber.Handler{h.subir}
	if limiter != nil {
		subir = append([]fiber.Handler{
			limiter.WindowUser(politicaSubida, limiteSubida, ventanaSubida),
			limiter.WindowUser(politicaSubidaDia, limiteSubidaDia, ventanaSubidaDia),
		}, subir...)
	}
	g.Post("/subida", chain(subir...)...)
	g.Post("/:id/confirmar", chain(h.confirmar)...)
	g.Delete("/:id", chain(h.eliminar)...)
}

func (h *Handler) disponible(c *fiber.Ctx) error {
	if h.svc == nil {
		return apperr.Unavailable("las imágenes no están configuradas")
	}
	return c.Next()
}

func subject(c *fiber.Ctx) string {
	if claims, ok := middleware.ClaimsFrom(c); ok {
		return claims.Subject
	}
	return ""
}

func (h *Handler) subir(c *fiber.Ctx) error {
	var req SubidaRequest
	if err := c.BodyParser(&req); err != nil {
		return apperr.BadRequest("cuerpo JSON inválido")
	}
	if err := h.validator.Struct(req); err != nil {
		return err
	}
	out, err := h.svc.Subir(c.UserContext(), subject(c), req)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(out)
}

func (h *Handler) confirmar(c *fiber.Ctx) error {
	out, err := h.svc.Confirmar(c.UserContext(), subject(c), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(out)
}

func (h *Handler) eliminar(c *fiber.Ctx) error {
	if err := h.svc.Eliminar(c.UserContext(), subject(c), c.Params("id")); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

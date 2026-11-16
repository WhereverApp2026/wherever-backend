package estadisticas

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

// WithActiveGuard agrega usuarios.RequireActive a las rutas del dueño.
func (h *Handler) WithActiveGuard(guard fiber.Handler) *Handler {
	h.active = guard
	return h
}

// Routes registra las estadísticas y el reporte del dueño (comerciante). Cumple httpapi.Module.
// Solo la política global de límites: cada GET cuesta lo de una lectura con sesión.
func (h *Handler) Routes(v1 fiber.Router, _ *middleware.Limiter) {
	dueno := []fiber.Handler{middleware.RequireRoles(middleware.RoleMerchant)}
	if h.active != nil {
		dueno = append(dueno, h.active)
	}
	with := func(fn fiber.Handler) []fiber.Handler { return append(append([]fiber.Handler{}, dueno...), fn) }
	v1.Get("/negocios/:id/estadisticas", with(h.estadisticas)...)
	v1.Get("/negocios/:id/reportes/mensual", with(h.mensual)...)
}

func (h *Handler) query(c *fiber.Ctx, dto any) error {
	if err := c.QueryParser(dto); err != nil {
		return apperr.BadRequest("parámetros inválidos")
	}
	return h.validator.Struct(dto)
}

func claims(c *fiber.Ctx) *middleware.Claims {
	cl, _ := middleware.ClaimsFrom(c)
	return cl
}

func (h *Handler) estadisticas(c *fiber.Ctx) error {
	var q EstadisticasQuery
	if err := h.query(c, &q); err != nil {
		return err
	}
	out, err := h.svc.Estadisticas(c.UserContext(), claims(c), c.Params("id"), q)
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.Status(fiber.StatusOK).JSON(out)
}

func (h *Handler) mensual(c *fiber.Ctx) error {
	var q ReporteQuery
	if err := h.query(c, &q); err != nil {
		return err
	}
	archivo, nombre, err := h.svc.Reporte(c.UserContext(), claims(c), c.Params("id"), q)
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	c.Set(fiber.HeaderContentType, "application/pdf")
	c.Set(fiber.HeaderContentDisposition, `attachment; filename="`+nombre+`"`)
	return c.Status(fiber.StatusOK).Send(archivo)
}

package clima

import (
	"github.com/gofiber/fiber/v2"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

// Query es `?lat=&lng=` o `?ciudad_id=`; con las dos, mandan las coordenadas.
type Query struct {
	Lat      *float64 `query:"lat" validate:"omitempty,latitude"`
	Lng      *float64 `query:"lng" validate:"omitempty,longitude"`
	CiudadID string   `query:"ciudad_id" validate:"omitempty,objectid"`
}

// Handler traduce HTTP ↔ Servicio.
type Handler struct {
	svc       *Servicio
	ciudades  Ciudades
	validator *validation.Validator
}

// NewHandler recibe el servicio, las ciudades y el validador por constructor.
func NewHandler(svc *Servicio, c Ciudades, v *validation.Validator) *Handler {
	return &Handler{svc: svc, ciudades: c, validator: v}
}

// Routes registra GET /api/v1/contexto/clima, pública. Cumple httpapi.Module.
func (h *Handler) Routes(v1 fiber.Router, _ *middleware.Limiter) {
	v1.Get("/contexto/clima", h.actual)
}

func (h *Handler) actual(c *fiber.Ctx) error {
	var q Query
	if err := c.QueryParser(&q); err != nil {
		return apperr.BadRequest("parámetros inválidos")
	}
	if err := h.validator.Struct(q); err != nil {
		return err
	}
	lat, lng, err := coordenadas(c.UserContext(), h.ciudades, q)
	if err != nil {
		return err
	}
	// No depende del usuario, pero la coordenada es una ubicación: solo caché privada.
	c.Set(fiber.HeaderCacheControl, "private, max-age=600")
	return c.JSON(h.svc.Actual(c.UserContext(), lat, lng))
}

package reportesnegocio

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

// Política de las rutas admin (docs/rate-limit-matrix.md), la misma de nv-43 y nv-55.
const (
	adminPolicy = "admin"
	adminLimit  = 200
	adminWindow = time.Minute
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

// WithActiveGuard agrega usuarios.RequireActive a crear un reporte.
func (h *Handler) WithActiveGuard(guard fiber.Handler) *Handler {
	h.active = guard
	return h
}

// Routes registra POST /lugares/{id}/reportes y /admin/reportes-negocio. Cumple
// httpapi.Module. El reporte no tiene política propia: el tope diario se cuenta en MongoDB.
func (h *Handler) Routes(v1 fiber.Router, limiter *middleware.Limiter) {
	crear := []fiber.Handler{middleware.RequireAuth()}
	if h.active != nil {
		crear = append(crear, h.active)
	}
	v1.Post("/lugares/:id/reportes", append(crear, h.crear)...)
	admin := []fiber.Handler{}
	if limiter != nil {
		admin = append(admin, limiter.Window(adminPolicy, adminLimit, adminWindow))
	}
	admin = append(admin, middleware.RequireRoles(middleware.RoleAdmin))
	g := v1.Group("/admin/reportes-negocio", admin...)
	g.Get("", h.cola)
	g.Patch("/:id", h.resolver)
}

func subject(c *fiber.Ctx) string {
	if claims, ok := middleware.ClaimsFrom(c); ok {
		return claims.Subject
	}
	return ""
}

// crear decodifica en estricto: campo de más = 400.
func (h *Handler) crear(c *fiber.Ctx) error {
	var req CrearRequest
	dec := json.NewDecoder(bytes.NewReader(c.Body()))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil || dec.More() {
		return apperr.BadRequest("cuerpo JSON inválido")
	}
	if err := h.validator.Struct(req); err != nil {
		return err
	}
	out, err := h.svc.Crear(c.UserContext(), subject(c), c.Params("id"), req)
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.Status(fiber.StatusCreated).JSON(out)
}

func (h *Handler) cola(c *fiber.Ctx) error {
	q := ColaQuery{Limit: pagination.DefaultLimit}
	if err := c.QueryParser(&q); err != nil {
		return apperr.BadRequest("parámetros inválidos")
	}
	if err := h.validator.Struct(q); err != nil {
		return err
	}
	out, err := h.svc.Cola(c.UserContext(), q)
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.JSON(out)
}

// ifMatch lee la versión de `If-Match` (`3`, `"3"` o `W/"3"`). Falta o mal formada = 400.
func ifMatch(c *fiber.Ctx) (int, error) {
	raw := strings.Trim(strings.TrimPrefix(strings.TrimSpace(c.Get(fiber.HeaderIfMatch)), "W/"), `"`)
	v, err := strconv.Atoi(raw)
	if err != nil || v < 1 {
		return 0, apperr.BadRequest("falta la cabecera If-Match con la versión del reporte")
	}
	return v, nil
}

func (h *Handler) resolver(c *fiber.Ctx) error {
	version, err := ifMatch(c)
	if err != nil {
		return err
	}
	var req ResolverRequest
	if err := c.BodyParser(&req); err != nil {
		return apperr.BadRequest("cuerpo JSON inválido")
	}
	if err := h.validator.Struct(req); err != nil {
		return err
	}
	out, err := h.svc.Resolver(c.UserContext(), subject(c), c.Params("id"), version, req)
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderETag, `"`+strconv.Itoa(out.Version)+`"`)
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.JSON(out)
}

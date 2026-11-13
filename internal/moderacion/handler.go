package moderacion

import (
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

// Política de las rutas admin (docs/rate-limit-matrix.md), la misma de la semilla de nv-20.
const (
	adminPolicy = "admin"
	adminLimit  = 200
	adminWindow = time.Minute
)

// Handler traduce HTTP ↔ service. No toca la base ni atrapa pánicos (test de arquitectura).
type Handler struct {
	svc       *Service
	validator *validation.Validator
}

// NewHandler recibe el service y el validador por constructor.
func NewHandler(svc *Service, v *validation.Validator) *Handler {
	return &Handler{svc: svc, validator: v}
}

// Routes registra /api/v1/admin/moderacion. Cumple httpapi.Module. `/reglas` va antes de
// `/:id`.
func (h *Handler) Routes(v1 fiber.Router, limiter *middleware.Limiter) {
	g := v1.Group("/admin/moderacion",
		limiter.Window(adminPolicy, adminLimit, adminWindow),
		middleware.RequireRoles(middleware.RoleAdmin))
	g.Get("", h.listar)
	g.Get("/reglas", h.verReglas)
	g.Post("/reglas", h.crearReglas)
	g.Patch("/reglas", h.ajustarReglas)
	g.Get("/:id", h.detalle)
	g.Patch("/:id", h.decidir)
}

func subject(c *fiber.Ctx) string {
	if cl, ok := middleware.ClaimsFrom(c); ok {
		return cl.Subject
	}
	return ""
}

// ifMatch lee la versión de `If-Match` (`3`, `"3"` o `W/"3"`). Falta o mal formada = 400.
func ifMatch(c *fiber.Ctx) (int, error) {
	raw := strings.Trim(strings.TrimPrefix(strings.TrimSpace(c.Get(fiber.HeaderIfMatch)), "W/"), `"`)
	version, err := strconv.Atoi(raw)
	if err != nil || version < 1 {
		return 0, apperr.BadRequest("falta la cabecera If-Match con la versión")
	}
	return version, nil
}

func (h *Handler) bind(c *fiber.Ctx, dto any) error {
	if err := c.BodyParser(dto); err != nil {
		return apperr.BadRequest("cuerpo JSON inválido")
	}
	return h.validator.Struct(dto)
}

func send(c *fiber.Ctx, status, version int, out any, err error) error {
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	if version > 0 {
		c.Set(fiber.HeaderETag, `"`+strconv.Itoa(version)+`"`)
	}
	return c.Status(status).JSON(out)
}

func (h *Handler) listar(c *fiber.Ctx) error {
	q := NuevaListaQuery()
	if err := c.QueryParser(&q); err != nil {
		return apperr.BadRequest("parámetros inválidos")
	}
	if err := h.validator.Struct(q); err != nil {
		return err
	}
	out, err := h.svc.Listar(c.UserContext(), q)
	return send(c, fiber.StatusOK, 0, out, err)
}

func (h *Handler) detalle(c *fiber.Ctx) error {
	d, err := h.svc.Detalle(c.UserContext(), c.Params("id"))
	return send(c, fiber.StatusOK, d.Version, d, err)
}

func (h *Handler) decidir(c *fiber.Ctx) error {
	version, err := ifMatch(c)
	if err != nil {
		return err
	}
	var req DecisionRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	d, err := h.svc.Decidir(c.UserContext(), subject(c), c.Params("id"), version, req)
	return send(c, fiber.StatusOK, d.Version, d, err)
}

func (h *Handler) verReglas(c *fiber.Ctx) error {
	r, err := h.svc.VerReglas(c.UserContext())
	return send(c, fiber.StatusOK, r.Version, r, err)
}

func (h *Handler) crearReglas(c *fiber.Ctx) error {
	var req ReglasRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	r, err := h.svc.CrearReglas(c.UserContext(), subject(c), req)
	return send(c, fiber.StatusCreated, r.Version, r, err)
}

func (h *Handler) ajustarReglas(c *fiber.Ctx) error {
	version, err := ifMatch(c)
	if err != nil {
		return err
	}
	var req ReglasPatch
	if err := h.bind(c, &req); err != nil {
		return err
	}
	r, err := h.svc.AjustarReglas(c.UserContext(), subject(c), version, req)
	return send(c, fiber.StatusOK, r.Version, r, err)
}

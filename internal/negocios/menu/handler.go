package menu

import (
	"strconv"
	"strings"

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

// Routes registra el menú del dueño y el público. Cumple httpapi.Module; va antes que el
// catálogo para que `/lugares/:id` no atienda `/lugares/:id/menu`.
func (h *Handler) Routes(v1 fiber.Router, _ *middleware.Limiter) {
	dueno := []fiber.Handler{middleware.RequireRoles(middleware.RoleMerchant)}
	if h.active != nil {
		dueno = append(dueno, h.active)
	}
	with := func(fn fiber.Handler) []fiber.Handler { return append(append([]fiber.Handler{}, dueno...), fn) }
	v1.Get("/lugares/:id/menu", h.publico)
	v1.Post("/negocios/:id/menu", with(h.crear)...)
	v1.Patch("/negocios/:id/menu/:item_id", with(h.editar)...)
	v1.Delete("/negocios/:id/menu/:item_id", with(h.borrar)...)
}

func claims(c *fiber.Ctx) *middleware.Claims {
	cl, _ := middleware.ClaimsFrom(c)
	return cl
}

func (h *Handler) bind(c *fiber.Ctx, dto any) error {
	if err := c.BodyParser(dto); err != nil {
		return apperr.BadRequest("cuerpo JSON inválido")
	}
	return h.validator.Struct(dto)
}

// ifMatch lee la versión de `If-Match` (`3`, `"3"` o `W/"3"`). Falta o mal formada = 400.
func ifMatch(c *fiber.Ctx) (int, error) {
	raw := strings.Trim(strings.TrimPrefix(strings.TrimSpace(c.Get(fiber.HeaderIfMatch)), "W/"), `"`)
	v, err := strconv.Atoi(raw)
	if raw == "" || err != nil || v < 1 {
		return 0, apperr.BadRequest("falta la cabecera If-Match con la versión")
	}
	return v, nil
}

func (h *Handler) publico(c *fiber.Ctx) error {
	var q MenuQuery
	if err := c.QueryParser(&q); err != nil {
		return apperr.BadRequest("parámetros inválidos")
	}
	if err := h.validator.Struct(q); err != nil {
		return err
	}
	out, err := h.svc.Publico(c.UserContext(), c.Params("id"), q)
	if err != nil {
		return err
	}
	return c.JSON(out)
}

func (h *Handler) crear(c *fiber.Ctx) error {
	var req CrearRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Crear(c.UserContext(), claims(c), c.Params("id"), req)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(out)
}

func (h *Handler) editar(c *fiber.Ctx) error {
	version, err := ifMatch(c)
	if err != nil {
		return err
	}
	var req EditarRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Editar(c.UserContext(), claims(c), c.Params("id"), c.Params("item_id"), version, req)
	if err != nil {
		return err
	}
	return c.JSON(out)
}

func (h *Handler) borrar(c *fiber.Ctx) error {
	version, err := ifMatch(c)
	if err != nil {
		return err
	}
	if err := h.svc.Borrar(c.UserContext(), claims(c), c.Params("id"), c.Params("item_id"), version); err != nil {
		return err
	}
	c.Status(fiber.StatusNoContent)
	return nil
}

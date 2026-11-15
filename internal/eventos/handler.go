package eventos

import (
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

// Publicar: 10 por día por usuario (nv-65, docs/rate-limit-matrix.md).
const (
	publicarPolicy = "eventos_publicar"
	publicarLimit  = 10
	publicarWindow = 24 * time.Hour
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

// WithActiveGuard agrega usuarios.RequireActive a las escrituras.
func (h *Handler) WithActiveGuard(guard fiber.Handler) *Handler {
	h.active = guard
	return h
}

// Routes registra /api/v1/eventos. Las lecturas son públicas (con sesión opcional para ver lo
// propio); las escrituras piden sesión y cuenta activa. Cumple httpapi.Module.
func (h *Handler) Routes(v1 fiber.Router, limiter *middleware.Limiter) {
	v1.Get("/eventos", h.lista)
	v1.Get("/eventos/:id", h.detalle)
	publicar := []fiber.Handler{middleware.RequireAuth(), middleware.RequireRoles(middleware.RoleAdmin, middleware.RoleMerchant)}
	publicar = append(publicar, h.write()...)
	if limiter != nil {
		publicar = append(publicar, limiter.WindowUser(publicarPolicy, publicarLimit, publicarWindow))
	}
	v1.Post("/eventos", append(publicar, h.crear)...)
	editar := append([]fiber.Handler{middleware.RequireAuth()}, h.write()...)
	v1.Patch("/eventos/:id", append(editar, h.editar)...)
	v1.Delete("/eventos/:id", append(editar, h.borrar)...)
}

func (h *Handler) write() []fiber.Handler {
	if h.active == nil {
		return nil
	}
	return []fiber.Handler{h.active}
}

func claims(c *fiber.Ctx) *middleware.Claims {
	cl, ok := middleware.ClaimsFrom(c)
	if !ok {
		return nil
	}
	return cl
}

// ifMatch lee la versión de `If-Match` (`3`, `"3"` o `W/"3"`). Falta o mal formada = 400.
func ifMatch(c *fiber.Ctx) (int, error) {
	raw := strings.Trim(strings.TrimPrefix(strings.TrimSpace(c.Get(fiber.HeaderIfMatch)), "W/"), `"`)
	v, err := strconv.Atoi(raw)
	if err != nil || v < 1 {
		return 0, apperr.BadRequest("falta la cabecera If-Match con la versión del evento")
	}
	return v, nil
}

func (h *Handler) bind(c *fiber.Ctx, dto any) error {
	if err := c.BodyParser(dto); err != nil {
		return apperr.BadRequest("cuerpo JSON inválido")
	}
	return h.validator.Struct(dto)
}

func send(c *fiber.Ctx, status int, v EventoView) error {
	c.Set(fiber.HeaderETag, `"`+strconv.Itoa(v.Version)+`"`)
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.Status(status).JSON(v)
}

func (h *Handler) lista(c *fiber.Ctx) error {
	q := NuevaListaQuery()
	if err := c.QueryParser(&q); err != nil {
		return apperr.BadRequest("parámetros inválidos")
	}
	if err := h.validator.Struct(q); err != nil {
		return err
	}
	out, err := h.svc.Lista(c.UserContext(), q)
	if err != nil {
		return err
	}
	return c.JSON(out)
}

func (h *Handler) detalle(c *fiber.Ctx) error {
	v, err := h.svc.Detalle(c.UserContext(), claims(c), c.Params("id"))
	if err != nil {
		return err
	}
	return send(c, fiber.StatusOK, v)
}

func (h *Handler) crear(c *fiber.Ctx) error {
	var req CrearRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	v, err := h.svc.Crear(c.UserContext(), claims(c), req)
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderLocation, "/api/v1/eventos/"+v.ID)
	return send(c, fiber.StatusCreated, v)
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
	v, err := h.svc.Editar(c.UserContext(), claims(c), c.Params("id"), version, req)
	if err != nil {
		return err
	}
	return send(c, fiber.StatusOK, v)
}

func (h *Handler) borrar(c *fiber.Ctx) error {
	version, err := ifMatch(c)
	if err != nil {
		return err
	}
	if err := h.svc.Borrar(c.UserContext(), claims(c), c.Params("id"), version); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

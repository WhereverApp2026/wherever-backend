package cupones

import (
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

// Límites por usuario de docs/rate-limit-matrix.md (ventana fija, `rate:{politica}:{usuario}`).
type politica struct {
	nombre  string
	limite  int
	ventana time.Duration
}

var (
	politicaCrear   = politica{"cupones_crear", 20, 24 * time.Hour}
	politicaGuardar = politica{"cupones_guardar", 30, time.Hour}
	politicaCanjear = politica{"cupones_canjear", 10, time.Hour}
	politicaValidar = politica{"cupones_validar", 60, 10 * time.Minute}
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

// WithActiveGuard agrega usuarios.RequireActive: con la cuenta en eliminación pendiente las
// rutas con token responden 403.
func (h *Handler) WithActiveGuard(guard fiber.Handler) *Handler {
	h.active = guard
	return h
}

// Routes registra las trece rutas de nv-61. Cumple httpapi.Module.
func (h *Handler) Routes(v1 fiber.Router, limiter *middleware.Limiter) {
	with := func(p *politica, first fiber.Handler, fn fiber.Handler) []fiber.Handler {
		out := []fiber.Handler{}
		if p != nil && limiter != nil {
			out = append(out, limiter.WindowUser(p.nombre, p.limite, p.ventana))
		}
		out = append(out, first)
		if h.active != nil {
			out = append(out, h.active)
		}
		return append(out, fn)
	}
	consumidor := middleware.RequireRoles(middleware.RoleConsumer)
	dueno := middleware.RequireRoles(middleware.RoleMerchant)

	v1.Post("/negocios/:id/cupones/sugerir-titulo", with(nil, dueno, h.sugerir)...)
	v1.Post("/negocios/:id/cupones/validar", with(&politicaValidar, dueno, h.validar)...)
	v1.Post("/negocios/:id/cupones", with(&politicaCrear, dueno, h.crear)...)
	v1.Get("/negocios/:id/cupones", with(nil, dueno, h.lista)...)
	v1.Get("/negocios/:id/cupones/:cid/estadisticas", with(nil, dueno, h.estadisticas)...)
	v1.Patch("/negocios/:id/cupones/:cid", with(nil, dueno, h.editar)...)
	v1.Delete("/negocios/:id/cupones/:cid", with(nil, dueno, h.borrar)...)
	v1.Get("/cupones/cerca", h.cerca)
	v1.Get("/cupones/:id", h.detalle)
	v1.Post("/cupones/:id/guardar", with(&politicaGuardar, consumidor, h.guardar)...)
	v1.Get("/mis-cupones", with(nil, consumidor, h.mias)...)
	v1.Post("/mis-cupones/:id/canjear", with(&politicaCanjear, consumidor, h.canjear)...)
	v1.Delete("/mis-cupones/:id", with(nil, consumidor, h.quitar)...)
}

func claims(c *fiber.Ctx) *middleware.Claims {
	cl, _ := middleware.ClaimsFrom(c)
	return cl
}

// bind lee el JSON y lo valida: cuerpo ilegible → 400, reglas → 422.
func (h *Handler) bind(c *fiber.Ctx, dto any) error {
	if err := c.BodyParser(dto); err != nil {
		return apperr.BadRequest("cuerpo JSON inválido")
	}
	return h.validator.Struct(dto)
}

func (h *Handler) query(c *fiber.Ctx, dto any) error {
	if err := c.QueryParser(dto); err != nil {
		return apperr.BadRequest("parámetros inválidos")
	}
	return h.validator.Struct(dto)
}

// ifMatch lee la versión de `If-Match` (`3`, `"3"` o `W/"3"`). Falta o mal formada = 400.
func ifMatch(c *fiber.Ctx) (int, error) {
	raw := strings.Trim(strings.TrimPrefix(strings.TrimSpace(c.Get(fiber.HeaderIfMatch)), "W/"), `"`)
	v, err := strconv.Atoi(raw)
	if err != nil || v < 1 {
		return 0, apperr.BadRequest("falta la cabecera If-Match con la versión del cupón")
	}
	return v, nil
}

func send(c *fiber.Ctx, status int, v any) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.Status(status).JSON(v)
}

func sendCupon(c *fiber.Ctx, status int, out CuponResponse) error {
	if out.Version != nil {
		c.Set(fiber.HeaderETag, `"`+strconv.Itoa(*out.Version)+`"`)
	}
	return send(c, status, out)
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
	return sendCupon(c, fiber.StatusCreated, out)
}

func (h *Handler) lista(c *fiber.Ctx) error {
	q := NewListaNegocioQuery()
	if err := h.query(c, &q); err != nil {
		return err
	}
	out, err := h.svc.Lista(c.UserContext(), claims(c), c.Params("id"), q)
	if err != nil {
		return err
	}
	return send(c, fiber.StatusOK, out)
}

func (h *Handler) estadisticas(c *fiber.Ctx) error {
	var q EstadisticasQuery
	if err := h.query(c, &q); err != nil {
		return err
	}
	out, err := h.svc.Estadisticas(c.UserContext(), claims(c), c.Params("id"), c.Params("cid"), q)
	if err != nil {
		return err
	}
	return send(c, fiber.StatusOK, out)
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
	out, err := h.svc.Editar(c.UserContext(), claims(c), c.Params("id"), c.Params("cid"), version, req)
	if err != nil {
		return err
	}
	return sendCupon(c, fiber.StatusOK, out)
}

func (h *Handler) borrar(c *fiber.Ctx) error {
	version, err := ifMatch(c)
	if err != nil {
		return err
	}
	if err := h.svc.Borrar(c.UserContext(), claims(c), c.Params("id"), c.Params("cid"), version); err != nil {
		return err
	}
	c.Status(fiber.StatusNoContent)
	return nil
}

func (h *Handler) sugerir(c *fiber.Ctx) error {
	var req SugerirRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Sugerir(c.UserContext(), claims(c), c.Params("id"), req)
	if err != nil {
		return err
	}
	return send(c, fiber.StatusOK, out)
}

// validar responde 200 en verde y 409 `{valido: false, motivo}` en rojo.
func (h *Handler) validar(c *fiber.Ctx) error {
	var req ValidarRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Validar(c.UserContext(), claims(c), c.Params("id"), req)
	if err != nil {
		return err
	}
	if !out.Valido {
		return send(c, fiber.StatusConflict, out)
	}
	return send(c, fiber.StatusOK, out)
}

func (h *Handler) cerca(c *fiber.Ctx) error {
	q := NewCercaQuery()
	if err := h.query(c, &q); err != nil {
		return err
	}
	out, err := h.svc.Cerca(c.UserContext(), q)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusOK).JSON(out)
}

func (h *Handler) detalle(c *fiber.Ctx) error {
	out, err := h.svc.Detalle(c.UserContext(), c.Params("id"))
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusOK).JSON(out)
}

func (h *Handler) guardar(c *fiber.Ctx) error {
	out, err := h.svc.Guardar(c.UserContext(), claims(c), c.Params("id"))
	if err != nil {
		return err
	}
	return send(c, fiber.StatusCreated, out)
}

func (h *Handler) mias(c *fiber.Ctx) error {
	q := NewMisQuery()
	if err := h.query(c, &q); err != nil {
		return err
	}
	out, err := h.svc.Mias(c.UserContext(), claims(c), q)
	if err != nil {
		return err
	}
	return send(c, fiber.StatusOK, out)
}

func (h *Handler) canjear(c *fiber.Ctx) error {
	out, err := h.svc.Canjear(c.UserContext(), claims(c), c.Params("id"))
	if err != nil {
		return err
	}
	return send(c, fiber.StatusOK, out)
}

func (h *Handler) quitar(c *fiber.Ctx) error {
	if err := h.svc.Quitar(c.UserContext(), claims(c), c.Params("id")); err != nil {
		return err
	}
	c.Status(fiber.StatusNoContent)
	return nil
}

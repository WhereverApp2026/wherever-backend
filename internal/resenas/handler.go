package resenas

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
	politicaCheckin = politica{"checkins", 10, time.Hour}
	politicaCrear   = politica{"resenas_crear", 10, 24 * time.Hour}
	politicaUtil    = politica{"resenas_util", 60, time.Hour}
	politicaReporte = politica{"resenas_reporte", 20, 24 * time.Hour}
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

// Routes registra las trece rutas de nv-62; `/resenas/pendientes` y `/resenas/mias` van antes
// que `/resenas/:id`. Cumple httpapi.Module.
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
	cualquiera := middleware.RequireAuth()

	v1.Post("/checkins", with(&politicaCheckin, consumidor, h.checkin)...)
	v1.Get("/checkins", with(nil, consumidor, h.checkins)...)
	v1.Get("/resenas/pendientes", with(nil, consumidor, h.pendientes)...)
	v1.Get("/resenas/mias", with(nil, consumidor, h.mias)...)
	v1.Post("/resenas", with(&politicaCrear, consumidor, h.crear)...)
	v1.Patch("/resenas/:id", with(nil, consumidor, h.editar)...)
	v1.Delete("/resenas/:id", with(nil, consumidor, h.borrar)...)
	v1.Put("/resenas/:id/util", with(&politicaUtil, cualquiera, h.util)...)
	v1.Post("/resenas/:id/reportes", with(&politicaReporte, cualquiera, h.reportar)...)
	v1.Get("/lugares/:id/resenas", h.publicas)
	v1.Get("/negocios/:id/resenas/resumen", with(nil, dueno, h.resumen)...)
	v1.Put("/negocios/:id/resenas/:rid/respuesta", with(nil, dueno, h.responder)...)
	v1.Post("/negocios/:id/resenas/:rid/sugerir-respuesta", with(nil, dueno, h.sugerir)...)
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
		return 0, apperr.BadRequest("falta la cabecera If-Match con la versión de la reseña")
	}
	return v, nil
}

func send(c *fiber.Ctx, status int, v any) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.Status(status).JSON(v)
}

func sendResena(c *fiber.Ctx, status int, out ResenaResponse) error {
	c.Set(fiber.HeaderETag, `"`+strconv.Itoa(out.Version)+`"`)
	return send(c, status, out)
}

func (h *Handler) checkin(c *fiber.Ctx) error {
	var req CheckinRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Checkin(c.UserContext(), claims(c), req)
	if err != nil {
		return err
	}
	return send(c, fiber.StatusCreated, out)
}

func (h *Handler) checkins(c *fiber.Ctx) error {
	q := NewListaQuery()
	if err := h.query(c, &q); err != nil {
		return err
	}
	out, err := h.svc.Checkins(c.UserContext(), claims(c), q)
	if err != nil {
		return err
	}
	return send(c, fiber.StatusOK, out)
}

func (h *Handler) pendientes(c *fiber.Ctx) error {
	out, err := h.svc.Pendientes(c.UserContext(), claims(c))
	if err != nil {
		return err
	}
	return send(c, fiber.StatusOK, out)
}

func (h *Handler) mias(c *fiber.Ctx) error {
	q := NewListaQuery()
	if err := h.query(c, &q); err != nil {
		return err
	}
	out, err := h.svc.Mias(c.UserContext(), claims(c), q)
	if err != nil {
		return err
	}
	return send(c, fiber.StatusOK, out)
}

func (h *Handler) crear(c *fiber.Ctx) error {
	var req CrearRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Crear(c.UserContext(), claims(c), req)
	if err != nil {
		return err
	}
	return sendResena(c, fiber.StatusCreated, out)
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
	out, err := h.svc.Editar(c.UserContext(), claims(c), c.Params("id"), version, req)
	if err != nil {
		return err
	}
	return sendResena(c, fiber.StatusOK, out)
}

func (h *Handler) borrar(c *fiber.Ctx) error {
	if err := h.svc.Borrar(c.UserContext(), claims(c), c.Params("id")); err != nil {
		return err
	}
	c.Status(fiber.StatusNoContent)
	return nil
}

func (h *Handler) util(c *fiber.Ctx) error {
	var req UtilRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Util(c.UserContext(), claims(c), c.Params("id"), req)
	if err != nil {
		return err
	}
	return send(c, fiber.StatusOK, out)
}

func (h *Handler) reportar(c *fiber.Ctx) error {
	var req ReporteRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Reportar(c.UserContext(), claims(c), c.Params("id"), req)
	if err != nil {
		return err
	}
	return send(c, fiber.StatusCreated, out)
}

func (h *Handler) publicas(c *fiber.Ctx) error {
	q := NewPublicaQuery()
	if err := h.query(c, &q); err != nil {
		return err
	}
	out, err := h.svc.Publicas(c.UserContext(), c.Params("id"), q)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusOK).JSON(out)
}

func (h *Handler) resumen(c *fiber.Ctx) error {
	q := ResumenQuery{Periodo: "mes"}
	if err := h.query(c, &q); err != nil {
		return err
	}
	out, err := h.svc.Resumen(c.UserContext(), claims(c), c.Params("id"), q)
	if err != nil {
		return err
	}
	return send(c, fiber.StatusOK, out)
}

func (h *Handler) responder(c *fiber.Ctx) error {
	var req RespuestaRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Responder(c.UserContext(), claims(c), c.Params("id"), c.Params("rid"), req)
	if err != nil {
		return err
	}
	return send(c, fiber.StatusOK, out)
}

func (h *Handler) sugerir(c *fiber.Ctx) error {
	out, err := h.svc.Sugerir(c.UserContext(), claims(c), c.Params("id"), c.Params("rid"))
	if err != nil {
		return err
	}
	return send(c, fiber.StatusOK, out)
}

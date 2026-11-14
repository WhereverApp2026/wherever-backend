package panel

import (
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

// La ruta del Sello usa la política `admin` de docs/rate-limit-matrix.md (la misma del alta).
const (
	politicaAdmin = "admin"
	limiteAdmin   = 200
	ventanaAdmin  = time.Minute
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

// Routes registra el panel del dueño y el Sello del administrador. Cumple httpapi.Module.
func (h *Handler) Routes(v1 fiber.Router, limiter *middleware.Limiter) {
	dueno := []fiber.Handler{middleware.RequireRoles(middleware.RoleMerchant)}
	if h.active != nil {
		dueno = append(dueno, h.active)
	}
	with := func(fn fiber.Handler) []fiber.Handler { return append(append([]fiber.Handler{}, dueno...), fn) }
	v1.Get("/negocios/:id/panel", with(h.panel)...)
	v1.Patch("/negocios/:id", with(h.perfil)...)
	v1.Delete("/negocios/:id", with(h.baja)...)
	v1.Post("/negocios/:id/pausa", with(h.pausa)...)
	v1.Put("/negocios/:id/reservas-config", with(h.reservas)...)
	v1.Get("/negocios/:id/qr", with(h.qr)...)
	v1.Put("/admin/negocios/:id/sello", limiter.Window(politicaAdmin, limiteAdmin, ventanaAdmin), middleware.RequireRoles(middleware.RoleAdmin), h.sello)
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

func noStore(c *fiber.Ctx, out any) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.JSON(out)
}

func (h *Handler) panel(c *fiber.Ctx) error {
	out, err := h.svc.Panel(c.UserContext(), claims(c), c.Params("id"))
	if err != nil {
		return err
	}
	return noStore(c, out)
}

func (h *Handler) perfil(c *fiber.Ctx) error {
	version, err := ifMatch(c)
	if err != nil {
		return err
	}
	var req PerfilRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Perfil(c.UserContext(), claims(c), c.Params("id"), version, req)
	if err != nil {
		return err
	}
	return noStore(c, out)
}

func (h *Handler) pausa(c *fiber.Ctx) error {
	var req PausaRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Pausa(c.UserContext(), claims(c), c.Params("id"), req)
	if err != nil {
		return err
	}
	return noStore(c, out)
}

func (h *Handler) reservas(c *fiber.Ctx) error {
	version, err := ifMatch(c)
	if err != nil {
		return err
	}
	var req ReservasConfigRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.ReservasConfig(c.UserContext(), claims(c), c.Params("id"), version, req)
	if err != nil {
		return err
	}
	return noStore(c, out)
}

func (h *Handler) qr(c *fiber.Ctx) error {
	out, err := h.svc.QR(c.UserContext(), claims(c), c.Params("id"))
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderCacheControl, "private, max-age=86400")
	return c.JSON(out)
}

func (h *Handler) baja(c *fiber.Ctx) error {
	var req BajaRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	if err := h.svc.Baja(c.UserContext(), claims(c), c.Params("id"), req); err != nil {
		return err
	}
	c.Status(fiber.StatusNoContent)
	return nil
}

func (h *Handler) sello(c *fiber.Ctx) error {
	var req SelloRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Sello(c.UserContext(), claims(c), c.Params("id"), req)
	if err != nil {
		return err
	}
	return noStore(c, out)
}

package reservas

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

// POST /reservas: política `reservas` de docs/rate-limit-matrix.md, 10 por hora por usuario.
const (
	politicaReservas = "reservas"
	limiteReservas   = 10
	ventanaReservas  = time.Hour
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

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

// Routes registra las doce rutas de nv-59. Cumple httpapi.Module.
func (h *Handler) Routes(v1 fiber.Router, limiter *middleware.Limiter) {
	with := func(first fiber.Handler, fn fiber.Handler) []fiber.Handler {
		out := []fiber.Handler{first}
		if h.active != nil {
			out = append(out, h.active)
		}
		return append(out, fn)
	}
	consumidor := middleware.RequireRoles(middleware.RoleConsumer)
	dueno := middleware.RequireRoles(middleware.RoleMerchant)
	auth := middleware.RequireAuth()

	v1.Get("/lugares/:id/disponibilidad", h.disponibilidad)
	crear := with(consumidor, h.crear)
	if limiter != nil {
		crear = append([]fiber.Handler{limiter.WindowUser(politicaReservas, limiteReservas, ventanaReservas)}, crear...)
	}
	v1.Post("/reservas", crear...)
	v1.Get("/reservas", with(consumidor, h.mias)...)
	v1.Get("/reservas/:id", with(auth, h.detalle)...)
	v1.Patch("/reservas/:id", with(consumidor, h.cambiar)...)
	v1.Get("/reservas/:id/reembolso", with(consumidor, h.reembolso)...)
	v1.Post("/reservas/:id/cancelar", with(auth, h.cancelar)...)
	v1.Get("/negocios/:id/reservas", with(dueno, h.agenda)...)
	v1.Post("/negocios/:id/reservas/escanear", with(dueno, h.escanear)...)
	v1.Patch("/negocios/:id/reservas/:rid", with(dueno, h.estado)...)
	v1.Post("/negocios/:id/bloqueos", with(dueno, h.crearBloqueo)...)
	v1.Delete("/negocios/:id/bloqueos/:bid", with(dueno, h.borrarBloqueo)...)
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
		return 0, apperr.BadRequest("falta la cabecera If-Match con la versión de la reserva")
	}
	return v, nil
}

func send(c *fiber.Ctx, status int, v any) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.Status(status).JSON(v)
}

func sendReserva(c *fiber.Ctx, status int, out ReservaResponse) error {
	c.Set(fiber.HeaderETag, `"`+strconv.Itoa(out.Version)+`"`)
	return send(c, status, out)
}

func (h *Handler) disponibilidad(c *fiber.Ctx) error {
	var q DisponibilidadQuery
	if err := h.query(c, &q); err != nil {
		return err
	}
	out, err := h.svc.Disponibilidad(c.UserContext(), c.Params("id"), q)
	if err != nil {
		return err
	}
	return send(c, fiber.StatusOK, out)
}

func (h *Handler) crear(c *fiber.Ctx) error {
	llave := strings.TrimSpace(c.Get("Idempotency-Key"))
	if !uuidPattern.MatchString(llave) {
		return apperr.BadRequest("falta la cabecera Idempotency-Key (UUID)")
	}
	var req CrearRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	out, created, err := h.svc.Crear(c.UserContext(), claims(c), strings.ToLower(llave), req)
	if err != nil {
		return err
	}
	status := fiber.StatusOK
	if created {
		status = fiber.StatusCreated
	}
	return sendReserva(c, status, out)
}

func (h *Handler) mias(c *fiber.Ctx) error {
	q := NewMiasQuery()
	if err := h.query(c, &q); err != nil {
		return err
	}
	out, err := h.svc.Mias(c.UserContext(), claims(c), q)
	if err != nil {
		return err
	}
	return send(c, fiber.StatusOK, out)
}

func (h *Handler) detalle(c *fiber.Ctx) error {
	out, err := h.svc.Detalle(c.UserContext(), claims(c), c.Params("id"))
	if err != nil {
		return err
	}
	return sendReserva(c, fiber.StatusOK, out)
}

func (h *Handler) cambiar(c *fiber.Ctx) error {
	version, err := ifMatch(c)
	if err != nil {
		return err
	}
	var req CambiarRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Cambiar(c.UserContext(), claims(c), c.Params("id"), version, req)
	if err != nil {
		return err
	}
	return sendReserva(c, fiber.StatusOK, out)
}

func (h *Handler) reembolso(c *fiber.Ctx) error {
	out, err := h.svc.CalcularReembolso(c.UserContext(), claims(c), c.Params("id"))
	if err != nil {
		return err
	}
	return send(c, fiber.StatusOK, out)
}

func (h *Handler) cancelar(c *fiber.Ctx) error {
	var req CancelarRequest
	if len(c.Body()) > 0 {
		if err := h.bind(c, &req); err != nil {
			return err
		}
	}
	out, err := h.svc.Cancelar(c.UserContext(), claims(c), c.Params("id"), req)
	if err != nil {
		return err
	}
	return sendReserva(c, fiber.StatusOK, out)
}

func (h *Handler) agenda(c *fiber.Ctx) error {
	var q AgendaQuery
	if err := h.query(c, &q); err != nil {
		return err
	}
	out, err := h.svc.Agenda(c.UserContext(), claims(c), c.Params("id"), q)
	if err != nil {
		return err
	}
	return send(c, fiber.StatusOK, out)
}

func (h *Handler) estado(c *fiber.Ctx) error {
	var req EstadoRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.CambiarEstado(c.UserContext(), claims(c), c.Params("id"), c.Params("rid"), req)
	if err != nil {
		return err
	}
	return sendReserva(c, fiber.StatusOK, out)
}

func (h *Handler) escanear(c *fiber.Ctx) error {
	var req EscanearRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Escanear(c.UserContext(), claims(c), c.Params("id"), req)
	if err != nil {
		return err
	}
	return sendReserva(c, fiber.StatusOK, out)
}

func (h *Handler) crearBloqueo(c *fiber.Ctx) error {
	var req BloqueoRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.CrearBloqueo(c.UserContext(), claims(c), c.Params("id"), req)
	if err != nil {
		return err
	}
	return send(c, fiber.StatusCreated, out)
}

// borrarBloqueo responde 204 sin cuerpo (c.Status, no SendStatus: SendStatus escribe texto).
func (h *Handler) borrarBloqueo(c *fiber.Ctx) error {
	if err := h.svc.BorrarBloqueo(c.UserContext(), claims(c), c.Params("id"), c.Params("bid")); err != nil {
		return err
	}
	c.Status(fiber.StatusNoContent)
	return nil
}

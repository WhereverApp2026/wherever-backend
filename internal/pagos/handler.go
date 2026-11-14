package pagos

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

// POST /reservas/{id}/pago: política `pagos` de docs/rate-limit-matrix.md, 10 por hora por usuario.
const (
	politicaPagos = "pagos"
	limitePagos   = 10
	ventanaPagos  = time.Hour
)

var (
	uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	errCuerpo   = errors.New("cuerpo con más de un valor JSON")
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

// WithActiveGuard agrega usuarios.RequireActive a las rutas con token.
func (h *Handler) WithActiveGuard(guard fiber.Handler) *Handler {
	h.active = guard
	return h
}

// Routes registra las rutas de nv-60. Cumple httpapi.Module.
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

	cobrar := with(consumidor, h.cobrar)
	if limiter != nil {
		cobrar = append([]fiber.Handler{limiter.WindowUser(politicaPagos, limitePagos, ventanaPagos)}, cobrar...)
	}
	v1.Post("/reservas/:id/pago", cobrar...)
	v1.Post("/webhooks/stripe", h.webhook)
	v1.Get("/pagos/:id/estado", with(consumidor, h.estado)...)
	v1.Get("/negocios/:id/reservas/:rid/cobro", with(dueno, h.cobro)...)
	v1.Post("/negocios/:id/cobros/conectar", with(dueno, h.conectar)...)
	v1.Get("/negocios/:id/saldo", with(dueno, h.saldo)...)
	v1.Get("/negocios/:id/liquidaciones", with(dueno, h.liquidaciones)...)
}

func claims(c *fiber.Ctx) *middleware.Claims {
	cl, _ := middleware.ClaimsFrom(c)
	return cl
}

func send(c *fiber.Ctx, v any) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.Status(fiber.StatusOK).JSON(v)
}

func (h *Handler) cobrar(c *fiber.Ctx) error {
	llave := strings.TrimSpace(c.Get("Idempotency-Key"))
	if !uuidPattern.MatchString(llave) {
		return apperr.BadRequest("falta la cabecera Idempotency-Key (UUID)")
	}
	var req CobrarRequest
	if err := DecodeEstricto(c.Body(), &req); err != nil {
		return apperr.BadRequest("cuerpo JSON inválido")
	}
	out, err := h.svc.Cobrar(c.UserContext(), claims(c), c.Params("id"), strings.ToLower(llave))
	if err != nil {
		return err
	}
	return send(c, out)
}

// webhook lee el cuerpo crudo: la firma se verifica antes de interpretar el JSON.
func (h *Handler) webhook(c *fiber.Ctx) error {
	if err := h.svc.Webhook(c.UserContext(), c.Body(), c.Get("Stripe-Signature")); err != nil {
		return err
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"recibido": true})
}

func (h *Handler) estado(c *fiber.Ctx) error {
	out, err := h.svc.Estado(c.UserContext(), claims(c), c.Params("id"))
	if err != nil {
		return err
	}
	return send(c, out)
}

func (h *Handler) cobro(c *fiber.Ctx) error {
	out, err := h.svc.Cobro(c.UserContext(), claims(c), c.Params("id"), c.Params("rid"))
	if err != nil {
		return err
	}
	return send(c, out)
}

func (h *Handler) conectar(c *fiber.Ctx) error {
	var req struct{}
	if err := DecodeEstricto(c.Body(), &req); err != nil {
		return apperr.BadRequest("cuerpo JSON inválido")
	}
	out, err := h.svc.Conectar(c.UserContext(), claims(c), c.Params("id"))
	if err != nil {
		return err
	}
	return send(c, out)
}

func (h *Handler) saldo(c *fiber.Ctx) error {
	out, err := h.svc.Saldo(c.UserContext(), claims(c), c.Params("id"))
	if err != nil {
		return err
	}
	return send(c, out)
}

func (h *Handler) liquidaciones(c *fiber.Ctx) error {
	q := NewLiquidacionesQuery()
	if err := c.QueryParser(&q); err != nil {
		return apperr.BadRequest("parámetros inválidos")
	}
	if err := h.validator.Struct(q); err != nil {
		return err
	}
	out, err := h.svc.Liquidaciones(c.UserContext(), claims(c), c.Params("id"), q)
	if err != nil {
		return err
	}
	return send(c, out)
}

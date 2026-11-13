package vaca

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

// WithActiveGuard agrega usuarios.RequireActive a las siete rutas: con la cuenta en
// eliminación pendiente la Vaca responde 403 (nv-58).
func (h *Handler) WithActiveGuard(guard fiber.Handler) *Handler {
	h.active = guard
	return h
}

// Routes registra la Vaca. Cumple httpapi.Module.
func (h *Handler) Routes(v1 fiber.Router, _ *middleware.Limiter) {
	v1.Post("/grupos/:id/gastos", h.chain(h.create)...)
	v1.Get("/grupos/:id/gastos", h.chain(h.list)...)
	v1.Get("/grupos/:id/vaca", h.chain(h.cut)...)
	v1.Patch("/gastos/:id", h.chain(h.patch)...)
	v1.Delete("/gastos/:id", h.chain(h.remove)...)
	v1.Get("/vaca/mis-deudas", h.chain(h.myDebts)...)
	v1.Patch("/vaca/deudas/:id", h.chain(h.markPaid)...)
}

// chain antepone la autenticación y el guardia de cuenta activa.
func (h *Handler) chain(handler fiber.Handler) []fiber.Handler {
	out := []fiber.Handler{middleware.RequireAuth()}
	if h.active != nil {
		out = append(out, h.active)
	}
	return append(out, handler)
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
		return 0, apperr.BadRequest("falta la cabecera If-Match con la versión del gasto")
	}
	return version, nil
}

func send(c *fiber.Ctx, status int, v any) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.Status(status).JSON(v)
}

func sendExpense(c *fiber.Ctx, status int, out ExpenseResult) error {
	c.Set(fiber.HeaderETag, `"`+strconv.Itoa(out.Expense.Version)+`"`)
	return send(c, status, out)
}

func (h *Handler) create(c *fiber.Ctx) error {
	var req CreateRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	out, created, err := h.svc.Create(c.UserContext(), subject(c), c.Params("id"), req)
	if err != nil {
		return err
	}
	status := fiber.StatusOK
	if created {
		status = fiber.StatusCreated
	}
	return sendExpense(c, status, out)
}

func (h *Handler) list(c *fiber.Ctx) error {
	q := NewListQuery()
	if err := h.query(c, &q); err != nil {
		return err
	}
	out, err := h.svc.List(c.UserContext(), subject(c), c.Params("id"), q)
	if err != nil {
		return err
	}
	return send(c, fiber.StatusOK, out)
}

func (h *Handler) cut(c *fiber.Ctx) error {
	out, err := h.svc.Cut(c.UserContext(), subject(c), c.Params("id"))
	if err != nil {
		return err
	}
	return send(c, fiber.StatusOK, out)
}

func (h *Handler) patch(c *fiber.Ctx) error {
	version, err := ifMatch(c)
	if err != nil {
		return err
	}
	var req PatchRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Patch(c.UserContext(), subject(c), c.Params("id"), version, req)
	if err != nil {
		return err
	}
	return sendExpense(c, fiber.StatusOK, out)
}

func (h *Handler) remove(c *fiber.Ctx) error {
	version, err := ifMatch(c)
	if err != nil {
		return err
	}
	confirm := false
	if raw := c.Query("confirmar"); raw != "" {
		if confirm, err = strconv.ParseBool(raw); err != nil {
			return apperr.BadRequest("confirmar debe ser true o false")
		}
	}
	if err := h.svc.Delete(c.UserContext(), subject(c), c.Params("id"), version, confirm); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) markPaid(c *fiber.Ctx) error {
	var req MarkPaidRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.MarkPaid(c.UserContext(), subject(c), c.Params("id"), req)
	if err != nil {
		return err
	}
	return send(c, fiber.StatusOK, out)
}

func (h *Handler) myDebts(c *fiber.Ctx) error {
	var q MyDebtsQuery
	if err := h.query(c, &q); err != nil {
		return err
	}
	out, err := h.svc.MyDebts(c.UserContext(), subject(c), q)
	if err != nil {
		return err
	}
	return send(c, fiber.StatusOK, out)
}

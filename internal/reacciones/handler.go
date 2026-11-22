package reacciones

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

// Límite de escrituras sociales por usuario (docs/rate-limit-matrix.md,
// `rate:social_escrituras:{usuario}`): me gusta, quitarlo y comentar comparten la cuota.
const (
	politicaSocial = "social_escrituras"
	limiteSocial   = 60
	ventanaSocial  = time.Hour
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

// WithActiveGuard exige cuenta activa (nv-05) en todas las rutas.
func (h *Handler) WithActiveGuard(guard fiber.Handler) *Handler {
	h.active = guard
	return h
}

// Routes registra las 13 rutas de nv-83. Va antes que grupos y planes: el grupo /planes de
// planes pide sesión para todo su prefijo y estas rutas ya la piden. Cumple httpapi.Module.
func (h *Handler) Routes(v1 fiber.Router, limiter *middleware.Limiter) {
	with := func(limitar bool, fn fiber.Handler) []fiber.Handler {
		out := []fiber.Handler{}
		if limitar && limiter != nil {
			out = append(out, limiter.WindowUser(politicaSocial, limiteSocial, ventanaSocial))
		}
		out = append(out, middleware.RequireAuth())
		if h.active != nil {
			out = append(out, h.active)
		}
		return append(out, fn)
	}
	for _, o := range []struct{ prefijo, tipo string }{{"/momentos", ObjetoMomento}, {"/planes", ObjetoPlan}} {
		tipo := o.tipo
		v1.Put(o.prefijo+"/:id/me-gusta", with(true, func(c *fiber.Ctx) error { return h.darMeGusta(c, tipo) })...)
		v1.Delete(o.prefijo+"/:id/me-gusta", with(true, func(c *fiber.Ctx) error { return h.quitarMeGusta(c, tipo) })...)
		v1.Get(o.prefijo+"/:id/comentarios", with(false, func(c *fiber.Ctx) error { return h.comentarios(c, tipo) })...)
		v1.Post(o.prefijo+"/:id/comentarios", with(true, func(c *fiber.Ctx) error { return h.comentar(c, tipo) })...)
		v1.Get(o.prefijo+"/:id/reacciones", with(false, func(c *fiber.Ctx) error { return h.reacciones(c, tipo) })...)
	}
	v1.Delete("/comentarios/:id", with(false, h.borrar)...)
}

func claims(c *fiber.Ctx) *middleware.Claims {
	cl, _ := middleware.ClaimsFrom(c)
	return cl
}

func (h *Handler) query(c *fiber.Ctx, dto any) error {
	if err := c.QueryParser(dto); err != nil {
		return apperr.BadRequest("parámetros inválidos")
	}
	return h.validator.Struct(dto)
}

func send(c *fiber.Ctx, status int, v any) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.Status(status).JSON(v)
}

// sinCuerpo responde 204 (SendStatus escribiría el texto del estado).
func sinCuerpo(c *fiber.Ctx) error {
	c.Status(fiber.StatusNoContent)
	return nil
}

func (h *Handler) darMeGusta(c *fiber.Ctx, tipo string) error {
	if err := h.svc.DarMeGusta(c.UserContext(), claims(c), tipo, c.Params("id")); err != nil {
		return err
	}
	return sinCuerpo(c)
}

func (h *Handler) quitarMeGusta(c *fiber.Ctx, tipo string) error {
	if err := h.svc.QuitarMeGusta(c.UserContext(), claims(c), tipo, c.Params("id")); err != nil {
		return err
	}
	return sinCuerpo(c)
}

func (h *Handler) comentarios(c *fiber.Ctx, tipo string) error {
	q := NewListaQuery()
	if err := h.query(c, &q); err != nil {
		return err
	}
	out, err := h.svc.Comentarios(c.UserContext(), claims(c), tipo, c.Params("id"), q)
	if err != nil {
		return err
	}
	return send(c, fiber.StatusOK, out)
}

func (h *Handler) comentar(c *fiber.Ctx, tipo string) error {
	var req ComentarioRequest
	if err := c.BodyParser(&req); err != nil {
		return apperr.BadRequest("cuerpo JSON inválido")
	}
	if err := h.validator.Struct(req); err != nil {
		return err
	}
	out, err := h.svc.Comentar(c.UserContext(), claims(c), tipo, c.Params("id"), req)
	if err != nil {
		return err
	}
	return send(c, fiber.StatusCreated, out)
}

func (h *Handler) reacciones(c *fiber.Ctx, tipo string) error {
	q := NewReaccionesQuery()
	if err := h.query(c, &q); err != nil {
		return err
	}
	out, err := h.svc.Reacciones(c.UserContext(), claims(c), tipo, c.Params("id"), q)
	if err != nil {
		return err
	}
	return send(c, fiber.StatusOK, out)
}

func (h *Handler) borrar(c *fiber.Ctx) error {
	if err := h.svc.BorrarComentario(c.UserContext(), claims(c), c.Params("id")); err != nil {
		return err
	}
	return sinCuerpo(c)
}

package grupos

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

// Límite por usuario de las invitaciones (nv-37, docs/rate-limit-matrix.md).
const (
	invitePolicy = "grupos_invitacion"
	inviteLimit  = 30
	inviteWindow = time.Hour
	// Vista pública por IP y unirse por usuario (nv-38). El código tiene 128 bits: el límite
	// frena el escaneo, no la adivinanza.
	previewPolicy = "plan_compartido"
	previewLimit  = 30
	previewWindow = time.Minute
	joinPolicy    = "grupos_unirse"
	joinLimit     = 30
	joinWindow    = time.Hour
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

// WithActiveGuard agrega usuarios.RequireActive a las escrituras: con la cuenta en
// eliminación pendiente solo se leen los grupos (403 al escribir).
func (h *Handler) WithActiveGuard(guard fiber.Handler) *Handler {
	h.active = guard
	return h
}

// Routes registra /api/v1/grupos y POST /api/v1/planes/:id/invitaciones. Cumple
// httpapi.Module. Va antes que planes en la lista de módulos: así la invitación no pasa por
// el grupo /planes. `/integrantes/yo` va antes que `/integrantes/:usuario_id`.
func (h *Handler) Routes(v1 fiber.Router, limiter *middleware.Limiter) {
	invites := func(handler fiber.Handler) []fiber.Handler {
		chain := []fiber.Handler{}
		if limiter != nil {
			chain = append(chain, limiter.WindowUser(invitePolicy, inviteLimit, inviteWindow))
		}
		return append(h.write(chain...), handler)
	}
	v1.Post("/planes/:id/invitaciones", append([]fiber.Handler{middleware.RequireAuth()}, invites(h.inviteToPlan)...)...)
	h.shareRoutes(v1, limiter)

	g := v1.Group("/grupos", middleware.RequireAuth())
	g.Post("/unirse", append(h.write(limited(limiter, func(l *middleware.Limiter) fiber.Handler {
		return l.WindowUser(joinPolicy, joinLimit, joinWindow)
	})...), h.join)...)
	g.Get("", h.list)
	g.Get("/:id", h.get)
	g.Post("", invites(h.create)...)
	g.Post("/:id/integrantes", invites(h.invite)...)
	g.Patch("/:id/integrantes/yo", append(h.write(), h.respond)...)
	g.Delete("/:id/integrantes/yo", append(h.write(), h.leave)...)
	g.Delete("/:id/integrantes/:usuario_id", append(h.write(), h.remove)...)
}

// write antepone el guardia de cuenta activa a los handlers dados.
func (h *Handler) write(handlers ...fiber.Handler) []fiber.Handler {
	if h.active == nil {
		return handlers
	}
	return append([]fiber.Handler{h.active}, handlers...)
}

// bind lee el JSON y lo valida: cuerpo ilegible → 400, reglas → 422.
func (h *Handler) bind(c *fiber.Ctx, dto any) error {
	if err := c.BodyParser(dto); err != nil {
		return apperr.BadRequest("cuerpo JSON inválido")
	}
	return h.validator.Struct(dto)
}

func subject(c *fiber.Ctx) string {
	if cl, ok := middleware.ClaimsFrom(c); ok {
		return cl.Subject
	}
	return ""
}

func send(c *fiber.Ctx, status int, v View, err error) error {
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.Status(status).JSON(v)
}

func (h *Handler) list(c *fiber.Ctx) error {
	q := NewListQuery()
	if err := c.QueryParser(&q); err != nil {
		return apperr.BadRequest("parámetros inválidos")
	}
	if err := h.validator.Struct(q); err != nil {
		return err
	}
	out, err := h.svc.List(c.UserContext(), subject(c), q)
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.JSON(out)
}

func (h *Handler) get(c *fiber.Ctx) error {
	v, err := h.svc.Get(c.UserContext(), subject(c), c.Params("id"))
	return send(c, fiber.StatusOK, v, err)
}

func (h *Handler) create(c *fiber.Ctx) error {
	var req CreateRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	v, err := h.svc.Create(c.UserContext(), subject(c), req)
	return send(c, fiber.StatusCreated, v, err)
}

func (h *Handler) invite(c *fiber.Ctx) error {
	var req InviteRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	v, err := h.svc.Invite(c.UserContext(), subject(c), c.Params("id"), req)
	return send(c, fiber.StatusOK, v, err)
}

func (h *Handler) inviteToPlan(c *fiber.Ctx) error {
	var req InviteRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	v, err := h.svc.InviteToPlan(c.UserContext(), subject(c), c.Params("id"), req)
	return send(c, fiber.StatusOK, v, err)
}

func (h *Handler) respond(c *fiber.Ctx) error {
	var req RespondRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	v, err := h.svc.Respond(c.UserContext(), subject(c), c.Params("id"), req)
	return send(c, fiber.StatusOK, v, err)
}

func (h *Handler) leave(c *fiber.Ctx) error {
	if err := h.svc.Leave(c.UserContext(), subject(c), c.Params("id")); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) remove(c *fiber.Ctx) error {
	if err := h.svc.Remove(c.UserContext(), subject(c), c.Params("id"), c.Params("usuario_id")); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// shareRoutes registra el enlace de invitación (nv-38). La vista pública no pide sesión.
func (h *Handler) shareRoutes(v1 fiber.Router, limiter *middleware.Limiter) {
	v1.Post("/planes/compartidos", append(limited(limiter, func(l *middleware.Limiter) fiber.Handler {
		return l.Window(previewPolicy, previewLimit, previewWindow)
	}), h.preview)...)
	v1.Post("/planes/:id/compartir", append([]fiber.Handler{middleware.RequireAuth()}, append(h.write(), h.share)...)...)
	v1.Delete("/planes/:id/compartir", append([]fiber.Handler{middleware.RequireAuth()}, append(h.write(), h.revoke)...)...)
}

// limited devuelve el limitador de la ruta, o nada si no hay limitador (scripts/routes).
func limited(limiter *middleware.Limiter, policy func(*middleware.Limiter) fiber.Handler) []fiber.Handler {
	if limiter == nil {
		return nil
	}
	return []fiber.Handler{policy(limiter)}
}

func (h *Handler) share(c *fiber.Ctx) error {
	out, err := h.svc.Share(c.UserContext(), subject(c), c.Params("id"))
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.Status(fiber.StatusCreated).JSON(out)
}

func (h *Handler) revoke(c *fiber.Ctx) error {
	if err := h.svc.Revoke(c.UserContext(), subject(c), c.Params("id")); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) preview(c *fiber.Ctx) error {
	var req CodeRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.Preview(c.UserContext(), req.Code)
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.JSON(out)
}

func (h *Handler) join(c *fiber.Ctx) error {
	var req JoinRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	v, err := h.svc.Join(c.UserContext(), subject(c), req)
	return send(c, fiber.StatusOK, v, err)
}

package planes

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

// NewHandler recibe el service y el validador compartido por constructor.
func NewHandler(svc *Service, v *validation.Validator) *Handler {
	return &Handler{svc: svc, validator: v}
}

// WithActiveGuard agrega usuarios.RequireActive a las escrituras: con la cuenta en
// eliminación pendiente solo se leen los planes (403 al escribir).
func (h *Handler) WithActiveGuard(guard fiber.Handler) *Handler {
	h.active = guard
	return h
}

// Routes registra /api/v1/planes, todo con JWT. Cumple httpapi.Module. `/paradas/orden`
// va antes de `/paradas/:parada_id`.
func (h *Handler) Routes(v1 fiber.Router, _ *middleware.Limiter) {
	g := v1.Group("/planes", middleware.RequireAuth())
	g.Get("", h.list)
	g.Get("/:id", h.get)
	g.Post("", h.write(h.create)...)
	g.Patch("/:id", h.write(h.patch)...)
	g.Delete("/:id", h.write(h.remove)...)
	g.Post("/:id/clonar", h.write(h.clone)...)
	g.Post("/:id/paradas", h.write(h.addStop)...)
	g.Put("/:id/paradas/orden", h.write(h.reorder)...)
	g.Patch("/:id/paradas/:parada_id", h.write(h.patchStop)...)
	g.Delete("/:id/paradas/:parada_id", h.write(h.removeStop)...)
}

func (h *Handler) write(handler fiber.Handler) []fiber.Handler {
	if h.active == nil {
		return []fiber.Handler{handler}
	}
	return []fiber.Handler{h.active, handler}
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

// ifMatch lee la versión de `If-Match` (`3`, `"3"` o `W/"3"`). Falta o mal formada = 400.
func ifMatch(c *fiber.Ctx) (int, error) {
	raw := strings.Trim(strings.TrimPrefix(strings.TrimSpace(c.Get(fiber.HeaderIfMatch)), "W/"), `"`)
	version, err := strconv.Atoi(raw)
	if err != nil || version < 1 {
		return 0, apperr.BadRequest("falta la cabecera If-Match con la versión del plan")
	}
	return version, nil
}

// send responde el plan con su versión como ETag (para el siguiente If-Match).
func send(c *fiber.Ctx, status int, p Plan, err error) error {
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderETag, `"`+strconv.Itoa(p.Version)+`"`)
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.Status(status).JSON(p)
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
	p, err := h.svc.Get(c.UserContext(), subject(c), c.Params("id"))
	return send(c, fiber.StatusOK, p, err)
}

func (h *Handler) create(c *fiber.Ctx) error {
	var req CreateRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	p, err := h.svc.Create(c.UserContext(), subject(c), req)
	return send(c, fiber.StatusCreated, p, err)
}

func (h *Handler) clone(c *fiber.Ctx) error {
	var req CloneRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	p, err := h.svc.Clone(c.UserContext(), subject(c), c.Params("id"), req)
	return send(c, fiber.StatusCreated, p, err)
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
	p, err := h.svc.Patch(c.UserContext(), subject(c), c.Params("id"), version, req)
	return send(c, fiber.StatusOK, p, err)
}

func (h *Handler) remove(c *fiber.Ctx) error {
	version, err := ifMatch(c)
	if err != nil {
		return err
	}
	if err := h.svc.Delete(c.UserContext(), subject(c), c.Params("id"), version); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) addStop(c *fiber.Ctx) error {
	version, err := ifMatch(c)
	if err != nil {
		return err
	}
	var req StopRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	p, err := h.svc.AddStop(c.UserContext(), subject(c), c.Params("id"), version, req)
	return send(c, fiber.StatusCreated, p, err)
}

func (h *Handler) patchStop(c *fiber.Ctx) error {
	version, err := ifMatch(c)
	if err != nil {
		return err
	}
	var req PatchStopRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	p, err := h.svc.PatchStop(c.UserContext(), subject(c), c.Params("id"), c.Params("parada_id"), version, req)
	return send(c, fiber.StatusOK, p, err)
}

func (h *Handler) reorder(c *fiber.Ctx) error {
	version, err := ifMatch(c)
	if err != nil {
		return err
	}
	var req OrderRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	p, err := h.svc.Reorder(c.UserContext(), subject(c), c.Params("id"), version, req)
	return send(c, fiber.StatusOK, p, err)
}

func (h *Handler) removeStop(c *fiber.Ctx) error {
	version, err := ifMatch(c)
	if err != nil {
		return err
	}
	p, err := h.svc.RemoveStop(c.UserContext(), subject(c), c.Params("id"), c.Params("parada_id"), version)
	return send(c, fiber.StatusOK, p, err)
}

package postales

import (
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

// Límite de momentos por usuario (docs/rate-limit-matrix.md, `rate:momentos:{usuario}`).
const (
	politicaMomentos = "momentos"
	limiteMomentos   = 30
	ventanaMomentos  = time.Hour
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

// Routes registra las 17 rutas de nv-63. Va antes que negocios y planes (atiende rutas de sus
// prefijos) y `/momentos/feed` antes que `/momentos/:id`. Cumple httpapi.Module.
func (h *Handler) Routes(v1 fiber.Router, limiter *middleware.Limiter) {
	with := func(limitar bool, first fiber.Handler, fn fiber.Handler) []fiber.Handler {
		out := []fiber.Handler{}
		if limitar && limiter != nil {
			out = append(out, limiter.WindowUser(politicaMomentos, limiteMomentos, ventanaMomentos))
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

	v1.Post("/postales", with(false, consumidor, h.crearPostal)...)
	v1.Get("/postales", with(false, cualquiera, h.album)...)
	v1.Get("/postales/:id", with(false, cualquiera, h.postal)...)
	v1.Patch("/postales/:id", with(false, cualquiera, h.editarPostal)...)
	v1.Delete("/postales/:id", with(false, cualquiera, h.borrarPostal)...)
	v1.Get("/lugares/:id/postales", h.galeria)
	v1.Get("/recuerdos", with(false, cualquiera, h.recuerdos)...)
	v1.Post("/planes/:id/recuerdo", with(false, cualquiera, h.generarRecuerdo)...)
	v1.Get("/planes/:id/recuerdo", with(false, cualquiera, h.recuerdo)...)
	v1.Delete("/recuerdos/:id", with(false, cualquiera, h.borrarRecuerdo)...)
	v1.Post("/momentos", with(true, consumidor, h.publicar)...)
	v1.Get("/momentos/feed", with(false, cualquiera, h.feed)...)
	v1.Get("/momentos/:id", h.momento)
	v1.Patch("/momentos/:id", with(false, cualquiera, h.editarMomento)...)
	v1.Delete("/momentos/:id", with(false, cualquiera, h.borrarMomento)...)
	v1.Get("/negocios/:id/momentos", with(false, dueno, h.panel)...)
	v1.Patch("/negocios/:id/momentos/:mid", with(false, dueno, h.marcar)...)
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
		return 0, apperr.BadRequest("falta la cabecera If-Match con la versión")
	}
	return v, nil
}

func send(c *fiber.Ctx, status int, v any) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.Status(status).JSON(v)
}

func sendVersion(c *fiber.Ctx, status, version int, v any) error {
	c.Set(fiber.HeaderETag, `"`+strconv.Itoa(version)+`"`)
	return send(c, status, v)
}

func (h *Handler) crearPostal(c *fiber.Ctx) error {
	var req CrearPostalRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	id, err := h.svc.CrearPostal(c.UserContext(), claims(c), req)
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderLocation, "/api/v1/postales/"+id)
	c.Status(fiber.StatusAccepted) // sin cuerpo: SendStatus escribiría "Accepted" como texto
	return nil
}

func (h *Handler) album(c *fiber.Ctx) error {
	q := NewAlbumQuery()
	if err := h.query(c, &q); err != nil {
		return err
	}
	out, err := h.svc.Album(c.UserContext(), claims(c), q)
	if err != nil {
		return err
	}
	return send(c, fiber.StatusOK, out)
}

func (h *Handler) postal(c *fiber.Ctx) error {
	out, err := h.svc.Postal(c.UserContext(), claims(c), c.Params("id"))
	if err != nil {
		return err
	}
	return sendVersion(c, fiber.StatusOK, out.Version, out)
}

func (h *Handler) editarPostal(c *fiber.Ctx) error {
	version, err := ifMatch(c)
	if err != nil {
		return err
	}
	var req EditarPostalRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.EditarPostal(c.UserContext(), claims(c), c.Params("id"), version, req)
	if err != nil {
		return err
	}
	return sendVersion(c, fiber.StatusOK, out.Version, out)
}

func (h *Handler) borrarPostal(c *fiber.Ctx) error {
	version, err := ifMatch(c)
	if err != nil {
		return err
	}
	if err := h.svc.BorrarPostal(c.UserContext(), claims(c), c.Params("id"), version); err != nil {
		return err
	}
	c.Status(fiber.StatusNoContent)
	return nil
}

func (h *Handler) galeria(c *fiber.Ctx) error {
	q := NewListaQuery()
	if err := h.query(c, &q); err != nil {
		return err
	}
	out, err := h.svc.Galeria(c.UserContext(), claims(c), c.Params("id"), q)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusOK).JSON(out)
}

func (h *Handler) recuerdos(c *fiber.Ctx) error {
	q := NewRecuerdosQuery()
	if err := h.query(c, &q); err != nil {
		return err
	}
	out, err := h.svc.Recuerdos(c.UserContext(), claims(c), q)
	if err != nil {
		return err
	}
	return send(c, fiber.StatusOK, out)
}

func (h *Handler) generarRecuerdo(c *fiber.Ctx) error {
	var req RecuerdoRequest
	if len(c.Body()) > 0 {
		if err := h.bind(c, &req); err != nil {
			return err
		}
	}
	out, existia, err := h.svc.GenerarRecuerdo(c.UserContext(), claims(c), c.Params("id"), req)
	if err != nil {
		return err
	}
	if existia {
		return send(c, fiber.StatusOK, out)
	}
	return send(c, fiber.StatusCreated, out)
}

func (h *Handler) recuerdo(c *fiber.Ctx) error {
	out, err := h.svc.RecuerdoDePlan(c.UserContext(), claims(c), c.Params("id"))
	if err != nil {
		return err
	}
	return send(c, fiber.StatusOK, out)
}

func (h *Handler) borrarRecuerdo(c *fiber.Ctx) error {
	if err := h.svc.BorrarRecuerdo(c.UserContext(), claims(c), c.Params("id")); err != nil {
		return err
	}
	c.Status(fiber.StatusNoContent)
	return nil
}

func (h *Handler) publicar(c *fiber.Ctx) error {
	var req MomentoRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.PublicarMomento(c.UserContext(), claims(c), req)
	if err != nil {
		return err
	}
	return sendVersion(c, fiber.StatusCreated, out.Version, out)
}

func (h *Handler) feed(c *fiber.Ctx) error {
	q := NewListaQuery()
	if err := h.query(c, &q); err != nil {
		return err
	}
	out, err := h.svc.Feed(c.UserContext(), claims(c), q)
	if err != nil {
		return err
	}
	return send(c, fiber.StatusOK, out)
}

func (h *Handler) momento(c *fiber.Ctx) error {
	out, err := h.svc.Momento(c.UserContext(), claims(c), c.Params("id"))
	if err != nil {
		return err
	}
	return sendVersion(c, fiber.StatusOK, out.Version, out)
}

func (h *Handler) editarMomento(c *fiber.Ctx) error {
	version, err := ifMatch(c)
	if err != nil {
		return err
	}
	var req EditarMomentoRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.EditarMomento(c.UserContext(), claims(c), c.Params("id"), version, req)
	if err != nil {
		return err
	}
	return sendVersion(c, fiber.StatusOK, out.Version, out)
}

func (h *Handler) borrarMomento(c *fiber.Ctx) error {
	version, err := ifMatch(c)
	if err != nil {
		return err
	}
	if err := h.svc.BorrarMomento(c.UserContext(), claims(c), c.Params("id"), version); err != nil {
		return err
	}
	c.Status(fiber.StatusNoContent)
	return nil
}

func (h *Handler) panel(c *fiber.Ctx) error {
	q := NewPanelQuery()
	if err := h.query(c, &q); err != nil {
		return err
	}
	out, err := h.svc.MomentosDelNegocio(c.UserContext(), claims(c), c.Params("id"), q)
	if err != nil {
		return err
	}
	return send(c, fiber.StatusOK, out)
}

func (h *Handler) marcar(c *fiber.Ctx) error {
	var req MarcarRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.MarcarMomento(c.UserContext(), claims(c), c.Params("id"), c.Params("mid"), req)
	if err != nil {
		return err
	}
	return send(c, fiber.StatusOK, out)
}

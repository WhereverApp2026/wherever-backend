package account

import (
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

// Límites por ruta (docs/rate-limit-matrix.md): registro 5 por hora por IP (hoja A1);
// correo disponible y descarga, provisionales hasta medir.
const (
	registerLimit  = 5
	registerWindow = time.Hour
	emailLimit     = 20
	downloadLimit  = 10
	minuteWindow   = time.Minute
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

// WithActiveGuard agrega usuarios.RequireActive a /yo: con eliminación pendiente solo pasan
// GET /yo y cancelar-eliminacion; lo demás responde 403.
func (h *Handler) WithActiveGuard(guard fiber.Handler) *Handler {
	h.active = guard
	return h
}

// Routes registra /api/v1/usuarios. Cumple httpapi.Module.
func (h *Handler) Routes(v1 fiber.Router, limiter *middleware.Limiter) {
	v1.Post("/usuarios", limiter.Window("registro", registerLimit, registerWindow), h.register)
	g := v1.Group("/usuarios")
	g.Get("/correo-disponible", limiter.Window("correo_disponible", emailLimit, minuteWindow), h.emailAvailable)
	g.Post("/exportaciones/descargar", limiter.Window("exportacion_descarga", downloadLimit, minuteWindow), h.downloadExport)

	yo := g.Group("/yo", middleware.RequireAuth())
	active := func(handler fiber.Handler) []fiber.Handler {
		if h.active == nil {
			return []fiber.Handler{handler}
		}
		return []fiber.Handler{h.active, handler}
	}
	// Abiertas también con la cuenta en eliminación pendiente: ver el perfil y cancelar.
	yo.Get("", h.profile)
	yo.Post("/cancelar-eliminacion", h.cancelDeletion)

	yo.Patch("", active(h.patchProfile)...)
	yo.Delete("", active(h.requestDeletion)...)
	yo.Put("/correo", active(h.changeEmail)...)
	yo.Put("/gustos", active(h.setTastes)...)
	yo.Patch("/preferencias", active(h.patchPreferences)...)
	yo.Patch("/privacidad", active(h.patchPrivacy)...)
	yo.Post("/consentimientos", active(h.acceptConsent)...)
	yo.Put("/telefono", active(h.setPhone)...)
	yo.Post("/modo", active(h.setMode)...)
	yo.Post("/exportar", active(h.requestExport)...)
	yo.Delete("/historial", active(h.purgeHistory)...)
}

// bind lee el JSON y lo valida: cuerpo ilegible → 400, reglas → 422.
func (h *Handler) bind(c *fiber.Ctx, dto any) error {
	if err := c.BodyParser(dto); err != nil {
		return apperr.BadRequest("cuerpo JSON inválido")
	}
	return h.validator.Struct(dto)
}

func claims(c *fiber.Ctx) *middleware.Claims {
	cl, _ := middleware.ClaimsFrom(c)
	return cl
}

// ifMatch lee la versión de `If-Match` (`3`, `"3"` o `W/"3"`). Falta o mal formada = 400.
func ifMatch(c *fiber.Ctx) (int, error) {
	raw := strings.Trim(strings.TrimPrefix(strings.TrimSpace(c.Get(fiber.HeaderIfMatch)), "W/"), `"`)
	version, err := strconv.Atoi(raw)
	if err != nil || version < 1 {
		return 0, apperr.BadRequest("falta la cabecera If-Match con la versión del perfil")
	}
	return version, nil
}

// sendProfile: el ETag de versión es para If-Match (nv-05); datos del usuario, sin caché
// ni 304 (nv-17).
func sendProfile(c *fiber.Ctx, p ProfileResponse) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	c.Set(fiber.HeaderETag, `"`+strconv.Itoa(p.Version)+`"`)
	return c.JSON(p)
}

func accepted(c *fiber.Ctx) error {
	c.Status(fiber.StatusAccepted) // sin cuerpo: SendStatus escribiría "Accepted" como texto
	return nil
}

func (h *Handler) register(c *fiber.Ctx) error {
	var req RegisterRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	res, err := h.svc.Register(c.UserContext(), RegisterInput{Request: req, IP: middleware.ClientIP(c)})
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(res)
}

func (h *Handler) emailAvailable(c *fiber.Ctx) error {
	var q EmailQuery
	if err := c.QueryParser(&q); err != nil {
		return apperr.BadRequest("parámetros inválidos")
	}
	if err := h.validator.Struct(q); err != nil {
		return err
	}
	available, err := h.svc.EmailAvailable(c.UserContext(), q.Email)
	if err != nil {
		return err
	}
	return c.JSON(EmailAvailableResponse{Available: available})
}

func (h *Handler) profile(c *fiber.Ctx) error {
	p, err := h.svc.Profile(c.UserContext(), claims(c))
	if err != nil {
		return err
	}
	return sendProfile(c, p)
}

func (h *Handler) patchProfile(c *fiber.Ctx) error {
	version, err := ifMatch(c)
	if err != nil {
		return err
	}
	var req PatchProfileRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	p, err := h.svc.PatchProfile(c.UserContext(), claims(c), version, req)
	if err != nil {
		return err
	}
	return sendProfile(c, p)
}

func (h *Handler) changeEmail(c *fiber.Ctx) error {
	var req ChangeEmailRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	p, err := h.svc.ChangeEmail(c.UserContext(), claims(c), req)
	if err != nil {
		return err
	}
	return sendProfile(c, p)
}

func (h *Handler) setTastes(c *fiber.Ctx) error {
	var req TastesRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	t, err := h.svc.SetTastes(c.UserContext(), claims(c), req)
	if err != nil {
		return err
	}
	return c.JSON(t)
}

func (h *Handler) patchPreferences(c *fiber.Ctx) error {
	var req PreferencesRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	p, err := h.svc.PatchPreferences(c.UserContext(), claims(c), req)
	if err != nil {
		return err
	}
	return c.JSON(p)
}

func (h *Handler) patchPrivacy(c *fiber.Ctx) error {
	var req PrivacyRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	p, err := h.svc.PatchPrivacy(c.UserContext(), claims(c), req)
	if err != nil {
		return err
	}
	return c.JSON(p)
}

func (h *Handler) acceptConsent(c *fiber.Ctx) error {
	var req ConsentRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	out, err := h.svc.AcceptConsent(c.UserContext(), claims(c), req)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(out)
}

func (h *Handler) setPhone(c *fiber.Ctx) error {
	var req PhoneRequest
	if err := c.BodyParser(&req); err != nil {
		return apperr.BadRequest("cuerpo JSON inválido")
	}
	req.Phone = NormalizePhone(req.Phone)
	if err := h.validator.Struct(req); err != nil {
		return err
	}
	if err := h.svc.SetPhone(c.UserContext(), claims(c), req.Phone); err != nil {
		return err
	}
	return c.JSON(PhoneResponse{PhoneRegistered: true})
}

func (h *Handler) setMode(c *fiber.Ctx) error {
	var req ModeRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	m, err := h.svc.SetMode(c.UserContext(), claims(c), req.Mode)
	if err != nil {
		return err
	}
	return c.JSON(m)
}

func (h *Handler) requestExport(c *fiber.Ctx) error {
	if err := h.svc.RequestExport(c.UserContext(), claims(c)); err != nil {
		return err
	}
	return accepted(c)
}

func (h *Handler) downloadExport(c *fiber.Ctx) error {
	var req DownloadExportRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	data, err := h.svc.DownloadExport(c.UserContext(), req.Token)
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
	c.Set(fiber.HeaderContentDisposition, `attachment; filename="mis-datos-wherever.json"`)
	return c.SendString(data)
}

func (h *Handler) purgeHistory(c *fiber.Ctx) error {
	if err := h.svc.PurgeHistory(c.UserContext(), claims(c)); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) requestDeletion(c *fiber.Ctx) error {
	var req DeleteRequest
	if err := h.bind(c, &req); err != nil {
		return err
	}
	res, err := h.svc.RequestDeletion(c.UserContext(), claims(c), req.Confirmation)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusAccepted).JSON(res)
}

func (h *Handler) cancelDeletion(c *fiber.Ctx) error {
	p, err := h.svc.CancelDeletion(c.UserContext(), claims(c))
	if err != nil {
		return err
	}
	return sendProfile(c, p)
}

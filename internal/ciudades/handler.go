package ciudades

import (
	"github.com/gofiber/fiber/v2"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/etag"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

// AutocompleteQuery es `?q=&limite=` de GET /ciudades/autocompletar.
type AutocompleteQuery struct {
	Q     string `query:"q" validate:"required,max=60"`
	Limit int    `query:"limite" validate:"min=1,max=50"`
}

// catalogCacheControl es la frescura del catálogo público (nv-17); en dev CloudFront no
// cachea, así que vale sobre todo para el cliente.
const catalogCacheControl = "public, max-age=60"

// Handler traduce HTTP ↔ service.
type Handler struct {
	svc       *Service
	validator *validation.Validator
}

// NewHandler recibe el service y el validador por constructor.
func NewHandler(svc *Service, v *validation.Validator) *Handler {
	return &Handler{svc: svc, validator: v}
}

// Routes registra /api/v1/ciudades. Cumple httpapi.Module.
func (h *Handler) Routes(v1 fiber.Router, _ *middleware.Limiter) {
	v1.Get("/ciudades/autocompletar", h.autocomplete)
}

func (h *Handler) autocomplete(c *fiber.Ctx) error {
	q := AutocompleteQuery{Limit: DefaultLimit}
	if err := c.QueryParser(&q); err != nil {
		return apperr.BadRequest("parámetros inválidos")
	}
	if err := h.validator.Struct(q); err != nil {
		return err
	}
	out, err := h.svc.Autocomplete(c.UserContext(), q.Q, q.Limit)
	if err != nil {
		return err
	}
	return sendCatalog(c, out)
}

// sendCatalog serializa una vez y responde con ETag débil sobre esos bytes (nv-17). Con
// If-None-Match vigente responde 304 sin cuerpo; el 304 no evita la consulta (eso es nv-18).
func sendCatalog(c *fiber.Ctx, out any) error {
	body, err := c.App().Config().JSONEncoder(out)
	if err != nil {
		return err
	}
	tag := etag.De(body)
	c.Set(fiber.HeaderCacheControl, catalogCacheControl)
	c.Set(fiber.HeaderETag, tag)
	if etag.Coincide(c.Get(fiber.HeaderIfNoneMatch), tag) {
		c.Status(fiber.StatusNotModified) // sin cuerpo: SendStatus escribiría el texto del estado
		return nil
	}
	c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
	return c.Send(body)
}

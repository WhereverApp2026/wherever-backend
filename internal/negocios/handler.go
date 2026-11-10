package negocios

import (
	"bytes"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

// Política de la ruta admin (docs/rate-limit-matrix.md).
const (
	adminPolicy = "admin"
	adminLimit  = 200
	adminWindow = time.Minute
)

// Handler traduce HTTP ↔ service.
type Handler struct {
	svc *Service
}

// NewHandler recibe el service y el validador por constructor (el DTO de la semilla no
// usa etiquetas `validate`: las reglas son de negocio y se acumulan por fila).
func NewHandler(svc *Service, _ *validation.Validator) *Handler {
	return &Handler{svc: svc}
}

// Routes registra /api/v1/categorias y la semilla admin. Cumple httpapi.Module.
func (h *Handler) Routes(v1 fiber.Router, limiter *middleware.Limiter) {
	v1.Get("/categorias", h.categories)
	v1.Post("/admin/negocios/importar",
		limiter.Window(adminPolicy, adminLimit, adminWindow),
		middleware.RequireRoles(middleware.RoleAdmin),
		h.importar)
}

func (h *Handler) categories(c *fiber.Ctx) error {
	out, err := h.svc.Facets(c.UserContext())
	if err != nil {
		return err
	}
	// Sin ETag ni 304 hasta NV-18: los conteos no se cachean.
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.JSON(out)
}

func (h *Handler) importar(c *fiber.Ctx) error {
	claims, ok := middleware.ClaimsFrom(c)
	if !ok {
		return apperr.Unauthorized("autenticación requerida")
	}
	reader, format, err := importBody(c)
	if err != nil {
		return err
	}
	report, err := h.svc.Import(c.UserContext(), claims.Subject, reader, format)
	if err != nil {
		return err
	}
	return c.JSON(report)
}

// importBody acepta multipart/form-data con el campo `archivo` o application/json con un
// arreglo. El formato del archivo se decide por su extensión.
func importBody(c *fiber.Ctx) (io.Reader, string, error) {
	contentType := c.Get(fiber.HeaderContentType)
	if strings.HasPrefix(contentType, "multipart/form-data") {
		file, err := c.FormFile("archivo")
		if err != nil {
			return nil, "", apperr.BadRequest("falta el archivo")
		}
		opened, err := file.Open()
		if err != nil {
			return nil, "", apperr.BadRequest("no se pudo leer el archivo")
		}
		defer func() { _ = opened.Close() }()
		data, err := io.ReadAll(opened)
		if err != nil {
			return nil, "", apperr.BadRequest("no se pudo leer el archivo")
		}
		format := strings.TrimPrefix(strings.ToLower(filepath.Ext(file.Filename)), ".")
		if format != "csv" && format != "json" {
			return nil, "", apperr.BadRequest("el archivo debe ser .csv o .json")
		}
		return bytes.NewReader(data), format, nil
	}
	if c.Is("json") {
		return bytes.NewReader(c.Body()), "json", nil
	}
	return nil, "", apperr.BadRequest("cuerpo no soportado: usa multipart/form-data o application/json")
}

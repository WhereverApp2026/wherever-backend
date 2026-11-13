package errores

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

// Límite por usuario del reporte (nv-64, docs/rate-limit-matrix.md).
const (
	reportPolicy = "errores_reporte"
	reportLimit  = 30
	reportWindow = time.Hour
)

// Handler traduce HTTP ↔ service. No toca la base ni atrapa pánicos (test de arquitectura).
type Handler struct {
	svc       *Service
	validator *validation.Validator
}

// NewHandler recibe el service y el validador por constructor.
func NewHandler(svc *Service, v *validation.Validator) *Handler {
	return &Handler{svc: svc, validator: v}
}

// Routes registra POST /api/v1/errores y GET /api/v1/admin/errores. Cumple httpapi.Module.
// El reporte no pide cuenta activa: una cuenta en eliminación también puede fallar.
func (h *Handler) Routes(v1 fiber.Router, limiter *middleware.Limiter) {
	report := []fiber.Handler{middleware.RequireAuth()}
	if limiter != nil {
		report = append(report, limiter.WindowUser(reportPolicy, reportLimit, reportWindow))
	}
	v1.Post("/errores", append(report, h.report)...)
	v1.Get("/admin/errores", middleware.RequireRoles(middleware.RoleAdmin), h.list)
}

// report decodifica en estricto: JSON ilegible o con campos de más → 400; reglas → 422.
func (h *Handler) report(c *fiber.Ctx) error {
	var req ReportRequest
	dec := json.NewDecoder(bytes.NewReader(c.Body()))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil || dec.More() {
		return apperr.BadRequest("cuerpo JSON inválido")
	}
	if err := h.validator.Struct(req); err != nil {
		return err
	}
	if err := h.svc.Report(c.UserContext(), req); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusAccepted)
}

func (h *Handler) list(c *fiber.Ctx) error {
	q := NewListQuery()
	if err := c.QueryParser(&q); err != nil {
		return apperr.BadRequest("parámetros inválidos")
	}
	if err := h.validator.Struct(q); err != nil {
		return err
	}
	out, err := h.svc.List(c.UserContext(), q)
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.JSON(out)
}

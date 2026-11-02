package middleware

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

// ErrorResponse es el cuerpo uniforme de error (constitución §4).
type ErrorResponse struct {
	Code    string              `json:"codigo"`
	Message string              `json:"mensaje"`
	Fields  []apperr.FieldError `json:"campos"`
	Detail  *string             `json:"detalle,omitempty"`
	TraceID string              `json:"traza_id"`
}

// NewErrorHandler devuelve el único manejador de errores de la app.
// showDetail es falso en prod: `detalle` nunca sale ahí.
func NewErrorHandler(showDetail bool, log *slog.Logger) fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		return WriteError(c, err, showDetail, log)
	}
}

// WriteError traduce err a su código HTTP y escribe el cuerpo uniforme.
func WriteError(c *fiber.Ctx, err error, showDetail bool, log *slog.Logger) error {
	appErr := resolve(err)
	status, code := appErr.Kind.Status(), appErr.Kind.Code()
	// Un código HTTP de Fiber sin tipo propio (405, 413...) se conserva tal cual.
	if fe := (*fiber.Error)(nil); errors.As(err, &fe) && appErr.Kind == apperr.KindInternal && fe.Code < http.StatusInternalServerError {
		status, code = fe.Code, "error_http"
	}
	if status >= http.StatusInternalServerError {
		log.Error("error interno", "request_id", RequestIDFrom(c), "error", err.Error())
	}

	body := ErrorResponse{
		Code:    code,
		Message: appErr.Message,
		Fields:  appErr.Fields,
		TraceID: RequestIDFrom(c),
	}
	if body.Fields == nil {
		body.Fields = []apperr.FieldError{}
	}
	if showDetail && appErr.Detail != "" {
		body.Detail = &appErr.Detail
	}
	if appErr.RetryAfter > 0 {
		c.Set(fiber.HeaderRetryAfter, strconv.Itoa(appErr.RetryAfter))
	}
	return c.Status(status).JSON(body)
}

// resolve convierte cualquier error en un *apperr.Error; lo desconocido es 500.
func resolve(err error) *apperr.Error {
	var appErr *apperr.Error
	if errors.As(err, &appErr) {
		return appErr
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return apperr.Timeout("la petición excedió el tiempo máximo").WithDetail(err.Error())
	}
	var fe *fiber.Error
	if errors.As(err, &fe) {
		return fromFiber(fe)
	}
	return &apperr.Error{Kind: apperr.KindInternal, Message: "error interno", Detail: err.Error()}
}

// fromFiber traduce los errores propios de Fiber (ruta inexistente, cuerpo grande, etc.).
func fromFiber(fe *fiber.Error) *apperr.Error {
	if kind, ok := apperr.KindFromStatus(fe.Code); ok {
		return &apperr.Error{Kind: kind, Message: kind.DefaultMessage()}
	}
	return &apperr.Error{Kind: apperr.KindInternal, Message: http.StatusText(fe.Code)}
}

// Errors es la etapa Recover/ErrorHandler: convierte pánicos en 500, escribe el error
// uniforme y registra la línea de acceso con su request_id.
func Errors(handler fiber.ErrorHandler, log *slog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) (err error) {
		start := time.Now()
		defer func() {
			if r := recover(); r != nil {
				err = handler(c, &apperr.Error{Kind: apperr.KindInternal, Message: "error interno", Detail: fmt.Sprint(r)})
			}
			log.Info("petición",
				"request_id", RequestIDFrom(c),
				"metodo", c.Method(),
				"ruta", c.Path(),
				"estado", c.Response().StatusCode(),
				"duracion_ms", time.Since(start).Milliseconds(),
			)
		}()
		if err = c.Next(); err != nil {
			err = handler(c, err)
		}
		return err
	}
}

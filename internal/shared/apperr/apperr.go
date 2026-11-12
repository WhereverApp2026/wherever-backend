// Package apperr define los errores tipados de la API (constitución §5).
package apperr

import "net/http"

// Kind clasifica un error y decide su código HTTP.
type Kind int

// Tipos de error en el orden de la constitución §5.
const (
	KindInternal Kind = iota
	KindBadRequest
	KindUnauthorized
	KindForbidden
	KindNotFound
	KindTimeout
	KindConflict
	KindGone
	KindValidation
	KindLocked
	KindRateLimited
	KindUnavailable
	// KindUpgradeRequired va al final para no mover los valores anteriores (nv-16).
	KindUpgradeRequired
)

type kindInfo struct {
	status  int
	code    string
	message string
}

var kinds = map[Kind]kindInfo{
	KindInternal:     {http.StatusInternalServerError, "error_interno", "error interno"},
	KindBadRequest:   {http.StatusBadRequest, "solicitud_invalida", "petición mal formada"},
	KindUnauthorized: {http.StatusUnauthorized, "no_autenticado", "autenticación requerida"},
	KindForbidden:    {http.StatusForbidden, "prohibido", "sin permiso para este recurso"},
	KindNotFound:     {http.StatusNotFound, "no_encontrado", "recurso no encontrado"},
	KindTimeout:      {http.StatusRequestTimeout, "tiempo_agotado", "la petición excedió el tiempo máximo"},
	KindConflict:     {http.StatusConflict, "conflicto", "conflicto con el estado actual"},
	KindGone:         {http.StatusGone, "vencido", "el recurso ya no está disponible"},
	KindValidation:   {http.StatusUnprocessableEntity, "validacion", "datos inválidos"},
	KindLocked:       {http.StatusLocked, "bloqueado", "recurso bloqueado"},
	KindRateLimited:  {http.StatusTooManyRequests, "demasiadas_peticiones", "demasiadas peticiones, intenta más tarde"},
	KindUnavailable:  {http.StatusServiceUnavailable, "no_disponible", "servicio no disponible"},
	KindUpgradeRequired: {http.StatusUpgradeRequired, "actualizacion_requerida", "actualiza la app"},
}

// Status devuelve el código HTTP del tipo; un tipo desconocido es 500.
func (k Kind) Status() int {
	if info, ok := kinds[k]; ok {
		return info.status
	}
	return http.StatusInternalServerError
}

// DefaultMessage devuelve el mensaje público por defecto del tipo.
func (k Kind) DefaultMessage() string {
	if info, ok := kinds[k]; ok {
		return info.message
	}
	return kinds[KindInternal].message
}

// KindFromStatus busca el tipo que corresponde a un código HTTP.
func KindFromStatus(status int) (Kind, bool) {
	for kind, info := range kinds {
		if info.status == status {
			return kind, true
		}
	}
	return KindInternal, false
}

// Code devuelve el `codigo` público del tipo.
func (k Kind) Code() string {
	if info, ok := kinds[k]; ok {
		return info.code
	}
	return kinds[KindInternal].code
}

// FieldError describe un campo inválido.
type FieldError struct {
	Field   string `json:"campo"`
	Message string `json:"mensaje"`
}

// Error es un error de la API con mensaje público.
type Error struct {
	Kind       Kind
	Message    string
	Fields     []FieldError
	Detail     string
	RetryAfter int
}

func (e *Error) Error() string { return e.Kind.Code() + ": " + e.Message }

// WithDetail agrega un detalle técnico que solo se muestra fuera de prod.
func (e *Error) WithDetail(detail string) *Error {
	e.Detail = detail
	return e
}

func newError(kind Kind, message string) *Error {
	return &Error{Kind: kind, Message: message}
}

// BadRequest: petición mal formada (400).
func BadRequest(message string) *Error { return newError(KindBadRequest, message) }

// Unauthorized: falta autenticación o no es válida (401).
func Unauthorized(message string) *Error { return newError(KindUnauthorized, message) }

// Forbidden: autenticado sin permiso (403).
func Forbidden(message string) *Error { return newError(KindForbidden, message) }

// NotFound: recurso inexistente o ajeno (404).
func NotFound(message string) *Error { return newError(KindNotFound, message) }

// Timeout: la operación excedió su tiempo (408).
func Timeout(message string) *Error { return newError(KindTimeout, message) }

// Conflict: choque de versión o de estado (409).
func Conflict(message string) *Error { return newError(KindConflict, message) }

// Gone: recurso vencido (410).
func Gone(message string) *Error { return newError(KindGone, message) }

// Validation: falla de validación o de regla de negocio (422).
func Validation(message string, fields ...FieldError) *Error {
	e := newError(KindValidation, message)
	e.Fields = fields
	return e
}

// Locked: recurso bloqueado (423).
func Locked(message string) *Error { return newError(KindLocked, message) }

// RateLimited: demasiadas peticiones (429) con segundos de espera.
func RateLimited(retryAfter int) *Error {
	e := newError(KindRateLimited, KindRateLimited.DefaultMessage())
	e.RetryAfter = retryAfter
	return e
}

// Unavailable: dependencia caída, respuesta fail-closed (503).
func Unavailable(message string) *Error { return newError(KindUnavailable, message) }

// UpgradeRequired: la app es más vieja que la versión mínima (426, nv-16).
func UpgradeRequired(minima string) *Error {
	return newError(KindUpgradeRequired, "actualiza la app a la versión "+minima+" o superior")
}

// Maintenance: modo mantenimiento (503) con `Retry-After` en segundos (nv-16).
func Maintenance(retryAfter int) *Error {
	e := Unavailable("en mantenimiento")
	e.RetryAfter = retryAfter
	return e
}

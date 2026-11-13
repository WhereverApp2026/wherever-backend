// Package validation valida los DTOs con go-playground/validator. Todas las reglas
// propias se registran aquí, en New; los módulos solo declaran etiquetas `validate`.
package validation

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/go-playground/validator/v10"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
)

// rules son las reglas propias de la API, por etiqueta.
var rules = map[string]validator.Func{
	"cursor":   validCursor,
	"password": validPassword,
	"adult":    validAdult,
	"e164":     validE164,
	"hhmm":     validHHMM,
	"objectid": validObjectID,
	"username": validUsername,
	"semver":   validSemver,
	"modulo":   validModule,
	"traceid":  validTraceID,
}

// Formatos de las reglas propias (hoja A1).
const (
	// DateLayout es el formato de fecha de los DTOs (`fecha_nacimiento`): ISO 8601 sin hora.
	DateLayout = "2006-01-02"
	adultAge   = 18
)

var (
	e164Pattern     = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)
	hhmmPattern     = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)
	objectIDPattern = regexp.MustCompile(`^[0-9a-f]{24}$`)
	usernamePattern = regexp.MustCompile(`^[a-z0-9_]{3,20}$`)
	// Reporte de errores de la app (nv-64).
	semverPattern  = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$`)
	modulePattern  = regexp.MustCompile(`^[a-z_]{2,40}$`)
	traceIDPattern = regexp.MustCompile(`^[A-Za-z0-9-]{1,64}$`)
)

// reservedUsernames no se pueden tomar: suplantarían a la plataforma o a su personal (nv-34).
var reservedUsernames = map[string]bool{
	"admin": true, "administrador": true, "wherever": true, "soporte": true, "ayuda": true,
	"root": true, "sistema": true, "moderador": true, "oficial": true, "api": true, "null": true,
}

// Política de contraseña (hojas A1 y J1): 8 caracteres o más con al menos una letra y un
// número. El tope evita hashear entradas enormes con Argon2id.
const (
	passwordMinLen = 8
	passwordMaxLen = 128
)

// Validator envuelve al validador ya configurado; es seguro para uso concurrente.
type Validator struct {
	v *validator.Validate
}

// New crea el validador y registra todas las reglas propias. Es el único punto de registro.
func New() (*Validator, error) {
	v := validator.New(validator.WithRequiredStructEnabled())
	v.RegisterTagNameFunc(fieldName)
	for tag, fn := range rules {
		if err := v.RegisterValidation(tag, fn); err != nil {
			return nil, fmt.Errorf("registrar regla %q: %w", tag, err)
		}
	}
	return &Validator{v: v}, nil
}

// Struct valida un DTO. Las fallas salen como apperr.Validation (422) con un campo por regla.
func (val *Validator) Struct(dto any) error {
	err := val.v.Struct(dto)
	if err == nil {
		return nil
	}
	var fieldErrs validator.ValidationErrors
	if !errors.As(err, &fieldErrs) {
		return apperr.BadRequest("petición mal formada").WithDetail(err.Error())
	}
	fields := make([]apperr.FieldError, 0, len(fieldErrs))
	for _, fe := range fieldErrs {
		fields = append(fields, apperr.FieldError{Field: fe.Field(), Message: message(fe)})
	}
	return apperr.Validation("datos inválidos", fields...)
}

// fieldName usa el nombre público del campo (query o json) para que coincida con la petición.
func fieldName(f reflect.StructField) string {
	for _, tag := range []string{"query", "json"} {
		name, _, _ := strings.Cut(f.Tag.Get(tag), ",")
		if name != "" && name != "-" {
			return name
		}
	}
	return f.Name
}

// messages son los textos por etiqueta; `%s` se reemplaza por el parámetro de la regla.
var messages = map[string]string{ // #nosec G101 -- mensajes de validación, no credenciales
	"required":   "es obligatorio",
	"min":        "debe ser al menos %s",
	"max":        "debe ser como máximo %s",
	"oneof":      "debe ser uno de: %s",
	"startswith": "debe empezar con %s",
	"cursor":     "cursor inválido",
	"password":   "debe tener de 8 a 128 caracteres con al menos una letra y un número",
	"email":      "no es un correo válido",
	"adult":      "debes tener 18 años o más (fecha AAAA-MM-DD)",
	"e164":       "debe estar en formato internacional, p. ej. +5215512345678",
	"hhmm":       "debe tener el formato HH:MM",
	"objectid":   "no es un identificador válido",
	"username":   "debe tener de 3 a 20 caracteres: minúsculas, números o guion bajo, y no estar reservado",
	"semver":     "debe ser una versión como 1.4.0",
	"modulo":     "debe tener de 2 a 40 minúsculas o guion bajo",
	"traceid":    "debe tener hasta 64 letras, números o guiones",
	"url":        "no es una URL válida",
	"gtefield":   "debe ser mayor o igual que el mínimo",
	"unique":     "tiene elementos repetidos",
}

func message(fe validator.FieldError) string {
	text, ok := messages[fe.Tag()]
	if !ok {
		return "no es válido"
	}
	if strings.Contains(text, "%s") {
		return fmt.Sprintf(text, fe.Param())
	}
	return text
}

// validSemver acepta MAYOR.MENOR.PARCHE con prerelease y build opcionales (semver.org).
func validSemver(fl validator.FieldLevel) bool { return semverPattern.MatchString(fl.Field().String()) }

func validModule(fl validator.FieldLevel) bool { return modulePattern.MatchString(fl.Field().String()) }

func validTraceID(fl validator.FieldLevel) bool {
	return traceIDPattern.MatchString(fl.Field().String())
}

func validCursor(fl validator.FieldLevel) bool {
	return pagination.Valid(fl.Field().String())
}

func validPassword(fl validator.FieldLevel) bool {
	pw := fl.Field().String()
	n := len([]rune(pw))
	if n < passwordMinLen || n > passwordMaxLen {
		return false
	}
	return strings.IndexFunc(pw, unicode.IsLetter) >= 0 && strings.IndexFunc(pw, unicode.IsDigit) >= 0
}

// IsAdult dice si alguien nacido en `birth` tiene 18 años o más en la fecha `today` (UTC).
func IsAdult(birth, today time.Time) bool {
	return !birth.AddDate(adultAge, 0, 0).After(today)
}

// validAdult: fecha AAAA-MM-DD válida y mayoría de edad con la fecha del servidor en UTC.
func validAdult(fl validator.FieldLevel) bool {
	birth, err := time.Parse(DateLayout, fl.Field().String())
	if err != nil {
		return false
	}
	return IsAdult(birth, time.Now().UTC())
}

func validE164(fl validator.FieldLevel) bool { return e164Pattern.MatchString(fl.Field().String()) }

func validHHMM(fl validator.FieldLevel) bool { return hhmmPattern.MatchString(fl.Field().String()) }

func validObjectID(fl validator.FieldLevel) bool {
	return objectIDPattern.MatchString(fl.Field().String())
}

// validUsername acepta 3 a 20 minúsculas, números o guion bajo que no estén reservados. Solo
// minúsculas: así el índice único ya es único sin importar mayúsculas.
func validUsername(fl validator.FieldLevel) bool {
	name := fl.Field().String()
	return usernamePattern.MatchString(name) && !reservedUsernames[name]
}

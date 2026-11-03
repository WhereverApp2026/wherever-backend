// Package validation valida los DTOs con go-playground/validator. Todas las reglas
// propias se registran aquí, en New; los módulos solo declaran etiquetas `validate`.
package validation

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"unicode"

	"github.com/go-playground/validator/v10"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
)

// rules son las reglas propias de la API, por etiqueta.
var rules = map[string]validator.Func{
	"cursor":   validCursor,
	"password": validPassword,
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

func message(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "es obligatorio"
	case "min":
		return "debe ser al menos " + fe.Param()
	case "max":
		return "debe ser como máximo " + fe.Param()
	case "cursor":
		return "cursor inválido"
	case "password":
		return "debe tener de 8 a 128 caracteres con al menos una letra y un número"
	case "email":
		return "no es un correo válido"
	default:
		return "no es válido"
	}
}

func validCursor(fl validator.FieldLevel) bool {
	_, _, err := pagination.Decode(fl.Field().String())
	return err == nil
}

func validPassword(fl validator.FieldLevel) bool {
	pw := fl.Field().String()
	n := len([]rune(pw))
	if n < passwordMinLen || n > passwordMaxLen {
		return false
	}
	return strings.IndexFunc(pw, unicode.IsLetter) >= 0 && strings.IndexFunc(pw, unicode.IsDigit) >= 0
}

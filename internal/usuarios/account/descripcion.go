package account

import (
	"context"
	"strings"
	"unicode"

	"github.com/WhereverApp2026/wherever-backend/internal/moderacion"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

// Moderador revisa la descripción del perfil (nv-77) con las reglas de nv-42/nv-71.
type Moderador interface {
	Evaluar(ctx context.Context, c moderacion.Contenido) moderacion.Resultado
}

// WithModeracion conecta el moderador de textos; sin él la descripción no se modera.
func (s *Service) WithModeracion(m Moderador) *Service {
	s.moderador = m
	return s
}

// LimpiarDescripcion deja la descripción en una línea: sin caracteres de control y con los
// espacios juntos. La validación del DTO ya fijó el máximo de 160 caracteres.
func LimpiarDescripcion(raw string) string {
	limpio := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
			return ' '
		}
		return r
	}, raw)
	return strings.Join(strings.Fields(limpio), " ")
}

// descripcion limpia y modera la descripción. Rechazada = 422; dudosa se guarda (el mismo
// criterio que los reportes de negocio: la cuarentena es para contenido que alguien revisa).
func (s *Service) descripcion(ctx context.Context, raw string) (string, error) {
	limpio := LimpiarDescripcion(raw)
	if limpio == "" || s.moderador == nil {
		return limpio, nil
	}
	if s.moderador.Evaluar(ctx, moderacion.Contenido{Texto: limpio}).Veredicto == moderacion.VeredictoRechazado {
		return "", apperr.Validation("datos inválidos", apperr.FieldError{Field: "descripcion", Message: "contenido no permitido"})
	}
	return limpio, nil
}

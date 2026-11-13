package moderacion

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"github.com/WhereverApp2026/wherever-backend/internal/ia"
)

// MaxRunasTexto es el largo máximo que se evalúa; más largo queda en cuarentena.
const MaxRunasTexto = 4000

// PuntajeDatosPersonales es lo que suma un teléfono, correo, enlace o usuario en el texto:
// cae en cuarentena con los umbrales por defecto para que un administrador lo revise.
const PuntajeDatosPersonales = 60

// Reglas son los umbrales y las palabras bloqueadas. Por debajo de Cuarentena aprueba, desde
// Cuarentena y por debajo de Rechazo queda en cuarentena, y desde Rechazo rechaza.
type Reglas struct {
	Cuarentena float64
	Rechazo    float64
	// Palabras son palabras o frases; se comparan sin acentos ni mayúsculas y por palabra
	// completa. Una coincidencia vale 100.
	Palabras []string
}

// ReglasPorDefecto son las de la spec: 40 y 80, con frases de venta de drogas sin ambigüedad.
func ReglasPorDefecto() Reglas {
	return Reglas{
		Cuarentena: 40,
		Rechazo:    80,
		Palabras:   []string{"fentanilo", "metanfetamina", "vendo mota", "vendo cristal", "vendo coca", "vendo perico"},
	}
}

// Validar exige 0 < Cuarentena < Rechazo <= 100.
func (r Reglas) Validar() error {
	if r.Cuarentena <= 0 || r.Cuarentena >= r.Rechazo || r.Rechazo > 100 {
		return fmt.Errorf("%w: se pide 0 < cuarentena (%v) < rechazo (%v) <= 100", ErrReglasInvalid, r.Cuarentena, r.Rechazo)
	}
	return nil
}

// Clasificar convierte el puntaje más alto en veredicto.
func (r Reglas) Clasificar(puntaje float64) Veredicto {
	switch {
	case puntaje >= r.Rechazo:
		return VeredictoRechazado
	case puntaje >= r.Cuarentena:
		return VeredictoCuarentena
	default:
		return VeredictoAprobado
	}
}

// Locales son los puntajes que no necesitan proveedor: palabras bloqueadas y datos personales.
func (r Reglas) Locales(texto string) map[string]float64 {
	out := map[string]float64{}
	limpio := " " + plano(texto) + " "
	for _, p := range r.Palabras {
		if p = plano(p); p != "" && strings.Contains(limpio, " "+p+" ") {
			out[FiltroPalabras] = 100
			break
		}
	}
	if ia.Anonimizar(texto) != texto {
		out[FiltroDatosPersonales] = PuntajeDatosPersonales
	}
	return out
}

var noLetras = regexp.MustCompile(`[^\p{L}\p{N}]+`)

// plano deja el texto en minúsculas, sin acentos y con un espacio entre palabras.
func plano(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	sin, _, err := transform.String(t, strings.ToLower(s))
	if err != nil {
		sin = strings.ToLower(s)
	}
	return strings.TrimSpace(noLetras.ReplaceAllString(sin, " "))
}

// Fijas son reglas que no cambian en ejecución, hasta que nv-43 las lea de moderacion_reglas.
type Fijas Reglas

// Reglas devuelve las reglas fijas.
func (f Fijas) Reglas(context.Context) (Reglas, error) { return Reglas(f), nil }

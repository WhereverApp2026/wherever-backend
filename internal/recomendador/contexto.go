// Package recomendador elige negocios sin datos de entrenamiento (spec nv-23): filtros
// duros en la consulta, puntaje ponderado de 0 a 100 y una explicación legible. Es el plan
// B del asistente (nv-31) y la fuente de candidatos de inicio (nv-24).
package recomendador

import (
	"slices"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/text"
)

// Radio por defecto y máximo, los mismos de GET /lugares (nv-21).
const (
	RadioDefaultM = 5000.0
	RadioMaxM     = 20000.0
)

// Contexto es lo que se sabe de quien pide y del momento. Lo arma quien llama (inicio o
// el asistente) con los gustos de A1 y lo que diga la petición.
type Contexto struct {
	Lat, Lng float64
	RadioM   float64 // 0 = RadioDefaultM; se recorta a RadioMaxM
	Momento  time.Time

	PresupuestoMin, PresupuestoMax int // MXN por persona; PresupuestoMax 0 = sin dato
	Gustos                         []string

	// Alergenos son ids del catálogo de negocios. AlergiaSinCatalogo dice que el usuario
	// declaró algo que no está en el catálogo: solo pasan negocios con «ninguno».
	Alergenos          []string
	AlergiaSinCatalogo bool
	Dietas             []string // ids del catálogo de dietas

	SillaRuedas, ConNinos, ConMascota bool

	// Excluidos son los negocios marcados con «No me interesa».
	Excluidos []bson.ObjectID
}

// radio devuelve el radio efectivo.
func (c Contexto) radio() float64 {
	switch {
	case c.RadioM <= 0:
		return RadioDefaultM
	case c.RadioM > RadioMaxM:
		return RadioMaxM
	default:
		return c.RadioM
	}
}

// alergenoSinonimos lleva el texto libre de `gustos.alergias` (A1) al catálogo de A2.
var alergenoSinonimos = map[string]string{
	"gluten": "gluten", "trigo": "gluten", "celiaco": "gluten", "celiaquia": "gluten",
	"lacteos": "lacteos", "lacteo": "lacteos", "leche": "lacteos", "lactosa": "lacteos", "queso": "lacteos",
	"huevo": "huevo", "huevos": "huevo",
	"soya": "soya", "soja": "soya",
	"cacahuate": "cacahuate", "cacahuates": "cacahuate", "cacahuete": "cacahuate", "mani": "cacahuate",
	"nueces": "nueces", "nuez": "nueces", "almendra": "nueces", "almendras": "nueces", "pistache": "nueces",
	"pescado": "pescado", "pescados": "pescado",
	"mariscos": "mariscos", "marisco": "mariscos", "camaron": "mariscos", "camarones": "mariscos", "crustaceos": "mariscos",
	"ajonjoli": "ajonjoli", "sesamo": "ajonjoli",
}

// sinAlergia son respuestas que dicen «no tengo».
var sinAlergia = []string{"ninguna", "ninguno", "no", "nada", "sin alergias", "no tengo"}

// ParseAlergias lee `gustos.alergias`. Separa por comas, punto y coma, barras, saltos de
// línea y « y ». Lo que no reconoce no se descarta: marca sinCatalogo, porque recomendar
// algo con un alérgeno no declarado es peor que no recomendar.
func ParseAlergias(raw string) (ids []string, sinCatalogo bool) {
	norm := text.Normalize(raw)
	norm = strings.ReplaceAll(norm, " y ", ",")
	parts := strings.FieldsFunc(norm, func(r rune) bool { return strings.ContainsRune(",;/|\n", r) })
	for _, part := range parts {
		token := strings.TrimSpace(part)
		if token == "" || slices.Contains(sinAlergia, token) {
			continue
		}
		id, ok := alergenoSinonimos[token]
		if !ok {
			sinCatalogo = true
			continue
		}
		if !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	return ids, sinCatalogo
}

// DietasDe toma de `gustos.restricciones` (texto libre de A1) solo las que están en el
// catálogo de dietas; las demás son preferencias, no reglas duras.
func DietasDe(restricciones []string) []string {
	var out []string
	for _, r := range restricciones {
		id := strings.ReplaceAll(text.Normalize(r), " ", "_")
		if negocios.ValidDietOption(id) && !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}

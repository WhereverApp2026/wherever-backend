package busquedas

import (
	"cmp"
	"slices"
	"strings"
	"unicode"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/text"
)

// MaxPalabras acota cuántas palabras de la consulta se comparan.
const MaxPalabras = 8

// Palabras deja solo letras y dígitos, sin acentos ni mayúsculas: «¡Café "La Nena"!» →
// [cafe la nena]. Lo que no es letra ni dígito separa palabras; así ningún operador de
// `$text` (comillas, guion) ni de expresiones regulares llega a MongoDB.
func Palabras(q string) []string {
	words := strings.FieldsFunc(text.Normalize(q), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	if len(words) > MaxPalabras {
		words = words[:MaxPalabras]
	}
	return words
}

// Tolerancia es cuántas letras puede fallar una palabra: ninguna con 3 o menos, una de 4 a
// 7 y dos con 8 o más.
func Tolerancia(word string) int {
	switch n := len([]rune(word)); {
	case n <= 3:
		return 0
	case n <= 7:
		return 1
	default:
		return 2
	}
}

// Distancia es la distancia de edición con transposición de letras vecinas (Damerau, versión
// de alineación óptima): «tcao» → «taco» cuesta 1.
func Distancia(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev2 := make([]int, len(rb)+1)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
			if i > 1 && j > 1 && ra[i-1] == rb[j-2] && ra[i-2] == rb[j-1] {
				cur[j] = min(cur[j], prev2[j-2]+1)
			}
		}
		prev2, prev, cur = prev, cur, prev2
	}
	return prev[len(rb)]
}

// Puntos de una palabra de la consulta contra las del negocio.
const (
	puntosExacta    = 2 // igual o prefijo de una palabra del negocio
	puntosAproxima  = 1 // a la distancia de su tolerancia
	minLetrasPrefij = 3 // «ta» no basta para dar por buena «tacos»
)

// puntos dice qué tan bien coincide `word` con alguna de `tokens`; 0 si con ninguna.
func puntos(word string, tokens []string) int {
	best := 0
	tol := Tolerancia(word)
	n := len([]rune(word))
	for _, t := range tokens {
		if t == word || (n >= minLetrasPrefij && strings.HasPrefix(t, word)) {
			return puntosExacta
		}
		if tol == 0 {
			continue
		}
		rt := []rune(t)
		// Misma palabra con un error, o el principio de una palabra más larga con un error.
		if Distancia(word, t) <= tol || (len(rt) > n && Distancia(word, string(rt[:n])) <= tol) {
			best = puntosAproxima
		}
	}
	return best
}

// tokens son las palabras con las que se puede encontrar un negocio: nombre, etiquetas y giro.
func tokens(b negocios.Business) []string {
	out := Palabras(b.Name)
	for _, tag := range b.Tags {
		out = append(out, Palabras(tag)...)
	}
	return append(out, Palabras(negocios.CategoryName(b.Category))...)
}

// Aproximados ordena los negocios en los que cada palabra de la consulta coincide igual, por
// prefijo o con errores menores; los que no cumplen todas quedan fuera. Primero los de más
// coincidencias exactas, luego por calificación e id.
func Aproximados(words []string, visibles []negocios.Business, excluir map[bson.ObjectID]bool, limite int) []negocios.Business {
	type scored struct {
		b     negocios.Business
		score int
	}
	var hits []scored
	for _, b := range visibles {
		if excluir[b.ID] {
			continue
		}
		toks := tokens(b)
		total := 0
		for _, w := range words {
			p := puntos(w, toks)
			if p == 0 {
				total = 0
				break
			}
			total += p
		}
		if total > 0 {
			hits = append(hits, scored{b: b, score: total})
		}
	}
	slices.SortFunc(hits, func(x, y scored) int {
		return cmp.Or(
			cmp.Compare(y.score, x.score),
			cmp.Compare(y.b.Rating.Promedio, x.b.Rating.Promedio),
			strings.Compare(x.b.ID.Hex(), y.b.ID.Hex()),
		)
	})
	out := make([]negocios.Business, 0, min(len(hits), limite))
	for _, h := range hits[:min(len(hits), limite)] {
		out = append(out, h.b)
	}
	return out
}

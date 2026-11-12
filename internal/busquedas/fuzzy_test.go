package busquedas

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
)

func TestPalabras_OnlyLettersAndDigitsWithoutAccents(t *testing.T) {
	assert.Equal(t, []string{"cafe", "la", "nena"}, Palabras(`¡Café "La Nena"!`))
	assert.Equal(t, []string{"tacos", "pastor"}, Palabras("-tacos .* $pastor"))
	assert.Equal(t, []string{"ne", "null", "where", "1", "1"}, Palabras(`{"$ne": null} || where 1=1`))
	assert.Empty(t, Palabras(`"" -- .* ()`))
	assert.Len(t, Palabras("a b c d e f g h i j"), MaxPalabras)
}

func TestTolerancia_GrowsWithLength(t *testing.T) {
	assert.Equal(t, 0, Tolerancia("bar"))
	assert.Equal(t, 1, Tolerancia("taco"))
	assert.Equal(t, 1, Tolerancia("cafeter"))
	assert.Equal(t, 2, Tolerancia("cafeteria"))
}

func TestDistancia_EditsAndTransposition(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want int
	}{
		{"taco", "taco", 0}, {"tcao", "taco", 1}, {"tago", "taco", 1}, {"tac", "taco", 1},
		{"tacoo", "taco", 1}, {"pizza", "pozole", 4}, {"", "abc", 3}, {"ñu", "nu", 1},
	} {
		assert.Equal(t, c.want, Distancia(c.a, c.b), c.a+"→"+c.b)
	}
}

func biz(name string, rating float64, tags ...string) negocios.Business {
	return negocios.Business{ID: bson.NewObjectID(), Name: name, Category: "restaurante", Tags: tags, Rating: negocios.Calificacion{Promedio: rating}}
}

func found(words []string, visibles []negocios.Business, excluir map[bson.ObjectID]bool, limite int) []negocios.Business {
	out, _ := Aproximados(words, visibles, excluir, limite)
	return out
}

func names(bs []negocios.Business) []string {
	out := make([]string, 0, len(bs))
	for _, b := range bs {
		out = append(out, b.Name)
	}
	return out
}

func TestAproximados_TyposPrefixesAndAllWords(t *testing.T) {
	visibles := []negocios.Business{
		biz("Taquería El Pastor", 4.0, "tacos"),
		biz("Café Tacuba", 4.8),
		biz("Pizzería Napoli", 4.5, "pizza"),
		biz("Bar La Esquina", 3.0),
	}
	// Un error de una letra («pizeria») y un prefijo con error («taqeria»).
	assert.Equal(t, []string{"Pizzería Napoli"}, names(found([]string{"pizeria"}, visibles, nil, 10)))
	assert.Equal(t, []string{"Taquería El Pastor"}, names(found([]string{"taqeria"}, visibles, nil, 10)))
	// Todas las palabras deben coincidir.
	assert.Equal(t, []string{"Taquería El Pastor"}, names(found([]string{"taco", "pastr"}, visibles, nil, 10)))
	// «tacuba» no se parece lo bastante a «tacos».
	assert.Equal(t, []string{"Café Tacuba"}, names(found([]string{"tacuba"}, visibles, nil, 10)))
	// Palabras cortas no aceptan errores.
	assert.Empty(t, found([]string{"baz"}, visibles, nil, 10))
	// El giro también cuenta, y los excluidos y el límite se respetan.
	all := found([]string{"restaurant"}, visibles, nil, 10)
	assert.Len(t, all, 4)
	assert.Equal(t, "Café Tacuba", all[0].Name)
	assert.Len(t, found([]string{"restaurant"}, visibles, map[bson.ObjectID]bool{visibles[1].ID: true}, 2), 2)
	// Solo es aproximado si alguna palabra necesitó un error.
	_, conErrores := Aproximados([]string{"pizzeria"}, visibles, nil, 10)
	assert.False(t, conErrores)
	_, conErrores = Aproximados([]string{"pizeria"}, visibles, nil, 10)
	assert.True(t, conErrores)
	assert.NotContains(t, names(found([]string{"restaurant"}, visibles, map[bson.ObjectID]bool{visibles[1].ID: true}, 10)), "Café Tacuba")
}

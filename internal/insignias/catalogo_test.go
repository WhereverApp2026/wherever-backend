package insignias

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCatalogo_IdsUnicosYMetasPositivas(t *testing.T) {
	vistos := map[string]bool{}
	fuentes := map[string]bool{FuentePlanes: true, FuenteResenas: true, FuenteLugares: true, FuenteAmigos: true, FuentePostales: true}
	for _, d := range Catalogo {
		assert.False(t, vistos[d.ID], "id repetido: %s", d.ID)
		vistos[d.ID] = true
		assert.Positive(t, d.Meta, d.ID)
		assert.True(t, fuentes[d.Fuente], "fuente desconocida en %s", d.ID)
		assert.NotEmpty(t, d.Nombre, d.ID)
		assert.NotEmpty(t, d.Descripcion, d.ID)
	}
}

func TestDefinicion_GanadaYProgreso(t *testing.T) {
	d := Definicion{ID: "explorador", Fuente: FuenteLugares, Meta: 5}
	assert.False(t, d.Ganada(map[string]int{}))
	assert.Equal(t, 0, d.Progreso(map[string]int{}))
	assert.False(t, d.Ganada(map[string]int{FuenteLugares: 4}))
	assert.Equal(t, 4, d.Progreso(map[string]int{FuenteLugares: 4}))
	assert.True(t, d.Ganada(map[string]int{FuenteLugares: 5}))
	assert.Equal(t, 5, d.Progreso(map[string]int{FuenteLugares: 9}), "el avance no pasa la meta")
}

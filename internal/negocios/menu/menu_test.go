package menu

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

func TestDTOs_Validador(t *testing.T) {
	v, err := validation.New()
	require.NoError(t, err)
	ok := CrearRequest{Seccion: "Tacos", Nombre: "Pastor", Precio: 25.5, Etiquetas: []string{"picante"}}
	assert.NoError(t, v.Struct(ok))

	malas := map[string]CrearRequest{
		"sin sección":       {Nombre: "Pastor"},
		"sin nombre":        {Seccion: "Tacos"},
		"precio negativo":   {Seccion: "Tacos", Nombre: "Pastor", Precio: -1},
		"precio alto":       {Seccion: "Tacos", Nombre: "Pastor", Precio: 100001},
		"etiqueta rara":     {Seccion: "Tacos", Nombre: "Pastor", Etiquetas: []string{"barato"}},
		"etiqueta repetida": {Seccion: "Tacos", Nombre: "Pastor", Etiquetas: []string{"picante", "picante"}},
		"sección larga":     {Seccion: "Una sección con más de treinta letras", Nombre: "Pastor"},
	}
	for nombre, r := range malas {
		assert.Error(t, v.Struct(r), nombre)
	}
	assert.Error(t, v.Struct(MenuQuery{Q: "a"}), "q de 2 a 60")
	assert.NoError(t, v.Struct(MenuQuery{Q: "ca"}))
	vacio := ""
	assert.Error(t, v.Struct(EditarRequest{Nombre: &vacio}))
}

func TestAgrupar_PorSeccionEnOrden(t *testing.T) {
	items := []Item{
		{ID: bson.NewObjectID(), Seccion: "Bebidas", Nombre: "Agua", Orden: 0},
		{ID: bson.NewObjectID(), Seccion: "Tacos", Nombre: "Pastor", Orden: 1, FotoURL: "https://img.test/a.jpg"},
		{ID: bson.NewObjectID(), Seccion: "Bebidas", Nombre: "Café", Orden: 2},
	}
	out := agrupar(items)
	require.Len(t, out.Secciones, 2)
	assert.Equal(t, "Bebidas", out.Secciones[0].Nombre)
	assert.Equal(t, []string{"Agua", "Café"}, []string{out.Secciones[0].Platillos[0].Nombre, out.Secciones[0].Platillos[1].Nombre})
	assert.Nil(t, out.Secciones[0].Platillos[0].FotoURL)
	assert.Equal(t, []string{}, out.Secciones[0].Platillos[0].Etiquetas)
	require.NotNil(t, out.Secciones[1].Platillos[0].FotoURL)
	assert.Equal(t, MenuResponse{Secciones: []Seccion{}}, agrupar(nil))
}

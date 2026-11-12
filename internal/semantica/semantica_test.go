package semantica

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/ia"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/recomendador"
)

func negocio(nombre, categoria string, calif float64, tags ...string) negocios.Business {
	return negocios.Business{ID: bson.NewObjectID(), Name: nombre, Category: categoria, Tags: tags,
		Rating: negocios.Calificacion{Promedio: calif, TotalResenas: 10}}
}

func nombres(bs []negocios.Business) []string {
	out := make([]string, len(bs))
	for i, b := range bs {
		out[i] = b.Name
	}
	return out
}

func TestPunto_EsElCosenoDeVectoresUnitarios(t *testing.T) {
	assert.InDelta(t, 1.0, Punto([]float32{0.6, 0.8}, []float32{0.6, 0.8}), 1e-6)
	assert.InDelta(t, 0.0, Punto([]float32{1, 0}, []float32{0, 1}), 1e-9)
	assert.InDelta(t, -1.0, Punto([]float32{1, 0}, []float32{-1, 0}), 1e-9)
}

func TestAfinidades_MinMax(t *testing.T) {
	a, b, c := bson.NewObjectID(), bson.NewObjectID(), bson.NewObjectID()
	out := Afinidades(map[bson.ObjectID]float64{a: 0.62, b: 0.52, c: 0.57})
	assert.InDelta(t, 1.0, out[a], 1e-9)
	assert.InDelta(t, 0.0, out[b], 1e-9)
	assert.InDelta(t, 0.5, out[c], 1e-9)
	assert.Equal(t, map[bson.ObjectID]float64{a: 0.5, b: 0.5}, Afinidades(map[bson.ObjectID]float64{a: 0.4, b: 0.4}), "todos iguales")
	assert.Nil(t, Afinidades(nil))
}

func TestPorAfinidad_OrdenEmpatesYSinVector(t *testing.T) {
	cafe, bar, museo, sinVector := negocio("cafe", "cafeteria", 4), negocio("bar", "bar", 4.8), negocio("museo", "entretenimiento", 3), negocio("sin", "bar", 5)
	afin := map[bson.ObjectID]float64{cafe.ID: 1, bar.ID: 0.3, museo.ID: 0.3}
	out := PorAfinidad([]negocios.Business{sinVector, museo, bar, cafe}, afin, 10)
	assert.Equal(t, []string{"cafe", "bar", "museo"}, nombres(out), "empate: gana la calificación; sin vector no sale")
	assert.Len(t, PorAfinidad([]negocios.Business{museo, bar, cafe}, afin, 2), 2)
}

func TestPalabras(t *testing.T) {
	assert.Equal(t, []string{"cafe", "tranquilo", "para", "leer"}, Palabras("Un café tranquilo, para leer... ¡café!"))
	assert.Empty(t, Palabras("a y o"))
}

func TestReglas_UsaLasPalabrasComoGustos(t *testing.T) {
	pesos, err := recomendador.PesosPorDefecto()
	require.NoError(t, err)
	lunes := time.Date(2026, 11, 9, 13, 0, 0, 0, time.UTC)
	bar := negocio("bar", "bar", 5, "mezcal")
	cafe := negocio("cafe", "cafeteria", 3, "postres")
	out := Reglas([]negocios.Business{bar, cafe}, "postres ricos", lunes, pesos, 5)
	assert.Equal(t, []string{"cafe", "bar"}, nombres(out), "la etiqueta pesa más que la calificación")
}

func TestHibrido_LaAfinidadMueveElOrden(t *testing.T) {
	pesos, err := recomendador.PesosPorDefecto()
	require.NoError(t, err)
	lunes := time.Date(2026, 11, 9, 13, 0, 0, 0, time.UTC)
	a, b := negocio("a", "bar", 4), negocio("b", "bar", 4.2)
	bs := []negocios.Business{a, b}
	assert.Equal(t, []string{"b", "a"}, nombres(Hibrido(bs, "algo", nil, lunes, pesos, 5)), "sin afinidad decide la calificación")
	afin := map[bson.ObjectID]float64{a.ID: 1, b.ID: 0}
	assert.Equal(t, []string{"a", "b"}, nombres(Hibrido(bs, "algo", afin, lunes, pesos, 5)))
}

func TestHuella_SinFraseNiUsuarioYEstable(t *testing.T) {
	h := Huella("gemini/gemini-embedding-2", "Café tranquilo")
	assert.Equal(t, h, Huella("gemini/gemini-embedding-2", "  cafe TRANQUILO "), "normaliza mayúsculas, acentos y espacios")
	assert.NotEqual(t, h, Huella("otro", "Café tranquilo"), "el modelo cambia la huella")
	assert.NotContains(t, h, "tranquilo")
	assert.Len(t, h, len("emb_")+64)
	assert.Equal(t, Huella("m", "llámame al 5512345678"), Huella("m", "llámame al 5587654321"), "se anonimiza antes")
}

func TestCodificar_IdaYVuelta(t *testing.T) {
	v := make([]float32, ia.Dimensiones)
	for i := range v {
		v[i] = float32(i) / 1000
	}
	got, ok := decodificar(codificar(v))
	require.True(t, ok)
	assert.Equal(t, v, got)
	_, ok = decodificar([]byte{1, 2, 3})
	assert.False(t, ok, "tamaño distinto: se ignora la caché")
}

func TestCausa_SinTextoDelProveedor(t *testing.T) {
	assert.Equal(t, "limite", causa(fmt.Errorf("x: %w", ia.ErrLimiteGlobal)))
	assert.Equal(t, "apagada", causa(errApagada))
	assert.Equal(t, "sin_vectores", causa(errSinVectores))
	assert.Equal(t, "falla", causa(errors.New("500 con texto del proveedor")))
}

func TestLimiteDe(t *testing.T) {
	assert.Equal(t, LimiteDefault, limiteDe(0))
	assert.Equal(t, LimiteMax, limiteDe(100))
	assert.Equal(t, 3, limiteDe(3))
}

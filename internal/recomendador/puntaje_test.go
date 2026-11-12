package recomendador

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
)

func pesosTest(t *testing.T) Pesos {
	t.Helper()
	p, err := PesosPorDefecto()
	require.NoError(t, err)
	return p
}

func TestPesos_VersionadosSumanUno(t *testing.T) {
	p := pesosTest(t)
	assert.InDelta(t, 1.0, p[FactorPresupuesto]+p[FactorDistancia]+p[FactorHorario]+p[FactorClima]+p[FactorGustos]+p[FactorCalificacion]+p[FactorAfinidad], 1e-9)
}

func TestPesos_Invalidos(t *testing.T) {
	cases := map[string]string{
		"no suman 1": `{"presupuesto":0.2,"distancia":0.2,"horario":0.2,"clima":0.2,"gustos":0.2,"calificacion":0.2}`,
		"falta uno":  `{"presupuesto":0.2,"distancia":0.2,"horario":0.2,"clima":0.2,"gustos":0.2,"otro":0}`,
		"de menos":   `{"presupuesto":1}`,
		"negativo":   `{"presupuesto":-0.2,"distancia":0.4,"horario":0.2,"clima":0.2,"gustos":0.2,"calificacion":0.2}`,
		"no es json": `[`,
	}
	for name, raw := range cases {
		_, err := ParsePesos([]byte(raw))
		assert.Error(t, err, name)
	}
}

func TestRecomendar_PesosInvalidosNoArranca(t *testing.T) {
	_, err := Recomendar(t.Context(), nil, Pesos{FactorClima: 1}, ctxBase(), 5)
	assert.ErrorIs(t, err, ErrPesos)
}

func TestPuntaje_Presupuesto(t *testing.T) {
	c := ctxBase()
	c.PresupuestoMin, c.PresupuestoMax = 100, 200
	p := NewPuntaje(c)
	assert.Equal(t, 1.0, p.Presupuesto(150))
	assert.Equal(t, 1.0, p.Presupuesto(80), "debajo del rango no castiga")
	assert.InDelta(t, 0.5, p.Presupuesto(300), 1e-9)
	assert.Equal(t, 0.0, p.Presupuesto(400), "al doble llega a 0")
	assert.Equal(t, 0.0, p.Presupuesto(900))
	assert.Equal(t, neutro, p.Presupuesto(0), "sin precio")
	assert.Equal(t, neutro, NewPuntaje(ctxBase()).Presupuesto(150), "sin presupuesto")
}

func TestPuntaje_Distancia(t *testing.T) {
	p := NewPuntaje(ctxBase())
	assert.Equal(t, 1.0, p.Distancia(0))
	assert.InDelta(t, 0.5, p.Distancia(2500), 1e-9, "radio por defecto 5 km")
	assert.Equal(t, 0.0, p.Distancia(6000))
	c := ctxBase()
	c.RadioM = 50000
	assert.InDelta(t, 0.5, NewPuntaje(c).Distancia(10000), 1e-9, "radio recortado a 20 km")
}

func TestPuntaje_HorarioMargenDeCierre(t *testing.T) {
	p := NewPuntaje(ctxBase())
	assert.Equal(t, 1.0, p.Horario(todoElDia()))
	assert.Equal(t, 0.0, p.Horario([]negocios.VentanaMin{{Dia: 1, Desde: 9 * 60, Hasta: 12*60 + 20}}), "cierra en 20 min")
}

func TestPuntaje_ClimaNeutroHastaNV25(t *testing.T) {
	assert.Equal(t, neutro, NewPuntaje(ctxBase()).Clima())
}

func TestPuntaje_Gustos(t *testing.T) {
	b := negocioBase()
	b.Category = "cafeteria"
	b.Tags = []string{"Postres", "terraza"}
	c := ctxBase()
	c.Gustos = []string{"Cafeterías", "postres", "sushi"}
	assert.Equal(t, 1.0, NewPuntaje(c).Gustos(b))
	c.Gustos = []string{"sushi", "terraza"}
	assert.Equal(t, 0.5, NewPuntaje(c).Gustos(b))
	c.Gustos = []string{"sushi"}
	assert.Equal(t, 0.0, NewPuntaje(c).Gustos(b))
	assert.Equal(t, neutro, NewPuntaje(ctxBase()).Gustos(b))
}

func TestPuntaje_AfinidadNeutraSinSemantica(t *testing.T) {
	b := negocioBase()
	assert.Equal(t, neutro, NewPuntaje(ctxBase()).Afinidad(b.ID), "sin búsqueda semántica")
	c := ctxBase()
	c.Afinidad = map[bson.ObjectID]float64{b.ID: 0.9}
	assert.Equal(t, 0.9, NewPuntaje(c).Afinidad(b.ID))
	assert.Equal(t, neutro, NewPuntaje(c).Afinidad(bson.NewObjectID()), "negocio sin vector")
}

// Sin semántica el factor vale 0,5 para todos y los demás pesos son los de nv-23 por 0,8:
// el orden es el mismo de antes.
func TestPesos_SinSemanticaConservaElOrden(t *testing.T) {
	p := pesosTest(t)
	viejos := map[string]float64{FactorPresupuesto: 0.20, FactorDistancia: 0.20, FactorHorario: 0.10, FactorClima: 0.10, FactorGustos: 0.25, FactorCalificacion: 0.15}
	for f, w := range viejos {
		assert.InDelta(t, w*(1-p[FactorAfinidad]), p[f], 1e-9, f)
	}
}

func TestPuntaje_Calificacion(t *testing.T) {
	p := NewPuntaje(ctxBase())
	assert.InDelta(t, 0.9, p.Calificacion(negocios.Calificacion{Promedio: 4.5, TotalResenas: 10}), 1e-9)
	assert.Equal(t, neutro, p.Calificacion(negocios.Calificacion{}))
}

func TestPuntaje_TotalFijo(t *testing.T) {
	pesos := pesosTest(t)
	f := map[string]float64{
		FactorPresupuesto: 1, FactorDistancia: 0.5, FactorHorario: 1,
		FactorClima: 0.5, FactorGustos: 1, FactorCalificacion: 0.9, FactorAfinidad: 0.5,
	}
	// 0.16 + 0.08 + 0.08 + 0.04 + 0.20 + 0.108 + 0.10 = 0.768
	assert.Equal(t, 76.8, Total(f, pesos))
	all := map[string]float64{}
	for _, k := range Factores {
		all[k] = 1
	}
	assert.Equal(t, 100.0, Total(all, pesos))
	assert.Equal(t, 0.0, Total(map[string]float64{}, pesos))
}

func TestOrdenar_EmpatesYFiltroEnGo(t *testing.T) {
	pesos := pesosTest(t)
	c := ctxBase()
	c.Alergenos = []string{"gluten"}
	a, b, conGluten := negocioBase(), negocioBase(), negocioBase()
	a.Rating = negocios.Calificacion{Promedio: 4, TotalResenas: 0}
	b.Rating = negocios.Calificacion{Promedio: 3, TotalResenas: 0}
	conGluten.Allergens = []string{"gluten"}
	cands := []negocios.Candidate{{Business: b}, {Business: conGluten}, {Business: a}}
	out := Ordenar(cands, pesos, c, 10)
	require.Len(t, out, 2, "el filtro en Go saca el de gluten aunque la consulta lo dejara pasar")
	assert.Equal(t, out[0].Puntaje, out[1].Puntaje)
	assert.Equal(t, a.ID, out[0].Negocio.ID, "empate: gana la calificación")

	b.Rating = a.Rating
	first, second := a, b
	if b.ID.Hex() < a.ID.Hex() {
		first, second = b, a
	}
	out = Ordenar([]negocios.Candidate{{Business: second}, {Business: first}}, pesos, c, 1)
	require.Len(t, out, 1)
	assert.Equal(t, first.ID, out[0].Negocio.ID, "empate total: gana el id menor")
	assert.Len(t, Ordenar(cands, pesos, c, 0), 2, "n inválido usa el tope")
}

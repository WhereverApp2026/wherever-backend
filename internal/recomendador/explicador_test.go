package recomendador

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExplicador_EligeLosQueMasAportan(t *testing.T) {
	e := NewExplicador(pesosTest(t))
	f := map[string]float64{
		FactorPresupuesto: 1, FactorDistancia: 0.7, FactorHorario: 1,
		FactorClima: 0.5, FactorGustos: 1, FactorCalificacion: 0.9,
	}
	// aportes: gustos .25, presupuesto .20, distancia .14, calificación .135, horario .10
	assert.Equal(t, []string{FactorGustos, FactorPresupuesto, FactorDistancia}, e.Razones(f))
}

func TestExplicador_ClimaNeutroYBajosNoSonRazon(t *testing.T) {
	e := NewExplicador(pesosTest(t))
	f := map[string]float64{FactorClima: 0.5, FactorDistancia: 0.59, FactorHorario: 1}
	assert.Equal(t, []string{FactorHorario}, e.Razones(f))
}

func TestExplicador_PorQueUnaDosTres(t *testing.T) {
	e := NewExplicador(pesosTest(t))
	assert.Equal(t, "Te lo recomendamos porque está cerca de ti.", e.PorQue([]string{FactorDistancia}))
	assert.Equal(t, "Te lo recomendamos porque coincide con tus gustos y está cerca de ti.",
		e.PorQue([]string{FactorGustos, FactorDistancia}))
	assert.Equal(t, "Te lo recomendamos porque coincide con tus gustos, entra en tu presupuesto y tiene muy buena calificación.",
		e.PorQue([]string{FactorGustos, FactorPresupuesto, FactorCalificacion}))
}

func TestExplicador_SinRazones(t *testing.T) {
	e := NewExplicador(pesosTest(t))
	assert.Empty(t, e.Razones(map[string]float64{}))
	assert.Equal(t, porQueBase, e.PorQue(nil))
}

func TestExplicador_SinDatosSensibles(t *testing.T) {
	e := NewExplicador(pesosTest(t))
	todas := e.PorQue(Factores) + porQueBase
	for _, prohibido := range append(palabrasAlergeno(), "alerg", "dieta", "vegan", "lat", "lng", "km") {
		assert.NotContains(t, strings.ToLower(todas), prohibido)
	}
	for _, r := range "0123456789" {
		assert.NotContains(t, todas, string(r), "sin cifras: ni distancias ni coordenadas")
	}
}

// palabrasAlergeno son las claves de sinónimos: ninguna puede aparecer en `por_que`.
func palabrasAlergeno() []string {
	out := make([]string, 0, len(alergenoSinonimos))
	for k := range alergenoSinonimos {
		out = append(out, k)
	}
	return out
}

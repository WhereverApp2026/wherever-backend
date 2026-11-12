package recomendador

import (
	"sort"
	"strings"
)

// Un factor entra a las razones si vale al menos esto; con menos no es un motivo.
const umbralRazon = 0.6

// maxRazones es cuántos factores explica `por_que` (spec: dos o tres).
const maxRazones = 3

// frases dicen cada factor sin datos sensibles: ni alergias, ni dieta, ni coordenadas.
var frases = map[string]string{
	FactorPresupuesto:  "entra en tu presupuesto",
	FactorDistancia:    "está cerca de ti",
	FactorHorario:      "sigue abierto un buen rato",
	FactorClima:        "va con el clima",
	FactorGustos:       "coincide con tus gustos",
	FactorCalificacion: "tiene muy buena calificación",
	FactorAfinidad:     "se parece a lo que buscas",
}

// porQueBase es la línea cuando ningún factor destaca: pasó los filtros y nada más.
const porQueBase = "Está abierto y cumple con lo que pediste."

// Explicador elige los factores que más sumaron y arma una línea en español (US-037).
type Explicador struct{ pesos Pesos }

// NewExplicador arma el explicador con los pesos del motor.
func NewExplicador(p Pesos) Explicador { return Explicador{pesos: p} }

// Razones son hasta tres factores, del que más aportó (valor × peso) al que menos; los
// empates siguen el orden canónico. El clima neutro no es una razón.
func (e Explicador) Razones(factores map[string]float64) []string {
	var out []string
	for _, f := range Factores {
		if factores[f] >= umbralRazon && e.pesos[f] > 0 {
			out = append(out, f)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return factores[out[i]]*e.pesos[out[i]] > factores[out[j]]*e.pesos[out[j]]
	})
	if len(out) > maxRazones {
		out = out[:maxRazones]
	}
	return out
}

// PorQue une las frases de las razones: «Te lo recomendamos porque a, b y c.»
func (e Explicador) PorQue(razones []string) string {
	if len(razones) == 0 {
		return porQueBase
	}
	parts := make([]string, 0, len(razones))
	for _, r := range razones {
		parts = append(parts, frases[r])
	}
	joined := parts[0]
	if n := len(parts); n > 1 {
		joined = strings.Join(parts[:n-1], ", ") + " y " + parts[n-1]
	}
	return "Te lo recomendamos porque " + joined + "."
}

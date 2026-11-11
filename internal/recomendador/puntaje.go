package recomendador

import (
	_ "embed" // pesos_recomendador.json viaja dentro del binario
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/text"
)

// Factores en el orden fijo en que se reportan.
const (
	FactorPresupuesto  = "presupuesto"
	FactorDistancia    = "distancia"
	FactorHorario      = "horario"
	FactorClima        = "clima"
	FactorGustos       = "gustos"
	FactorCalificacion = "calificacion"
)

// Factores es el orden canónico.
var Factores = []string{FactorPresupuesto, FactorDistancia, FactorHorario, FactorClima, FactorGustos, FactorCalificacion}

// neutro es el valor de un factor sin dato: ni suma ni resta frente a los demás.
const neutro = 0.5

// margenCierre es lo que debe seguir abierto para que el horario sume (spec nv-23).
const margenCierre = 30 * time.Minute

// Pesos es el peso de cada factor; suman 1.
type Pesos map[string]float64

//go:embed pesos_recomendador.json
var pesosJSON []byte

// PesosPorDefecto lee y valida los pesos versionados junto al paquete.
func PesosPorDefecto() (Pesos, error) { return ParsePesos(pesosJSON) }

// ParsePesos lee un JSON de pesos y lo valida: los seis factores, ninguno extra, cada uno
// entre 0 y 1 y la suma igual a 1 (±0,001).
func ParsePesos(raw []byte) (Pesos, error) {
	var p Pesos
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("pesos del recomendador: %w", err)
	}
	return p, p.Validate()
}

// Validate revisa los pesos; el motor no se arma con pesos inválidos.
func (p Pesos) Validate() error {
	if len(p) != len(Factores) {
		return errors.New("pesos del recomendador: deben ser exactamente los seis factores")
	}
	sum := 0.0
	for _, f := range Factores {
		w, ok := p[f]
		if !ok {
			return fmt.Errorf("pesos del recomendador: falta %s", f)
		}
		if w < 0 || w > 1 {
			return fmt.Errorf("pesos del recomendador: %s fuera de [0, 1]", f)
		}
		sum += w
	}
	if math.Abs(sum-1) > 0.001 {
		return fmt.Errorf("pesos del recomendador: suman %.3f, deben sumar 1", sum)
	}
	return nil
}

// Puntaje calcula cada factor entre 0 y 1. Una función por factor.
type Puntaje struct{ c Contexto }

// NewPuntaje arma el puntaje de un contexto.
func NewPuntaje(c Contexto) Puntaje { return Puntaje{c: c} }

// Factores devuelve el valor de cada factor para un candidato.
func (p Puntaje) Factores(cand negocios.Candidate) map[string]float64 {
	return map[string]float64{
		FactorPresupuesto:  p.Presupuesto(cand.AveragePrice),
		FactorDistancia:    p.Distancia(cand.DistanceM),
		FactorHorario:      p.Horario(cand.ScheduleMin),
		FactorClima:        p.Clima(),
		FactorGustos:       p.Gustos(cand.Business),
		FactorCalificacion: p.Calificacion(cand.Rating),
	}
}

// Total es la suma ponderada en escala 0–100, redondeada a un decimal para que el orden
// no dependa de ruido de punto flotante.
func Total(factores map[string]float64, pesos Pesos) float64 {
	sum := 0.0
	for _, f := range Factores {
		sum += factores[f] * pesos[f]
	}
	return math.Round(sum*1000) / 10
}

// Presupuesto vale 1 dentro del rango o por debajo y cae en línea recta hasta 0 al doble
// del máximo. Sin presupuesto o sin precio, neutro.
func (p Puntaje) Presupuesto(precio int) float64 {
	limit := p.c.PresupuestoMax
	if limit <= 0 || precio <= 0 {
		return neutro
	}
	if precio <= limit {
		return 1
	}
	return clamp01(1 - float64(precio-limit)/float64(limit))
}

// Distancia decae en línea recta de 1 en el punto a 0 en el borde del radio.
func (p Puntaje) Distancia(metros float64) float64 {
	return clamp01(1 - metros/p.c.radio())
}

// Horario vale 1 si el negocio sigue abierto 30 minutos después; si cierra antes, 0.
func (p Puntaje) Horario(ventanas []negocios.VentanaMin) float64 {
	if negocios.OpenAt(ventanas, p.c.Momento.Add(margenCierre)) {
		return 1
	}
	return 0
}

// Clima es neutro para todos hasta que nv-25 traiga el clima; así no cambia el orden.
func (p Puntaje) Clima() float64 { return neutro }

// Gustos cuenta cuántos gustos del usuario coinciden con el giro, su nombre o una
// etiqueta; dos o más coincidencias valen 1. Sin gustos, neutro.
func (p Puntaje) Gustos(b negocios.Business) float64 {
	if len(p.c.Gustos) == 0 {
		return neutro
	}
	terms := make([]string, 0, len(b.Tags)+2)
	terms = append(terms, text.Normalize(b.Category), text.Normalize(negocios.CategoryName(b.Category)))
	for _, tag := range b.Tags {
		terms = append(terms, text.Normalize(tag))
	}
	hits := 0
	for _, g := range p.c.Gustos {
		if slices.Contains(terms, text.Normalize(g)) {
			hits++
		}
	}
	return clamp01(float64(hits) / 2)
}

// Calificacion es el promedio sobre 5; sin reseñas, neutro.
func (p Puntaje) Calificacion(r negocios.Calificacion) float64 {
	if r.TotalResenas <= 0 {
		return neutro
	}
	return clamp01(r.Promedio / 5)
}

func clamp01(v float64) float64 { return math.Max(0, math.Min(1, v)) }

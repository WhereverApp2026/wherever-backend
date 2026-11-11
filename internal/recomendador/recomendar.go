package recomendador

import (
	"context"
	"errors"
	"sort"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
)

// MaxCandidatos es cuántos negocios cercanos que pasan los filtros se puntúan.
const MaxCandidatos = 200

// MaxResultados es el tope de `n`.
const MaxResultados = 50

// Fuente da los candidatos ya filtrados; la implementa *negocios.Repository.
type Fuente interface {
	Candidates(ctx context.Context, q negocios.CandidateQuery) ([]negocios.Candidate, error)
}

// Recomendacion es un negocio con su puntaje y su explicación.
type Recomendacion struct {
	Negocio    negocios.Business
	DistanciaM float64
	Puntaje    float64            // 0–100, un decimal
	Factores   map[string]float64 // 0–1 por factor
	Razones    []string
	PorQue     string
}

// ErrPesos indica que el motor se pidió con pesos inválidos.
var ErrPesos = errors.New("pesos del recomendador inválidos")

// Recomendar trae los candidatos que pasan los filtros duros en MongoDB, los puntúa y
// devuelve los `n` mejores. Determinista: misma entrada, mismo resultado.
func Recomendar(ctx context.Context, src Fuente, pesos Pesos, c Contexto, n int) ([]Recomendacion, error) {
	if err := pesos.Validate(); err != nil {
		return nil, errors.Join(ErrPesos, err)
	}
	cands, err := src.Candidates(ctx, negocios.CandidateQuery{
		Lat: c.Lat, Lng: c.Lng, RadiusM: c.radio(),
		Conditions: NewFiltro(c).Condiciones(), Limit: MaxCandidatos,
	})
	if err != nil {
		return nil, err
	}
	return Ordenar(cands, pesos, c, n), nil
}

// Ordenar vuelve a aplicar el filtro en Go (una alergia nunca se cuela aunque la consulta
// cambie), puntúa y ordena por puntaje, luego calificación y luego id.
func Ordenar(cands []negocios.Candidate, pesos Pesos, c Contexto, n int) []Recomendacion {
	filtro, puntaje, explicador := NewFiltro(c), NewPuntaje(c), NewExplicador(pesos)
	out := make([]Recomendacion, 0, len(cands))
	for _, cand := range cands {
		if !filtro.Cumple(cand.Business) {
			continue
		}
		factores := puntaje.Factores(cand)
		razones := explicador.Razones(factores)
		out = append(out, Recomendacion{
			Negocio: cand.Business, DistanciaM: cand.DistanceM,
			Puntaje: Total(factores, pesos), Factores: factores,
			Razones: razones, PorQue: explicador.PorQue(razones),
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return antes(out[i], out[j]) })
	return out[:min(len(out), limite(n))]
}

func antes(a, b Recomendacion) bool {
	if a.Puntaje != b.Puntaje {
		return a.Puntaje > b.Puntaje
	}
	if ra, rb := a.Negocio.Rating.Promedio, b.Negocio.Rating.Promedio; ra != rb {
		return ra > rb
	}
	return a.Negocio.ID.Hex() < b.Negocio.ID.Hex()
}

func limite(n int) int {
	if n <= 0 || n > MaxResultados {
		return MaxResultados
	}
	return n
}

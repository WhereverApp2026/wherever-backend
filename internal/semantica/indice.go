// Package semantica busca lugares por significado (spec nv-51): vectoriza la frase con el
// modelo de embeddings de nv-50, la compara con los vectores de los negocios aprobados y,
// si la IA no está, no tiene cupo o falla, responde el motor de reglas de nv-23.
package semantica

import (
	"context"
	"math"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Indice da la similitud entre la consulta y cada negocio de `ids` que tenga vector. Hoy es
// ProductoPunto (plan B: el MongoDB local no tiene `$vectorSearch`); un índice de Atlas
// Vector Search entraría detrás de la misma interfaz.
type Indice interface {
	Similitudes(ctx context.Context, consulta []float32, ids []bson.ObjectID) (map[bson.ObjectID]float64, error)
}

// Vectores lee los vectores guardados de unos negocios; lo implementa *Repository.
type Vectores interface {
	Vectores(ctx context.Context, ids []bson.ObjectID) (map[bson.ObjectID][]float32, error)
}

// ProductoPunto calcula la similitud en la API. Los vectores de nv-50 y el de la consulta
// están normalizados, así que el producto punto es el coseno.
type ProductoPunto struct{ vectores Vectores }

// NewProductoPunto arma el índice sobre el repositorio de vectores.
func NewProductoPunto(v Vectores) ProductoPunto { return ProductoPunto{vectores: v} }

// Similitudes ignora los vectores de otra dimensión (un modelo distinto).
func (p ProductoPunto) Similitudes(ctx context.Context, consulta []float32, ids []bson.ObjectID) (map[bson.ObjectID]float64, error) {
	vs, err := p.vectores.Vectores(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[bson.ObjectID]float64, len(vs))
	for id, v := range vs {
		if len(v) == len(consulta) {
			out[id] = Punto(consulta, v)
		}
	}
	return out, nil
}

// Punto es el producto punto de dos vectores del mismo largo.
func Punto(a, b []float32) float64 {
	var s float64
	for i := range a {
		s += float64(a[i]) * float64(b[i])
	}
	return s
}

// Afinidades lleva las similitudes a 0–1 entre el menos y el más parecido del catálogo.
// Los cosenos de un modelo de embeddings caen en una franja angosta; sin este paso el
// factor casi no separa a los negocios. Si todos se parecen igual, todos valen 0,5.
func Afinidades(sims map[bson.ObjectID]float64) map[bson.ObjectID]float64 {
	if len(sims) == 0 {
		return nil
	}
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, s := range sims {
		lo, hi = math.Min(lo, s), math.Max(hi, s)
	}
	out := make(map[bson.ObjectID]float64, len(sims))
	for id, s := range sims {
		if hi-lo < 1e-9 {
			out[id] = 0.5
			continue
		}
		out[id] = (s - lo) / (hi - lo)
	}
	return out
}

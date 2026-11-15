package planes

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Sorpresa es lo único que ve la persona sorprendida de un plan de grupo (nv-69) hasta
// `revelar_en`: la hora de la primera parada y dónde verse.
type Sorpresa struct {
	RevelarEn time.Time      `json:"revelar_en"`
	Inicio    *time.Time     `json:"inicio,omitempty"`
	Punto     *PuntoSorpresa `json:"punto,omitempty"`
}

// PuntoSorpresa es el punto de encuentro del grupo o, si no hay, el lugar de la primera parada.
type PuntoSorpresa struct {
	Nombre    string     `json:"nombre,omitempty"`
	Lat       *float64   `json:"lat,omitempty"`
	Lng       *float64   `json:"lng,omitempty"`
	Hora      *time.Time `json:"hora,omitempty"`
	NegocioID string     `json:"negocio_id,omitempty"`
}

// Surprises dice si el grupo del plan le oculta las paradas a userID en este momento; nil si
// no. Lo cumple el repositorio de grupos.
type Surprises interface {
	SurpriseFor(ctx context.Context, planID, userID bson.ObjectID, now time.Time) (*Sorpresa, error)
}

type noSurprises struct{}

func (noSurprises) SurpriseFor(context.Context, bson.ObjectID, bson.ObjectID, time.Time) (*Sorpresa, error) {
	return nil, nil
}

// WithSurprises recorta el plan a la persona sorprendida (nv-69).
func (s *Service) WithSurprises(sp Surprises) *Service {
	s.surprises = sp
	return s
}

// cut deja del plan solo la fecha, el inicio y el punto: sin paradas, traslados ni costos.
// Se decide al leer; no hay temporizador.
func cut(p *Plan, sp *Sorpresa) {
	if len(p.Paradas) > 0 {
		first := p.Paradas[0]
		for _, st := range p.Paradas[1:] {
			if st.Llega.Before(first.Llega) {
				first = st
			}
		}
		at := first.Llega
		sp.Inicio = &at
		if sp.Punto == nil {
			sp.Punto = &PuntoSorpresa{NegocioID: first.NegocioID.Hex()}
		}
	}
	p.Paradas, p.Traslados = []Parada{}, []Traslado{}
	p.CostoTotal, p.CostoPorPersona, p.Presupuesto = 0, 0, 0
	p.Sorpresa = sp
}

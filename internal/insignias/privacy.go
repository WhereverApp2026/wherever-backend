package insignias

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Privacy es la parte de nv-78 en ARCO (nv-05): exporta las insignias ganadas y las borra con
// la cuenta. No son historial de uso: «Borrar mi historial» no las toca.
type Privacy struct {
	repo *Repository
}

// NewPrivacy recibe el repositorio por constructor.
func NewPrivacy(repo *Repository) *Privacy { return &Privacy{repo: repo} }

// Name es la llave en la exportación.
func (p *Privacy) Name() string { return Collection }

// PurgeHistory no borra nada.
func (p *Privacy) PurgeHistory(context.Context, bson.ObjectID) error { return nil }

// PurgeAccount borra las insignias de la persona.
func (p *Privacy) PurgeAccount(ctx context.Context, userID bson.ObjectID) error {
	return p.repo.BorrarDe(ctx, userID)
}

type insigniaExportada struct {
	Insignia string    `json:"insignia"`
	GanadaEn time.Time `json:"ganada_en"`
}

// Export devuelve las insignias ganadas con su fecha.
func (p *Privacy) Export(ctx context.Context, userID bson.ObjectID) (any, error) {
	fechas, err := p.repo.Ganadas(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]insigniaExportada, 0, len(fechas))
	for _, d := range Catalogo {
		if f, ok := fechas[d.ID]; ok {
			out = append(out, insigniaExportada{Insignia: d.ID, GanadaEn: f.UTC()})
		}
	}
	return out, nil
}

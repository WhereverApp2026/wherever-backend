package votaciones

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Privacy es la parte de votaciones en ARCO (nv-05): cumple privacidad.Purger y
// privacidad.Exporter sobre `votaciones`.
type Privacy struct {
	repo *Repository
}

// NewPrivacy recibe el repositorio por constructor.
func NewPrivacy(repo *Repository) *Privacy { return &Privacy{repo: repo} }

// Name identifica al módulo en el registro y en el JSON exportado.
func (p *Privacy) Name() string { return Collection }

// PurgeHistory no toca las votaciones: son decisiones del grupo, no historial de uso.
func (p *Privacy) PurgeHistory(context.Context, bson.ObjectID) error { return nil }

// PurgeAccount borra las votaciones que abrió el usuario y quita sus votos de las demás.
func (p *Privacy) PurgeAccount(ctx context.Context, userID bson.ObjectID) error {
	return p.repo.DeleteByUser(ctx, userID)
}

type exportedBallot struct {
	VoteID    string    `json:"votacion_id"`
	GroupID   string    `json:"grupo_id"`
	StopID    string    `json:"parada_id"`
	Choice    string    `json:"negocio_id"`
	Status    string    `json:"estado"`
	CreatedAt time.Time `json:"creada_en"`
}

// Export devuelve los votos del usuario; nada de los votos de los demás.
func (p *Privacy) Export(ctx context.Context, userID bson.ObjectID) (any, error) {
	rows, err := p.repo.ListVotedBy(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]exportedBallot, 0, len(rows))
	for _, v := range rows {
		out = append(out, exportedBallot{
			VoteID: v.ID.Hex(), GroupID: v.GroupID.Hex(), StopID: v.StopID.Hex(),
			Choice: v.Ballots[userID.Hex()].Hex(), Status: v.Status, CreatedAt: v.CreatedAt,
		})
	}
	return out, nil
}

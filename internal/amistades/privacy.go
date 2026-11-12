package amistades

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Privacy es la parte de amistades en ARCO (nv-05): cumple privacidad.Purger y
// privacidad.Exporter sobre `amistades`.
type Privacy struct {
	repo *Repository
}

// NewPrivacy recibe el repositorio por constructor.
func NewPrivacy(repo *Repository) *Privacy { return &Privacy{repo: repo} }

// Name identifica al módulo en el registro y en el JSON exportado.
func (p *Privacy) Name() string { return Collection }

// PurgeHistory no toca las amistades: no son historial de uso.
func (p *Privacy) PurgeHistory(context.Context, bson.ObjectID) error { return nil }

// PurgeAccount borra todas las relaciones del usuario, en cualquier sentido.
func (p *Privacy) PurgeAccount(ctx context.Context, userID bson.ObjectID) error {
	return p.repo.DeleteByUser(ctx, userID)
}

type exportedFriendship struct {
	PersonID  string    `json:"persona_id"`
	Status    string    `json:"estado"`
	Direction string    `json:"direccion"`
	CreatedAt time.Time `json:"creada_en"`
	UpdatedAt time.Time `json:"actualizada_en"`
}

// Export devuelve las relaciones del usuario. Un bloqueo que hizo la otra persona no se
// incluye: bloquear no avisa.
func (p *Privacy) Export(ctx context.Context, userID bson.ObjectID) (any, error) {
	rows, err := p.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]exportedFriendship, 0, len(rows))
	for _, f := range rows {
		if f.Status == StatusBlocked && f.From != userID {
			continue
		}
		direction := DirectionReceived
		if f.From == userID {
			direction = DirectionSent
		}
		out = append(out, exportedFriendship{
			PersonID: f.Other(userID).Hex(), Status: f.Status, Direction: direction,
			CreatedAt: f.CreatedAt, UpdatedAt: f.UpdatedAt,
		})
	}
	return out, nil
}

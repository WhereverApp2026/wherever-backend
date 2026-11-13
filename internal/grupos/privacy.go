package grupos

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Privacy es la parte de grupos en ARCO (nv-05): cumple privacidad.Purger y
// privacidad.Exporter sobre `grupos`.
type Privacy struct {
	repo *Repository
}

// NewPrivacy recibe el repositorio por constructor.
func NewPrivacy(repo *Repository) *Privacy { return &Privacy{repo: repo} }

// Name identifica al módulo en el registro y en el JSON exportado.
func (p *Privacy) Name() string { return Collection }

// PurgeHistory no toca los grupos: son contenido compartido, no historial de uso.
func (p *Privacy) PurgeHistory(context.Context, bson.ObjectID) error { return nil }

// PurgeAccount borra los grupos que organiza el usuario y lo saca de los demás.
func (p *Privacy) PurgeAccount(ctx context.Context, userID bson.ObjectID) error {
	return p.repo.DeleteByUser(ctx, userID)
}

type exportedGroup struct {
	ID        string    `json:"id"`
	PlanID    string    `json:"plan_id"`
	Role      string    `json:"rol"`
	Status    string    `json:"estado"`
	Since     time.Time `json:"desde"`
	CreatedAt time.Time `json:"creado_en"`
}

// Export devuelve los grupos del usuario con su rol y estado; nada de los demás integrantes.
func (p *Privacy) Export(ctx context.Context, userID bson.ObjectID) (any, error) {
	rows, err := p.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]exportedGroup, 0, len(rows))
	for _, g := range rows {
		m, _ := g.Member(userID)
		out = append(out, exportedGroup{
			ID: g.ID.Hex(), PlanID: g.PlanID.Hex(), Role: m.Role, Status: m.Status, Since: m.Since, CreatedAt: g.CreatedAt,
		})
	}
	return out, nil
}

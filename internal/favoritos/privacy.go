package favoritos

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Privacy es la parte de favoritos en ARCO (nv-05): cumple privacidad.Purger y
// privacidad.Exporter sobre `favoritos`.
type Privacy struct {
	repo *Repository
}

// NewPrivacy recibe el repositorio por constructor.
func NewPrivacy(repo *Repository) *Privacy { return &Privacy{repo: repo} }

// Name identifica al módulo en el registro y en el JSON exportado.
func (p *Privacy) Name() string { return Collection }

// PurgeHistory no toca los favoritos: son elecciones de la persona, no historial de uso.
func (p *Privacy) PurgeHistory(context.Context, bson.ObjectID) error { return nil }

// PurgeAccount borra los favoritos del usuario.
func (p *Privacy) PurgeAccount(ctx context.Context, userID bson.ObjectID) error {
	return p.repo.DeleteByUser(ctx, userID)
}

type exportedFavorite struct {
	Type      string    `json:"tipo"`
	TargetID  string    `json:"objetivo_id"`
	Name      string    `json:"nombre"`
	CreatedAt time.Time `json:"creado_en"`
}

// Export devuelve los favoritos del usuario: tipo, objetivo, nombre y fecha.
func (p *Privacy) Export(ctx context.Context, userID bson.ObjectID) (any, error) {
	rows, err := p.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]exportedFavorite, 0, len(rows))
	for _, f := range rows {
		out = append(out, exportedFavorite{Type: f.Type, TargetID: f.TargetID.Hex(), Name: f.Summary.Name, CreatedAt: f.CreatedAt})
	}
	return out, nil
}

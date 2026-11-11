package recomendaciones

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Privacy es la parte de recomendaciones en ARCO (nv-05): cumple privacidad.Purger y
// privacidad.Exporter sobre `descartes_usuario`.
type Privacy struct {
	repo *Repository
}

// NewPrivacy recibe el repositorio por constructor.
func NewPrivacy(repo *Repository) *Privacy { return &Privacy{repo: repo} }

// Name identifica al módulo en el registro y en el JSON exportado.
func (p *Privacy) Name() string { return Collection }

// PurgeHistory borra los descartes: salen del uso de la app.
func (p *Privacy) PurgeHistory(ctx context.Context, userID bson.ObjectID) error {
	return p.repo.DeleteByUser(ctx, userID)
}

// PurgeAccount borra los descartes del usuario.
func (p *Privacy) PurgeAccount(ctx context.Context, userID bson.ObjectID) error {
	return p.repo.DeleteByUser(ctx, userID)
}

type exportedDescarte struct {
	BusinessID string    `json:"negocio_id"`
	CreatedAt  time.Time `json:"creado_en"`
}

// Export devuelve los negocios marcados con «No me interesa».
func (p *Privacy) Export(ctx context.Context, userID bson.ObjectID) (any, error) {
	rows, err := p.repo.ListByUser(ctx, userID, 0)
	if err != nil {
		return nil, err
	}
	out := make([]exportedDescarte, 0, len(rows))
	for _, d := range rows {
		out = append(out, exportedDescarte{BusinessID: d.BusinessID.Hex(), CreatedAt: d.CreatedAt})
	}
	return out, nil
}

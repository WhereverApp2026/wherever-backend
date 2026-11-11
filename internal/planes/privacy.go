package planes

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Privacy es la parte de planes en los derechos ARCO (nv-05): cumple privacidad.Purger y
// privacidad.Exporter sobre `planes`.
type Privacy struct {
	repo *Repository
}

// NewPrivacy recibe el repositorio de planes.
func NewPrivacy(repo *Repository) *Privacy { return &Privacy{repo: repo} }

// Name identifica al módulo en el registro y en el JSON exportado.
func (p *Privacy) Name() string { return Collection }

// PurgeHistory no borra nada: un plan es contenido que la persona armó, no historial de uso.
func (p *Privacy) PurgeHistory(context.Context, bson.ObjectID) error { return nil }

// PurgeAccount borra todos los planes del usuario.
func (p *Privacy) PurgeAccount(ctx context.Context, userID bson.ObjectID) error {
	return p.repo.DeleteByOwner(ctx, userID)
}

// Export devuelve los planes del usuario con su forma JSON (sin `dueno_id`).
func (p *Privacy) Export(ctx context.Context, userID bson.ObjectID) (any, error) {
	return p.repo.ListByOwner(ctx, userID)
}

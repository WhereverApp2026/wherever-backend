package busquedas

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Privacy es la parte de búsquedas en ARCO (nv-05): cumple privacidad.Purger y
// privacidad.Exporter sobre `busquedas_recientes`.
type Privacy struct {
	repo *Repository
}

// NewPrivacy recibe el repositorio por constructor.
func NewPrivacy(repo *Repository) *Privacy { return &Privacy{repo: repo} }

// Name identifica al módulo en el registro y en el JSON exportado.
func (p *Privacy) Name() string { return Collection }

// PurgeHistory borra las búsquedas recientes: son historial de uso.
func (p *Privacy) PurgeHistory(ctx context.Context, userID bson.ObjectID) error {
	return p.repo.Borrar(ctx, userID)
}

// PurgeAccount borra las búsquedas recientes del usuario.
func (p *Privacy) PurgeAccount(ctx context.Context, userID bson.ObjectID) error {
	return p.repo.Borrar(ctx, userID)
}

// Export devuelve las búsquedas recientes, de la más nueva a la más vieja.
func (p *Privacy) Export(ctx context.Context, userID bson.ObjectID) (any, error) {
	rows, err := p.repo.Recientes(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]RecienteItem, 0, len(rows))
	for _, r := range rows {
		out = append(out, RecienteItem{Texto: r.Text, BuscadoEn: r.SearchedAt.UTC()})
	}
	return out, nil
}

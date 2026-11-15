package eventos

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Privacy es la parte de eventos en ARCO (nv-05): cumple privacidad.Purger y
// privacidad.Exporter sobre `eventos`.
type Privacy struct {
	repo *Repository
}

// NewPrivacy recibe el repositorio por constructor.
func NewPrivacy(repo *Repository) *Privacy { return &Privacy{repo: repo} }

// Name identifica al módulo en el registro y en el JSON exportado.
func (p *Privacy) Name() string { return Collection }

// PurgeHistory no hace nada: un evento no es historial de uso.
func (p *Privacy) PurgeHistory(context.Context, bson.ObjectID) error { return nil }

// PurgeAccount borra los eventos propios sin negocio y le quita `creado_por` a los del
// negocio, que siguen siendo del negocio.
func (p *Privacy) PurgeAccount(ctx context.Context, userID bson.ObjectID) error {
	return p.repo.PurgarCreador(ctx, userID)
}

type exportedEvento struct {
	ID        string    `json:"id"`
	Titulo    string    `json:"titulo"`
	Tipo      string    `json:"tipo"`
	Inicio    time.Time `json:"inicio"`
	Fin       time.Time `json:"fin"`
	Estado    string    `json:"estado"`
	NegocioID string    `json:"negocio_id,omitempty"`
}

// Export devuelve los eventos que creó el usuario.
func (p *Privacy) Export(ctx context.Context, userID bson.ObjectID) (any, error) {
	rows, err := p.repo.DelCreador(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]exportedEvento, 0, len(rows))
	for _, e := range rows {
		row := exportedEvento{ID: e.ID.Hex(), Titulo: e.Titulo, Tipo: e.Tipo, Inicio: e.Inicio, Fin: e.Fin, Estado: e.Estado}
		if e.NegocioID != nil {
			row.NegocioID = e.NegocioID.Hex()
		}
		out = append(out, row)
	}
	return out, nil
}

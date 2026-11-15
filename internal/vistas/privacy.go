package vistas

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Privacy es la parte de vistas en ARCO (nv-05): cumple privacidad.Purger y
// privacidad.Exporter sobre `historial_vistas`. Son historial: se borran con «borrar
// historial» y con la cuenta.
type Privacy struct {
	repo   *Repository
	places Places
}

// NewPrivacy recibe el repositorio y los lugares (para el nombre en la exportación).
func NewPrivacy(repo *Repository, places Places) *Privacy {
	return &Privacy{repo: repo, places: places}
}

// Name identifica al módulo en el registro y en el JSON exportado.
func (p *Privacy) Name() string { return Collection }

// PurgeHistory borra todas las vistas del usuario.
func (p *Privacy) PurgeHistory(ctx context.Context, userID bson.ObjectID) error {
	return p.repo.BorrarTodo(ctx, userID)
}

// PurgeAccount borra todas las vistas del usuario.
func (p *Privacy) PurgeAccount(ctx context.Context, userID bson.ObjectID) error {
	return p.repo.BorrarTodo(ctx, userID)
}

type exportedVista struct {
	NegocioID string    `json:"negocio_id"`
	Nombre    string    `json:"nombre,omitempty"`
	Origen    string    `json:"origen,omitempty"`
	Fecha     time.Time `json:"fecha"`
}

// Export devuelve las vistas de la más nueva a la más vieja, con el nombre del lugar si sigue
// visible.
func (p *Privacy) Export(ctx context.Context, userID bson.ObjectID) (any, error) {
	rows, err := p.repo.Recientes(ctx, userID, 0)
	if err != nil {
		return nil, err
	}
	ids := make([]bson.ObjectID, 0, len(rows))
	for _, v := range rows {
		ids = append(ids, v.NegocioID)
	}
	names := map[string]string{}
	if p.places != nil && len(ids) > 0 {
		cards, err := p.places.FindCards(ctx, ids, time.Now())
		if err != nil {
			return nil, err
		}
		for _, c := range cards {
			names[c.ID] = c.Name
		}
	}
	out := make([]exportedVista, 0, len(rows))
	for _, v := range rows {
		out = append(out, exportedVista{NegocioID: v.NegocioID.Hex(), Nombre: names[v.NegocioID.Hex()], Origen: v.Origen, Fecha: v.Fecha})
	}
	return out, nil
}

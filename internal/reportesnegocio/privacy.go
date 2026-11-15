package reportesnegocio

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Privacy es la parte de reportes en ARCO (nv-05): cumple privacidad.Purger y
// privacidad.Exporter sobre `reportes_negocio`.
type Privacy struct {
	repo     *Repository
	negocios Negocios
}

// NewPrivacy recibe el repositorio y los negocios (para el nombre en la exportación).
func NewPrivacy(repo *Repository, n Negocios) *Privacy { return &Privacy{repo: repo, negocios: n} }

// Name identifica al módulo en el registro y en el JSON exportado.
func (p *Privacy) Name() string { return Collection }

// PurgeHistory no hace nada: un reporte no es historial de uso.
func (p *Privacy) PurgeHistory(context.Context, bson.ObjectID) error { return nil }

// PurgeAccount borra todos los reportes de la persona, abiertos y resueltos.
func (p *Privacy) PurgeAccount(ctx context.Context, userID bson.ObjectID) error {
	return p.repo.BorrarDelUsuario(ctx, userID)
}

type exportedReporte struct {
	NegocioID  string     `json:"negocio_id"`
	Negocio    string     `json:"negocio,omitempty"`
	Motivo     string     `json:"motivo"`
	Detalle    string     `json:"detalle,omitempty"`
	Estado     string     `json:"estado"`
	CreadoEn   time.Time  `json:"creado_en"`
	ResueltoEn *time.Time `json:"resuelto_en,omitempty"`
}

// Export devuelve los reportes de la persona, del más nuevo al más viejo.
func (p *Privacy) Export(ctx context.Context, userID bson.ObjectID) (any, error) {
	rows, err := p.repo.DelUsuario(ctx, userID)
	if err != nil {
		return nil, err
	}
	ids := make([]bson.ObjectID, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.NegocioID)
	}
	names := map[string]string{}
	if p.negocios != nil && len(ids) > 0 {
		cards, err := p.negocios.FindCards(ctx, ids, time.Now())
		if err != nil {
			return nil, err
		}
		for _, c := range cards {
			names[c.ID] = c.Name
		}
	}
	out := make([]exportedReporte, 0, len(rows))
	for _, r := range rows {
		out = append(out, exportedReporte{NegocioID: r.NegocioID.Hex(), Negocio: names[r.NegocioID.Hex()], Motivo: r.Motivo,
			Detalle: r.Detalle, Estado: r.Estado, CreadoEn: r.CreadoEn, ResueltoEn: r.ResueltoEn})
	}
	return out, nil
}

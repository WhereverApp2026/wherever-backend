package cupones

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Privacy es la parte de la cartera de cupones en ARCO (nv-05). Los cupones del negocio no son
// datos de la persona: se van con la baja del negocio (Observador).
type Privacy struct {
	repo *Repository
}

// NewPrivacy recibe el repositorio por constructor.
func NewPrivacy(repo *Repository) *Privacy { return &Privacy{repo: repo} }

// Name identifica al módulo en el registro y en el JSON exportado.
func (p *Privacy) Name() string { return Collection }

// PurgeHistory borra los cupones ya canjeados o vencidos; conserva los que siguen vigentes.
func (p *Privacy) PurgeHistory(ctx context.Context, userID bson.ObjectID) error {
	return p.repo.BorrarCarteras(ctx, userID, CarteraCanjeado, CarteraVencido)
}

// PurgeAccount borra toda la cartera y quita a la persona de los canjes que validó.
func (p *Privacy) PurgeAccount(ctx context.Context, userID bson.ObjectID) error {
	if err := p.repo.BorrarCarteras(ctx, userID); err != nil {
		return err
	}
	return p.repo.OlvidarValidador(ctx, userID)
}

type exportado struct {
	CuponID       string     `json:"cupon_id"`
	NegocioID     string     `json:"negocio_id"`
	Titulo        string     `json:"titulo"`
	Tipo          string     `json:"tipo"`
	Valor         *int64     `json:"valor"`
	NegocioNombre string     `json:"negocio_nombre"`
	Estado        string     `json:"estado"`
	VenceEn       time.Time  `json:"vence_en"`
	CanjeadoEn    *time.Time `json:"canjeado_en,omitempty"`
	CreadoEn      time.Time  `json:"creado_en"`
}

// Export devuelve la cartera (resumen, estado y fechas); nunca el código de canje.
func (p *Privacy) Export(ctx context.Context, userID bson.ObjectID) (any, error) {
	cs, err := p.repo.CarterasDe(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]exportado, 0, len(cs))
	for _, c := range cs {
		out = append(out, exportado{
			CuponID: c.CuponID.Hex(), NegocioID: c.NegocioID.Hex(), Titulo: c.Resumen.Titulo, Tipo: c.Resumen.Tipo,
			Valor: valorPublico(c.Resumen.Tipo, c.Resumen.Valor), NegocioNombre: c.Resumen.NegocioNombre, Estado: c.Estado,
			VenceEn: c.VenceEn.UTC(), CanjeadoEn: c.CanjeadoEn, CreadoEn: c.CreadoEn.UTC(),
		})
	}
	return out, nil
}

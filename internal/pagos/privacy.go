package pagos

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
)

// ARCO: borrar la cuenta o el historial no toca `pagos` ni `liquidaciones`: se conservan 5 años
// (Código Fiscal de la Federación, como dice el aviso) y el TTL los borra después. `pagos` no
// guarda nombre, correo ni teléfono, solo ids.

// Privacy exporta los pagos de la persona.
type Privacy struct{ repo *Repository }

// NewPrivacy recibe el repositorio por constructor.
func NewPrivacy(repo *Repository) *Privacy { return &Privacy{repo: repo} }

// Name cumple privacidad.Exporter.
func (p *Privacy) Name() string { return Collection }

type pagoExportado struct {
	Concepto      string      `json:"concepto"`
	MontoCentavos int64       `json:"monto_centavos"`
	Moneda        string      `json:"moneda"`
	Estado        string      `json:"estado"`
	Fecha         time.Time   `json:"fecha"`
	Tarjeta       *Tarjeta    `json:"tarjeta,omitempty"`
	Reembolsos    []Reembolso `json:"reembolsos"`
}

// Export devuelve concepto, monto, fecha, estado, marca y últimos 4 de cada pago.
func (p *Privacy) Export(ctx context.Context, userID bson.ObjectID) (any, error) {
	ps, err := p.repo.DeUsuario(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]pagoExportado, 0, len(ps))
	for _, x := range ps {
		reembolsos := x.Reembolsos
		if reembolsos == nil {
			reembolsos = []Reembolso{}
		}
		out = append(out, pagoExportado{Concepto: x.Concepto, MontoCentavos: x.MontoCentavos, Moneda: x.Moneda, Estado: x.Estado,
			Fecha: x.CreadoEn.UTC(), Tarjeta: x.Tarjeta, Reembolsos: reembolsos})
	}
	return out, nil
}

// LiquidacionesPrivacy exporta las liquidaciones de los negocios del comerciante.
type LiquidacionesPrivacy struct{ repo *Repository }

// NewLiquidacionesPrivacy recibe el repositorio por constructor.
func NewLiquidacionesPrivacy(repo *Repository) *LiquidacionesPrivacy {
	return &LiquidacionesPrivacy{repo: repo}
}

// Name cumple privacidad.Exporter.
func (p *LiquidacionesPrivacy) Name() string { return LiquidacionesCollection }

type liquidacionExportada struct {
	NegocioID string `json:"negocio_id"`
	LiquidacionResponse
}

// Export devuelve las liquidaciones de sus negocios (vacío si no es comerciante).
func (p *LiquidacionesPrivacy) Export(ctx context.Context, userID bson.ObjectID) (any, error) {
	ids, err := p.repo.NegociosDe(ctx, userID)
	if err != nil || len(ids) == 0 {
		return []liquidacionExportada{}, err
	}
	ls, err := p.repo.LiquidacionesDeNegocios(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]liquidacionExportada, 0, len(ls))
	for _, l := range ls {
		out = append(out, liquidacionExportada{NegocioID: l.NegocioID.Hex(), LiquidacionResponse: liquidacionResponse(l, negocios.MexicoCity())})
	}
	return out, nil
}

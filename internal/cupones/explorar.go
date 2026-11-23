package cupones

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
)

// OffersFor son los cupones vigentes ahora de places (ix_cupones_negocio), con lo necesario
// para que Explorar elija el mejor de cada tarjeta (nv-85). Cumple negocios.Coupons.
func (r *Repository) OffersFor(ctx context.Context, places []bson.ObjectID, now time.Time) ([]negocios.CouponOffer, error) {
	out := []negocios.CouponOffer{}
	if len(places) == 0 {
		return out, nil
	}
	filter := append(bson.D{{Key: "negocio_id", Value: bson.D{{Key: "$in", Value: places}}}}, vigenteFilter(now)...)
	proj := options.Find().SetProjection(bson.D{
		{Key: "negocio_id", Value: 1}, {Key: "titulo", Value: 1}, {Key: "tipo", Value: 1},
		{Key: "valor", Value: 1}, {Key: "vigencia_fin", Value: 1},
	})
	rows, err := findAll[Cupon](ctx, r.col, filter, proj)
	if err != nil {
		return nil, err
	}
	for _, c := range rows {
		out = append(out, offerOf(c))
	}
	return out, nil
}

// offerOf traduce el tipo del cupón a lo que compara Explorar.
func offerOf(c Cupon) negocios.CouponOffer {
	o := negocios.CouponOffer{Place: c.NegocioID, ID: c.ID, Title: c.Titulo, Ends: c.VigenciaFin}
	switch c.Tipo {
	case TipoPorcentaje:
		o.Percent = c.Valor
	case TipoMonto:
		o.AmountCents = c.Valor
	case TipoDosPorUno:
		o.TwoForOne = true
	}
	return o
}

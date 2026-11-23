package cupones

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/notificaciones"
)

// Cupón por vencer (nv-86): un cupón guardado sin canjear que vence en los próximos 3 días
// avisa una vez. Solo de 10:00 a 21:59 de México, para no avisar de madrugada.
const (
	AvisoAntes         = 72 * time.Hour
	AvisoDesdeHora     = 10
	AvisoHastaHora     = 22
	MaxAvisosPorVencer = 500
)

// AvisoPorVencer es la tarea del barrido de avisos de la cartera.
type AvisoPorVencer struct {
	repo  *Repository
	unico notificaciones.Unico
}

// NewAvisoPorVencer recibe el repositorio y el aviso único por constructor.
func NewAvisoPorVencer(repo *Repository, unico notificaciones.Unico) *AvisoPorVencer {
	return &AvisoPorVencer{repo: repo, unico: unico}
}

// Name identifica la tarea en el log.
func (a *AvisoPorVencer) Name() string { return notificaciones.KindCouponExpiring }

// EnHorarioDeAviso dice si `now` cae de 10:00 a 21:59 en México.
func EnHorarioDeAviso(now time.Time) bool {
	h := now.In(negocios.MexicoCity()).Hour()
	return h >= AvisoDesdeHora && h < AvisoHastaHora
}

// Avisar busca los cupones guardados que vencen en (now, now+72 h].
func (a *AvisoPorVencer) Avisar(ctx context.Context, now time.Time) (int, error) {
	if !EnHorarioDeAviso(now) {
		return 0, nil
	}
	cs, err := a.repo.GuardadosPorVencer(ctx, now, now.Add(AvisoAntes), MaxAvisosPorVencer)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, c := range cs {
		data := map[string]string{"cupon_id": c.CuponID.Hex(), "negocio_id": c.NegocioID.Hex()}
		if a.unico.Once(ctx, c.UsuarioID, notificaciones.KindCouponExpiring, "cupon:"+c.CuponID.Hex(), data) {
			n++
		}
	}
	return n, nil
}

// GuardadosPorVencer son las entradas `guardado` de la cartera con `vence_en` en (desde, hasta]
// (ix_cartera_vence).
func (r *Repository) GuardadosPorVencer(ctx context.Context, desde, hasta time.Time, limit int) ([]Cartera, error) {
	filter := bson.D{
		{Key: "estado", Value: CarteraGuardado},
		{Key: "vence_en", Value: bson.D{{Key: "$gt", Value: desde}, {Key: "$lte", Value: hasta}}},
	}
	return findAll[Cartera](ctx, r.cartera, filter, options.Find().SetSort(bson.D{{Key: "vence_en", Value: 1}}).SetLimit(int64(limit)))
}

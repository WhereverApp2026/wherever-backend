package resenas

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/notificaciones"
)

// Reseña pendiente (nv-86): un check-in válido sin reseña avisa «Cuéntanos cómo te fue» a las
// 2 h, una vez por check-in. Pasado un día ya no avisa (la persona aún puede reseñar desde
// «Por reseñar» los DiasPorResenar días).
const (
	PendienteTras     = 2 * time.Hour
	PendienteHasta    = 24 * time.Hour
	MaxAvisosPorCorte = 500
)

// AvisoPendiente es la tarea del barrido de avisos que pide reseña de cada visita.
type AvisoPendiente struct {
	repo  *Repository
	unico notificaciones.Unico
}

// NewAvisoPendiente recibe el repositorio y el aviso único por constructor.
func NewAvisoPendiente(repo *Repository, unico notificaciones.Unico) *AvisoPendiente {
	return &AvisoPendiente{repo: repo, unico: unico}
}

// Name identifica la tarea en el log.
func (a *AvisoPendiente) Name() string { return notificaciones.KindReviewPending }

// Avisar busca los check-ins válidos sin reseña creados hace entre 2 y 24 h.
func (a *AvisoPendiente) Avisar(ctx context.Context, now time.Time) (int, error) {
	cs, err := a.repo.SinResenaEntre(ctx, now.Add(-PendienteHasta), now.Add(-PendienteTras), MaxAvisosPorCorte)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, c := range cs {
		data := map[string]string{"checkin_id": c.ID.Hex(), "negocio_id": c.NegocioID.Hex()}
		if a.unico.Once(ctx, c.UsuarioID, notificaciones.KindReviewPending, "checkin:"+c.ID.Hex(), data) {
			n++
		}
	}
	return n, nil
}

// SinResenaEntre son los check-ins válidos sin reseña creados en [desde, hasta)
// (ix_checkins_pendientes).
func (r *Repository) SinResenaEntre(ctx context.Context, desde, hasta time.Time, limit int) ([]Checkin, error) {
	filter := bson.D{
		{Key: "valido", Value: true},
		{Key: "creado_en", Value: bson.D{{Key: "$gte", Value: desde}, {Key: "$lt", Value: hasta}}},
		{Key: "resena_id", Value: bson.D{{Key: "$exists", Value: false}}},
	}
	return listar[Checkin](ctx, r.checkins, filter, bson.D{{Key: "creado_en", Value: 1}}, limit)
}

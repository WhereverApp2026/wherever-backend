package planes

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/notificaciones"
)

// Hora de salir (nv-86): con un plan de hoy, avisa a quien lo hizo cuando falta el tiempo de
// traslado más 5 minutos para llegar a la siguiente parada. Una vez por tramo.
const (
	SalidaMargen    = 5 * time.Minute
	MaxPlanesDelDia = 1000
)

// Salida es un tramo que ya toca avisar: la parada a la que hay que ir y cuándo salir.
type Salida struct {
	Parada  Parada
	SaleEn  time.Time
	PlanID  bson.ObjectID
	DuenoID bson.ObjectID
}

// SalidasPorAvisar son los tramos del plan cuya hora de salir (llegada − traslado − 5 min) ya
// pasó y cuya llegada aún no: entre esos dos momentos el aviso sirve.
func SalidasPorAvisar(p Plan, now time.Time) []Salida {
	byID := make(map[bson.ObjectID]Parada, len(p.Paradas))
	for _, s := range p.Paradas {
		byID[s.ID] = s
	}
	out := []Salida{}
	for _, t := range p.Traslados {
		dest, ok := byID[t.Hasta]
		if !ok || t.Minutos <= 0 {
			continue
		}
		sale := dest.Llega.Add(-time.Duration(t.Minutos)*time.Minute - SalidaMargen)
		if now.Before(sale) || !now.Before(dest.Llega) {
			continue
		}
		out = append(out, Salida{Parada: dest, SaleEn: sale, PlanID: p.ID, DuenoID: p.OwnerID})
	}
	return out
}

// AvisoSalida es la tarea del barrido de avisos que mira solo los planes de hoy.
type AvisoSalida struct {
	repo  *Repository
	unico notificaciones.Unico
}

// NewAvisoSalida recibe el repositorio y el aviso único por constructor.
func NewAvisoSalida(repo *Repository, unico notificaciones.Unico) *AvisoSalida {
	return &AvisoSalida{repo: repo, unico: unico}
}

// Name identifica la tarea en el log.
func (a *AvisoSalida) Name() string { return notificaciones.KindTimeToLeave }

// Avisar revisa los planes activos con fecha de hoy en México. Las preferencias y el horario
// silencioso los decide el centro de avisos al guardar.
func (a *AvisoSalida) Avisar(ctx context.Context, now time.Time) (int, error) {
	local := now.In(negocios.MexicoCity())
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, local.Location())
	ps, err := a.repo.DelDia(ctx, today, MaxPlanesDelDia)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, p := range ps {
		for _, s := range SalidasPorAvisar(p, now) {
			data := map[string]string{"plan_id": s.PlanID.Hex(), "parada_id": s.Parada.ID.Hex(), "negocio_id": s.Parada.NegocioID.Hex()}
			if a.unico.Once(ctx, s.DuenoID, notificaciones.KindTimeToLeave, "plan:"+s.PlanID.Hex()+":"+s.Parada.ID.Hex(), data) {
				n++
			}
		}
	}
	return n, nil
}

// DelDia trae los planes activos de ese día con al menos un traslado (ix_estado_fecha); solo
// lee paradas y traslados.
func (r *Repository) DelDia(ctx context.Context, day time.Time, limit int) ([]Plan, error) {
	filter := bson.D{
		{Key: "estado", Value: StatusActive},
		{Key: "fecha", Value: day},
		{Key: "traslados.0", Value: bson.D{{Key: "$exists", Value: true}}},
	}
	opts := options.Find().SetLimit(int64(limit)).
		SetProjection(bson.D{{Key: "dueno_id", Value: 1}, {Key: "paradas", Value: 1}, {Key: "traslados", Value: 1}})
	cur, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	out := []Plan{}
	return out, cur.All(ctx, &out)
}

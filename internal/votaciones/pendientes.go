package votaciones

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/grupos"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
)

// MaxGruposPendientes son los grupos más recientes de la persona que revisa «Falta tu voto»
// (nv-84): de sobra con el tope de 12 integrantes y planes de pocos días.
const MaxGruposPendientes = 100

// MyGroups son los grupos vivos donde está la persona, del más reciente al más viejo (lo
// cumple grupos.Repository).
type MyGroups interface {
	ListForMember(ctx context.Context, userID bson.ObjectID, after *grupos.Position, limit int) ([]grupos.Group, error)
}

// WithPendientes conecta GET /votaciones/pendientes; sin él la ruta responde 503.
func (s *Service) WithPendientes(g MyGroups) *Service {
	s.myGroups = g
	return s
}

// GrupoRef es el grupo de una votación pendiente: su nombre o, sin él, el del plan.
type GrupoRef struct {
	ID     string `json:"id"`
	Nombre string `json:"nombre"`
}

// PendingView es una votación abierta donde falta el voto de quien llama.
type PendingView struct {
	ID       string    `json:"id"`
	Grupo    GrupoRef  `json:"grupo"`
	CierraEn time.Time `json:"cierra_en"`
}

// PendingResponse es GET /votaciones/pendientes.
type PendingResponse struct {
	Items []PendingView `json:"items"`
}

// PuedeVotarEn dice si la persona ve y puede votar las votaciones del grupo ahora: invitada o
// confirmada (no declinó) y sin una sorpresa que le oculte las paradas (nv-69).
func PuedeVotarEn(g grupos.Group, me bson.ObjectID, now time.Time) bool {
	m, in := g.Member(me)
	return in && m.Status != grupos.StatusDeclined && !g.Surprise.HidesFrom(me, now)
}

// Pendientes son las votaciones abiertas y sin vencer de los grupos de quien llama donde aún
// no votó, la que cierra antes primero. Un grupo con el plan borrado no cuenta.
func (s *Service) Pendientes(ctx context.Context, subject string) (PendingResponse, error) {
	out := PendingResponse{Items: []PendingView{}}
	if s.myGroups == nil {
		return out, errStore()
	}
	me, err := self(subject)
	if err != nil {
		return out, err
	}
	now := s.now().UTC()
	gs, err := s.myGroups.ListForMember(ctx, me, nil, MaxGruposPendientes)
	if err != nil {
		return out, errStore()
	}
	byID, ids := votables(gs, me, now)
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.repo.Pending(ctx, ids, me, now)
	if err != nil {
		return out, errStore()
	}
	if out.Items, err = s.pendingViews(ctx, rows, byID); err != nil {
		return PendingResponse{}, err
	}
	return out, nil
}

// votables son los grupos donde la persona puede votar ahora, por id.
func votables(gs []grupos.Group, me bson.ObjectID, now time.Time) (map[bson.ObjectID]grupos.Group, []bson.ObjectID) {
	byID := map[bson.ObjectID]grupos.Group{}
	ids := []bson.ObjectID{}
	for _, g := range gs {
		if PuedeVotarEn(g, me, now) {
			byID[g.ID] = g
			ids = append(ids, g.ID)
		}
	}
	return byID, ids
}

// pendingViews arma cada votación con el nombre de su grupo; las de un plan borrado no salen.
func (s *Service) pendingViews(ctx context.Context, rows []Vote, byID map[bson.ObjectID]grupos.Group) ([]PendingView, error) {
	out := []PendingView{}
	nombres := map[bson.ObjectID]string{}
	for _, v := range rows {
		nombre, ok := nombres[v.GroupID]
		if !ok {
			var err error
			if nombre, err = s.nombreGrupo(ctx, byID[v.GroupID]); err != nil {
				return nil, err
			}
			nombres[v.GroupID] = nombre
		}
		if nombre != "" {
			out = append(out, PendingView{ID: v.ID.Hex(), Grupo: GrupoRef{ID: v.GroupID.Hex(), Nombre: nombre}, CierraEn: v.ClosesAt.UTC()})
		}
	}
	return out, nil
}

// nombreGrupo es el nombre del grupo o el de su plan activo; "" si el plan ya no está.
func (s *Service) nombreGrupo(ctx context.Context, g grupos.Group) (string, error) {
	p, err := s.plans.FindActive(ctx, g.PlanID)
	if errors.Is(err, planes.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", errStore()
	}
	if g.Name != "" {
		return g.Name, nil
	}
	return p.Nombre, nil
}

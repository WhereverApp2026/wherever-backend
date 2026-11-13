package sincronizacion

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// Plans trae los planes del dueño cambiados después de un número.
type Plans interface {
	ChangedAfter(ctx context.Context, owner bson.ObjectID, after int64, limit int) ([]planes.Plan, error)
}

// Users lee el perfil (sus ajustes y su versión).
type Users interface {
	FindByID(ctx context.Context, id bson.ObjectID) (usuarios.User, error)
}

// Service arma la descarga de cambios.
type Service struct {
	repo   *Repository
	plans  Plans
	users  Users
	cursor *pagination.Signer
	now    func() time.Time
}

// NewService recibe sus dependencias por constructor; sin cursor la descarga falla cerrada.
func NewService(repo *Repository, plans Plans, users Users, cursor *pagination.Signer) *Service {
	return &Service{repo: repo, plans: plans, users: users, cursor: cursor, now: time.Now}
}

func errStore() error { return apperr.Unavailable("servicio no disponible") }

func invalidCursor() error {
	return apperr.Validation("cursor de sincronización inválido", apperr.FieldError{Field: "desde", Message: "cursor inválido; descarga todo de nuevo"})
}

// position es lo que guarda el cursor: el último número entregado, la versión del perfil que
// ya tiene la app y cuándo lo dio el servidor (para saber si los borrados ya vencieron). La
// hora es del servidor; la del teléfono nunca se usa.
type position struct {
	Seq     int64
	Profile int
	Issued  time.Time
}

func (s *Service) scope(me bson.ObjectID) string { return s.cursor.Scope("sync", me.Hex()) }

func (s *Service) decode(me bson.ObjectID, raw string) (position, error) {
	c, ok, err := s.cursor.Decode(s.scope(me), raw)
	if err != nil {
		return position{}, invalidCursor()
	}
	if !ok {
		return position{}, nil
	}
	parts := strings.Split(c.Sort, ":")
	if len(parts) != 3 {
		return position{}, invalidCursor()
	}
	seq, errSeq := strconv.ParseInt(parts[0], 10, 64)
	profile, errProfile := strconv.Atoi(parts[1])
	issued, errIssued := strconv.ParseInt(parts[2], 10, 64)
	if errSeq != nil || errProfile != nil || errIssued != nil || seq < 0 {
		return position{}, invalidCursor()
	}
	return position{Seq: seq, Profile: profile, Issued: time.Unix(issued, 0).UTC()}, nil
}

func (s *Service) encode(me bson.ObjectID, p position) (string, error) {
	return s.cursor.Encode(pagination.Cursor{
		ID: me.Hex(), Scope: s.scope(me), Sort: fmt.Sprintf("%d:%d:%d", p.Seq, p.Profile, p.Issued.Unix()),
	})
}

// Changes devuelve lo que cambió desde `desde`: planes nuevos o editados, borrados y los
// ajustes si cambiaron. Sin `desde` es la descarga completa. Un cursor de hace más de 30
// días es 410 con `requiere_completa`: sus borrados ya vencieron.
func (s *Service) Changes(ctx context.Context, subject string, q ChangesQuery) (ChangesResponse, error) {
	me, err := bson.ObjectIDFromHex(subject)
	if err != nil {
		return ChangesResponse{}, apperr.Unauthorized("autenticación requerida")
	}
	from, err := s.decode(me, q.From)
	if err != nil {
		return ChangesResponse{}, err
	}
	now := s.now().UTC()
	if q.From != "" && now.Sub(from.Issued) > Retention {
		return ChangesResponse{}, errFullRequired()
	}
	items, more, err := s.collect(ctx, me, from.Seq, q.Limit)
	if err != nil {
		return ChangesResponse{}, err
	}
	out, last := split(items, from.Seq)
	out.More = more
	u, err := s.users.FindByID(ctx, me)
	if err != nil {
		return ChangesResponse{}, errStore()
	}
	if q.From == "" || u.Version != from.Profile {
		settings := newSettings(u)
		out.Settings = &settings
	}
	out.Next, err = s.encode(me, position{Seq: last, Profile: u.Version, Issued: now})
	if err != nil {
		return ChangesResponse{}, errStore()
	}
	return out, nil
}

// errFullRequired es el 410 de esta ruta: la app descarga todo de nuevo (sin `desde`). El
// campo `desde` con mensaje `requiere_completa` lo distingue de cualquier otro error.
func errFullRequired() error {
	e := apperr.Gone("pasaron más de 30 días desde la última sincronización; descarga todo de nuevo")
	e.Fields = []apperr.FieldError{{Field: "desde", Message: "requiere_completa"}}
	return e
}

type change struct {
	seq  int64
	plan *planes.Plan
	tomb *Tombstone
}

// collect une planes y borrados en orden de número y corta en `limit`.
func (s *Service) collect(ctx context.Context, me bson.ObjectID, after int64, limit int) ([]change, bool, error) {
	plans, err := s.plans.ChangedAfter(ctx, me, after, limit+1)
	if err != nil {
		return nil, false, errStore()
	}
	tombs, err := s.repo.TombstonesAfter(ctx, me, after, limit+1)
	if err != nil {
		return nil, false, errStore()
	}
	out := make([]change, 0, len(plans)+len(tombs))
	i, j := 0, 0
	for i < len(plans) || j < len(tombs) {
		if j >= len(tombs) || (i < len(plans) && plans[i].SyncSeq < tombs[j].Seq) {
			out = append(out, change{seq: plans[i].SyncSeq, plan: &plans[i]})
			i++
			continue
		}
		out = append(out, change{seq: tombs[j].Seq, tomb: &tombs[j]})
		j++
	}
	if len(out) > limit {
		return out[:limit], true, nil
	}
	return out, false, nil
}

// split reparte los cambios en planes y borrados y da el último número entregado.
func split(items []change, from int64) (ChangesResponse, int64) {
	out := ChangesResponse{Plans: []planes.Plan{}, Deleted: []DeletedItem{}}
	last := from
	for _, it := range items {
		if it.plan != nil {
			out.Plans = append(out.Plans, *it.plan)
		} else {
			out.Deleted = append(out.Deleted, DeletedItem{Kind: it.tomb.Kind, ID: it.tomb.EntityID.Hex()})
		}
		last = it.seq
	}
	return out, last
}

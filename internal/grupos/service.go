package grupos

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// Plans es lo que grupos lee de planes: el plan propio y cuáles siguen activos.
type Plans interface {
	FindOwned(ctx context.Context, owner, id bson.ObjectID) (planes.Plan, error)
	ActiveIDs(ctx context.Context, ids []bson.ObjectID) (map[bson.ObjectID]bool, error)
}

// Friends dice si dos personas son amigas (nv-36).
type Friends interface {
	FindByPair(ctx context.Context, a, b bson.ObjectID) (amistades.Friendship, bool, error)
}

// Users trae lo público de las cuentas activas.
type Users interface {
	FindPublicByIDs(ctx context.Context, ids []bson.ObjectID) ([]usuarios.User, error)
}

// Service maneja los grupos (nv-37). La notificación `invitacion_plan` llega con nv-40.
type Service struct {
	repo    *Repository
	plans   Plans
	friends Friends
	users   Users
	cursor  *pagination.Signer
	now     func() time.Time
}

// NewService recibe sus dependencias por constructor; sin cursor la lista falla cerrada.
func NewService(repo *Repository, plans Plans, friends Friends, users Users, cursor *pagination.Signer) *Service {
	return &Service{repo: repo, plans: plans, friends: friends, users: users, cursor: cursor, now: time.Now}
}

// Errores públicos: se crean en cada llamada porque la etapa de errores puede anotarlos.
func errGroupNotFound() error { return apperr.NotFound("grupo no encontrado") }
func errPlanNotFound() error  { return apperr.NotFound("plan no encontrado") }
func errFull() error          { return apperr.Conflict("el grupo está lleno (máximo 12 personas)") }
func errNotOrganizer() error  { return apperr.Forbidden("solo el organizador puede hacer esto") }

// errStore no lleva el texto de MongoDB: podría repetir ids o nombres.
func errStore() error { return apperr.Unavailable("servicio no disponible") }

func self(subject string) (bson.ObjectID, error) {
	id, err := bson.ObjectIDFromHex(subject)
	if err != nil {
		return bson.ObjectID{}, apperr.Unauthorized("autenticación requerida")
	}
	return id, nil
}

// Create crea el grupo de un plan propio e invita a los amigos indicados (201).
func (s *Service) Create(ctx context.Context, subject string, req CreateRequest) (View, error) {
	me, err := self(subject)
	if err != nil {
		return View{}, err
	}
	planID, err := bson.ObjectIDFromHex(req.PlanID)
	if err != nil {
		return View{}, errPlanNotFound()
	}
	if err := s.ownPlan(ctx, me, planID); err != nil {
		return View{}, err
	}
	invitees, err := s.checkInvitees(ctx, me, req.Invited)
	if err != nil {
		return View{}, err
	}
	g, err := s.newGroup(ctx, me, planID)
	if errors.Is(err, ErrDuplicate) {
		return View{}, apperr.Conflict("el plan ya tiene grupo")
	}
	if err != nil {
		return View{}, errStore()
	}
	return s.invite(ctx, g, me, invitees)
}

// InviteToPlan invita a amigos al grupo del plan propio y crea el grupo si no existe.
func (s *Service) InviteToPlan(ctx context.Context, subject, rawPlanID string, req InviteRequest) (View, error) {
	me, err := self(subject)
	if err != nil {
		return View{}, err
	}
	planID, err := bson.ObjectIDFromHex(rawPlanID)
	if err != nil {
		return View{}, errPlanNotFound()
	}
	if err := s.ownPlan(ctx, me, planID); err != nil {
		return View{}, err
	}
	invitees, err := s.checkInvitees(ctx, me, req.Users)
	if err != nil {
		return View{}, err
	}
	g, err := s.repo.FindByPlan(ctx, planID)
	if errors.Is(err, ErrNotFound) {
		g, err = s.newGroup(ctx, me, planID)
		if errors.Is(err, ErrDuplicate) { // otra petición lo creó primero
			g, err = s.repo.FindByPlan(ctx, planID)
		}
	}
	if err != nil {
		return View{}, errStore()
	}
	return s.invite(ctx, g, me, invitees)
}

// Invite invita a amigos a un grupo; solo el organizador.
func (s *Service) Invite(ctx context.Context, subject, rawID string, req InviteRequest) (View, error) {
	me, g, err := s.load(ctx, subject, rawID)
	if err != nil {
		return View{}, err
	}
	if g.OrganizerID != me {
		return View{}, errNotOrganizer()
	}
	invitees, err := s.checkInvitees(ctx, me, req.Users)
	if err != nil {
		return View{}, err
	}
	return s.invite(ctx, g, me, invitees)
}

func (s *Service) ownPlan(ctx context.Context, me, planID bson.ObjectID) error {
	_, err := s.plans.FindOwned(ctx, me, planID)
	if errors.Is(err, planes.ErrNotFound) {
		return errPlanNotFound()
	}
	if err != nil {
		return errStore()
	}
	return nil
}

func (s *Service) newGroup(ctx context.Context, me, planID bson.ObjectID) (Group, error) {
	now := s.now().UTC()
	g := Group{
		ID: bson.NewObjectID(), PlanID: planID, OrganizerID: me, Occupied: 1, CreatedAt: now, UpdatedAt: now,
		Members: []Member{{UserID: me, Role: RoleOrganizer, Status: StatusConfirmed, Since: now}},
	}
	return g, s.repo.Create(ctx, &g)
}

// checkInvitees exige que cada invitado sea otra persona, con cuenta activa y amistad
// aceptada. Un solo error (422) para todos los casos: no revela cuál cuenta existe.
func (s *Service) checkInvitees(ctx context.Context, me bson.ObjectID, raw []string) ([]bson.ObjectID, error) {
	notFriend := invalid("usuarios", "solo puedes invitar a tus amigos")
	ids := make([]bson.ObjectID, 0, len(raw))
	for _, r := range raw {
		id, err := bson.ObjectIDFromHex(r)
		if err != nil || id == me {
			return nil, notFriend
		}
		ids = append(ids, id)
	}
	active, err := s.users.FindPublicByIDs(ctx, ids)
	if err != nil {
		return nil, errStore()
	}
	if len(active) != len(ids) {
		return nil, notFriend
	}
	for _, id := range ids {
		f, ok, err := s.friends.FindByPair(ctx, me, id)
		if err != nil {
			return nil, errStore()
		}
		if !ok || f.Status != amistades.StatusAccepted {
			return nil, notFriend
		}
	}
	return ids, nil
}

// invite agrega a cada invitado con su propia operación atómica. Antes revisa que quepan
// todos; si otra petición llena el grupo a la mitad, los que alcanzaron quedan y es 409.
func (s *Service) invite(ctx context.Context, g Group, me bson.ObjectID, invitees []bson.ObjectID) (View, error) {
	needed := 0
	for _, id := range invitees {
		if m, in := g.Member(id); !in || m.Status == StatusDeclined {
			needed++
		}
	}
	if g.Occupied+needed > MaxMembers {
		return View{}, errFull()
	}
	now := s.now().UTC()
	for _, id := range invitees {
		err := s.repo.Invite(ctx, g.ID, me, id, now)
		if errors.Is(err, ErrFull) {
			return View{}, errFull()
		}
		if err != nil {
			return View{}, errStore()
		}
	}
	return s.view(ctx, g.ID, me)
}

// load trae el grupo de un plan activo donde quien llama es integrante (cualquier estado);
// si no, 404 sin revelar que existe.
func (s *Service) load(ctx context.Context, subject, rawID string) (bson.ObjectID, Group, error) {
	me, err := self(subject)
	if err != nil {
		return me, Group{}, err
	}
	id, err := bson.ObjectIDFromHex(rawID)
	if err != nil {
		return me, Group{}, errGroupNotFound()
	}
	g, err := s.repo.FindByID(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return me, Group{}, errGroupNotFound()
	}
	if err != nil {
		return me, Group{}, errStore()
	}
	if _, in := g.Member(me); !in {
		return me, Group{}, errGroupNotFound()
	}
	active, err := s.plans.ActiveIDs(ctx, []bson.ObjectID{g.PlanID})
	if err != nil {
		return me, Group{}, errStore()
	}
	if !active[g.PlanID] {
		return me, Group{}, errGroupNotFound()
	}
	return me, g, nil
}

// view vuelve a leer el grupo y lo arma para me.
func (s *Service) view(ctx context.Context, id, me bson.ObjectID) (View, error) {
	g, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return View{}, errStore()
	}
	views, err := s.views(ctx, []Group{g}, me)
	if err != nil {
		return View{}, err
	}
	return views[0], nil
}

// views arma varios grupos con una sola lectura de cuentas.
func (s *Service) views(ctx context.Context, groups []Group, me bson.ObjectID) ([]View, error) {
	ids := []bson.ObjectID{}
	for _, g := range groups {
		for _, m := range g.Members {
			ids = append(ids, m.UserID)
		}
	}
	users, err := s.users.FindPublicByIDs(ctx, ids)
	if err != nil {
		return nil, errStore()
	}
	people := make(map[bson.ObjectID]usuarios.User, len(users))
	for _, u := range users {
		people[u.ID] = u
	}
	out := make([]View, 0, len(groups))
	for _, g := range groups {
		out = append(out, newView(g, me, people))
	}
	return out, nil
}

// Get trae un grupo donde quien llama es integrante.
func (s *Service) Get(ctx context.Context, subject, rawID string) (View, error) {
	me, g, err := s.load(ctx, subject, rawID)
	if err != nil {
		return View{}, err
	}
	views, err := s.views(ctx, []Group{g}, me)
	if err != nil {
		return View{}, err
	}
	return views[0], nil
}

// Respond confirma o declina la invitación. Volver a confirmar después de declinar exige
// cupo. El organizador no responde (422).
func (s *Service) Respond(ctx context.Context, subject, rawID string, req RespondRequest) (View, error) {
	me, g, err := s.load(ctx, subject, rawID)
	if err != nil {
		return View{}, err
	}
	mine, _ := g.Member(me)
	if mine.Role == RoleOrganizer {
		return View{}, invalid("estado", "el organizador no responde a su propio grupo")
	}
	if mine.Status != req.Status {
		ok, err := s.repo.SetStatus(ctx, g.ID, me, mine.Status, req.Status, s.now().UTC())
		if err != nil {
			return View{}, errStore()
		}
		if !ok {
			return View{}, s.respondConflict(mine.Status)
		}
	}
	return s.view(ctx, g.ID, me)
}

// respondConflict explica por qué no aplicó: si venía de declinar, el grupo se llenó; si no,
// su estado cambió en otro dispositivo.
func (s *Service) respondConflict(from string) error {
	if from == StatusDeclined {
		return errFull()
	}
	return apperr.Conflict("tu respuesta cambió en otro dispositivo; vuelve a cargar el grupo")
}

// Leave saca a quien llama del grupo. El organizador no sale (422): su plan es el grupo.
func (s *Service) Leave(ctx context.Context, subject, rawID string) error {
	me, g, err := s.load(ctx, subject, rawID)
	if err != nil {
		return err
	}
	if g.OrganizerID == me {
		return invalid("usuario_id", "el organizador no puede salir; borra el plan para cerrar el grupo")
	}
	return s.remove(ctx, g.ID, me)
}

// Remove saca a un integrante; solo el organizador, y nunca a sí mismo.
func (s *Service) Remove(ctx context.Context, subject, rawID, rawUserID string) error {
	me, g, err := s.load(ctx, subject, rawID)
	if err != nil {
		return err
	}
	if g.OrganizerID != me {
		return errNotOrganizer()
	}
	target, err := bson.ObjectIDFromHex(rawUserID)
	if err != nil {
		return apperr.NotFound("integrante no encontrado")
	}
	if target == me {
		return invalid("usuario_id", "el organizador no puede sacarse a sí mismo")
	}
	return s.remove(ctx, g.ID, target)
}

func (s *Service) remove(ctx context.Context, groupID, userID bson.ObjectID) error {
	ok, err := s.repo.RemoveMember(ctx, groupID, userID, s.now().UTC())
	if err != nil {
		return errStore()
	}
	if !ok {
		return apperr.NotFound("integrante no encontrado")
	}
	return nil
}

// List devuelve una página de los grupos de quien llama, del más reciente al más viejo. Los
// grupos de planes borrados no salen (la página puede traer menos de `limite`).
func (s *Service) List(ctx context.Context, subject string, q ListQuery) (pagination.PageResult[View], error) {
	me, err := self(subject)
	if err != nil {
		return pagination.PageResult[View]{}, err
	}
	scope := s.cursor.Scope("grupos", me.Hex())
	after, err := position(s.cursor, scope, q.Cursor)
	if err != nil {
		return pagination.PageResult[View]{}, err
	}
	rows, err := s.repo.ListForMember(ctx, me, after, q.Limit)
	if err != nil {
		return pagination.PageResult[View]{}, errStore()
	}
	next := ""
	if len(rows) > q.Limit {
		rows = rows[:q.Limit]
		last := rows[len(rows)-1]
		next, err = s.cursor.Encode(pagination.Cursor{ID: last.ID.Hex(), Sort: last.UpdatedAt.UTC().Format(time.RFC3339Nano), Scope: scope})
		if err != nil {
			return pagination.PageResult[View]{}, errStore()
		}
	}
	visible, err := s.withActivePlan(ctx, rows)
	if err != nil {
		return pagination.PageResult[View]{}, err
	}
	views, err := s.views(ctx, visible, me)
	if err != nil {
		return pagination.PageResult[View]{}, err
	}
	return pagination.NewPageResult(views, next, q.Limit), nil
}

func (s *Service) withActivePlan(ctx context.Context, rows []Group) ([]Group, error) {
	ids := make([]bson.ObjectID, 0, len(rows))
	for _, g := range rows {
		ids = append(ids, g.PlanID)
	}
	active, err := s.plans.ActiveIDs(ctx, ids)
	if err != nil {
		return nil, errStore()
	}
	out := rows[:0]
	for _, g := range rows {
		if active[g.PlanID] {
			out = append(out, g)
		}
	}
	return out, nil
}

// position lee el cursor firmado; uno manipulado o de otra cuenta es 422.
func position(signer *pagination.Signer, scope, raw string) (*Position, error) {
	c, ok, err := validation.Position(signer, scope, raw)
	if err != nil || !ok {
		return nil, err
	}
	bad := invalid("cursor", "cursor inválido")
	at, err := time.Parse(time.RFC3339Nano, c.Sort)
	if err != nil {
		return nil, bad
	}
	id, err := bson.ObjectIDFromHex(c.ID)
	if err != nil {
		return nil, bad
	}
	return &Position{UpdatedAt: at, ID: id}, nil
}

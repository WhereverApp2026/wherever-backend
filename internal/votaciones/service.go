package votaciones

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/grupos"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

// Groups trae el grupo de la votación.
type Groups interface {
	FindByID(ctx context.Context, id bson.ObjectID) (grupos.Group, error)
}

// Plans lee el plan activo y aplica la ganadora (nv-30).
type Plans interface {
	FindActive(ctx context.Context, id bson.ObjectID) (planes.Plan, error)
}

// Applier cambia el negocio de una parada del plan del dueño y lo recalcula.
type Applier interface {
	ReplaceBusiness(ctx context.Context, ownerID, planID, stopID, businessID bson.ObjectID) (planes.Plan, error)
}

// Places da la tarjeta pública de los negocios visibles (las opciones).
type Places interface {
	FindCards(ctx context.Context, ids []bson.ObjectID, now time.Time) ([]negocios.PlaceItem, error)
}

// Service maneja las votaciones. Las notificaciones `votacion_abierta` y `votacion_cerrada`
// llegan con nv-40/41.
type Service struct {
	repo    *Repository
	groups  Groups
	plans   Plans
	applier Applier
	places  Places
	log     *slog.Logger
	now     func() time.Time
}

// NewService recibe sus dependencias por constructor.
func NewService(repo *Repository, groups Groups, plans Plans, applier Applier, places Places, log *slog.Logger) *Service {
	return &Service{repo: repo, groups: groups, plans: plans, applier: applier, places: places, log: log, now: time.Now}
}

func errVoteNotFound() error  { return apperr.NotFound("votación no encontrada") }
func errGroupNotFound() error { return apperr.NotFound("grupo no encontrado") }
func errClosed() error        { return apperr.Conflict("la votación ya cerró") }
func errStore() error         { return apperr.Unavailable("servicio no disponible") }
func errNotOrganizer() error {
	return apperr.Forbidden("solo el organizador puede hacer esto")
}

func invalid(field, message string) error {
	return apperr.Validation(message, apperr.FieldError{Field: field, Message: message})
}

func self(subject string) (bson.ObjectID, error) {
	id, err := bson.ObjectIDFromHex(subject)
	if err != nil {
		return bson.ObjectID{}, apperr.Unauthorized("autenticación requerida")
	}
	return id, nil
}

// access es quien llama dentro de un grupo de un plan activo.
type access struct {
	me     bson.ObjectID
	group  grupos.Group
	plan   planes.Plan
	member grupos.Member
}

// groupAccess carga el grupo y su plan activo; quien no es integrante (o el plan está
// borrado) recibe 404, igual que si no existiera.
func (s *Service) groupAccess(ctx context.Context, me, groupID bson.ObjectID) (access, error) {
	g, err := s.groups.FindByID(ctx, groupID)
	if errors.Is(err, grupos.ErrNotFound) {
		return access{}, errGroupNotFound()
	}
	if err != nil {
		return access{}, errStore()
	}
	m, in := g.Member(me)
	if !in {
		return access{}, errGroupNotFound()
	}
	p, err := s.plans.FindActive(ctx, g.PlanID)
	if errors.Is(err, planes.ErrNotFound) {
		return access{}, errGroupNotFound()
	}
	if err != nil {
		return access{}, errStore()
	}
	return access{me: me, group: g, plan: p, member: m}, nil
}

// Open abre una votación para una parada del plan del grupo; solo el organizador (201).
func (s *Service) Open(ctx context.Context, subject, rawGroupID string, req OpenRequest) (View, error) {
	me, err := self(subject)
	if err != nil {
		return View{}, err
	}
	groupID, err := bson.ObjectIDFromHex(rawGroupID)
	if err != nil {
		return View{}, errGroupNotFound()
	}
	a, err := s.groupAccess(ctx, me, groupID)
	if err != nil {
		return View{}, err
	}
	if a.group.OrganizerID != me {
		return View{}, errNotOrganizer()
	}
	v, err := s.newVote(ctx, a, req)
	if err != nil {
		return View{}, err
	}
	if err := s.repo.Create(ctx, &v); errors.Is(err, ErrDuplicate) {
		return View{}, apperr.Conflict("esa parada ya tiene una votación abierta")
	} else if err != nil {
		return View{}, errStore()
	}
	return s.view(ctx, v, me)
}

// newVote valida la parada, las opciones (negocios visibles y distintos) y la hora límite.
func (s *Service) newVote(ctx context.Context, a access, req OpenRequest) (Vote, error) {
	stopID, err := bson.ObjectIDFromHex(req.StopID)
	if err != nil || !hasStop(a.plan, stopID) {
		return Vote{}, invalid("parada_id", "la parada no está en el plan")
	}
	now := s.now().UTC()
	closes := req.ClosesAt.UTC()
	if closes.Before(now.Add(MinDuration)) || closes.After(now.Add(MaxDuration)) {
		return Vote{}, invalid("cierra_en", "la hora límite debe estar entre 5 minutos y 7 días a partir de ahora")
	}
	options := make([]bson.ObjectID, 0, len(req.Options))
	for _, raw := range req.Options {
		id, err := bson.ObjectIDFromHex(raw)
		if err != nil {
			return Vote{}, invalid("opciones", "opción inválida")
		}
		options = append(options, id)
	}
	cards, err := s.places.FindCards(ctx, options, now)
	if err != nil {
		return Vote{}, errStore()
	}
	if len(cards) != len(options) {
		return Vote{}, invalid("opciones", "todas las opciones deben ser lugares disponibles")
	}
	return Vote{
		ID: bson.NewObjectID(), GroupID: a.group.ID, PlanID: a.plan.ID, StopID: stopID, Options: options,
		Ballots: map[string]bson.ObjectID{}, ClosesAt: closes, Status: StatusOpen, CreatedBy: a.me, CreatedAt: now, UpdatedAt: now,
	}, nil
}

func hasStop(p planes.Plan, stopID bson.ObjectID) bool {
	for _, st := range p.Paradas {
		if st.ID == stopID {
			return true
		}
	}
	return false
}

// List devuelve las votaciones del grupo a cualquier integrante; antes cierra las vencidas.
func (s *Service) List(ctx context.Context, subject, rawGroupID string) (ListResponse, error) {
	me, err := self(subject)
	if err != nil {
		return ListResponse{}, err
	}
	groupID, err := bson.ObjectIDFromHex(rawGroupID)
	if err != nil {
		return ListResponse{}, errGroupNotFound()
	}
	if _, err := s.groupAccess(ctx, me, groupID); err != nil {
		return ListResponse{}, err
	}
	rows, err := s.repo.ListByGroup(ctx, groupID)
	if err != nil {
		return ListResponse{}, errStore()
	}
	out := ListResponse{Items: make([]View, 0, len(rows))}
	for _, v := range rows {
		if s.due(v) {
			if done, err := s.finish(ctx, v, ClosedByDeadline); err == nil {
				v = done
			}
		}
		view, err := s.view(ctx, v, me)
		if err != nil {
			return ListResponse{}, err
		}
		out.Items = append(out.Items, view)
	}
	return out, nil
}

// voteAccess carga la votación y el acceso de quien llama a su grupo.
func (s *Service) voteAccess(ctx context.Context, subject, rawID string) (Vote, access, error) {
	me, err := self(subject)
	if err != nil {
		return Vote{}, access{}, err
	}
	id, err := bson.ObjectIDFromHex(rawID)
	if err != nil {
		return Vote{}, access{}, errVoteNotFound()
	}
	v, err := s.repo.FindByID(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return Vote{}, access{}, errVoteNotFound()
	}
	if err != nil {
		return Vote{}, access{}, errStore()
	}
	a, err := s.groupAccess(ctx, me, v.GroupID)
	if err != nil {
		return Vote{}, access{}, errVoteNotFound()
	}
	return v, a, nil
}

// Cast guarda o cambia el voto de quien llama. Votan invitados y confirmados; quien declinó
// no (403). Una votación cerrada o vencida = 409.
func (s *Service) Cast(ctx context.Context, subject, rawID string, req CastRequest) (View, error) {
	v, a, err := s.voteAccess(ctx, subject, rawID)
	if err != nil {
		return View{}, err
	}
	if a.member.Status == grupos.StatusDeclined {
		return View{}, apperr.Forbidden("declinaste este plan; confírmalo para votar")
	}
	choice, err := bson.ObjectIDFromHex(req.BusinessID)
	if err != nil || !contains(v.Options, choice) {
		return View{}, invalid("negocio_id", "esa opción no está en la votación")
	}
	ok, err := s.repo.Cast(ctx, v.ID, a.me, choice, s.now().UTC())
	if err != nil {
		return View{}, errStore()
	}
	if !ok {
		if s.due(v) {
			_, _ = s.finish(ctx, v, ClosedByDeadline)
		}
		return View{}, errClosed()
	}
	return s.reload(ctx, v.ID, a.me)
}

func contains(ids []bson.ObjectID, id bson.ObjectID) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

// Close cierra la votación antes de su hora; solo el organizador. Ya cerrada = 409.
func (s *Service) Close(ctx context.Context, subject, rawID string) (View, error) {
	v, a, err := s.voteAccess(ctx, subject, rawID)
	if err != nil {
		return View{}, err
	}
	if a.group.OrganizerID != a.me {
		return View{}, errNotOrganizer()
	}
	if v.Status != StatusOpen {
		return View{}, errClosed()
	}
	done, err := s.finish(ctx, v, ClosedByOrganizer)
	if err != nil {
		return View{}, err
	}
	if done.ClosedBy != ClosedByOrganizer {
		return View{}, errClosed() // la cerró el vencimiento un instante antes
	}
	return s.view(ctx, done, a.me)
}

func (s *Service) due(v Vote) bool {
	return v.Status == StatusOpen && !s.now().Before(v.ClosesAt)
}

// finish cierra la votación (si sigue abierta) y aplica la ganadora al plan (si falta).
// Cerrar es una sola operación condicionada: con dos instancias o una petición en carrera,
// solo una la cierra; las demás releen y siguen con lo que quedó.
func (s *Service) finish(ctx context.Context, v Vote, by string) (Vote, error) {
	v, err := s.closeOpen(ctx, v, by)
	if err != nil || v.Status != StatusClosed || v.Applied != nil {
		return v, err
	}
	// Solo una instancia aplica: la otra ve la reserva y devuelve lo que hay.
	claimed, err := s.repo.ClaimApply(ctx, v.ID, s.now().UTC())
	if err != nil {
		return v, errStore()
	}
	if !claimed {
		return v, nil
	}
	if err := s.apply(ctx, v); err != nil {
		return v, err
	}
	return s.repo.FindByID(ctx, v.ID)
}

// closeOpen cierra la votación si sigue abierta, contando de nuevo si alguien votó en medio
// (hasta 3 intentos), y devuelve cómo quedó.
func (s *Service) closeOpen(ctx context.Context, v Vote, by string) (Vote, error) {
	for range 3 {
		if v.Status != StatusOpen {
			return v, nil
		}
		ok, err := s.repo.Close(ctx, v, WinnerOf(v), by, s.now().UTC())
		if err != nil {
			return v, errStore()
		}
		if v, err = s.repo.FindByID(ctx, v.ID); err != nil {
			return v, errStore()
		}
		if ok {
			return v, nil
		}
	}
	return v, nil
}

// apply pone la ganadora en la parada del plan y deja constancia. Si el plan o la parada ya
// no existen, o el lugar no cabe (cerrado a esa hora), queda `aplicada: false` con motivo;
// un fallo de la base deja la aplicación pendiente para el siguiente intento.
func (s *Service) apply(ctx context.Context, v Vote) error {
	now := s.now().UTC()
	g, err := s.groups.FindByID(ctx, v.GroupID)
	if errors.Is(err, grupos.ErrNotFound) {
		return s.record(ctx, v.ID, false, "el grupo ya no existe", now)
	}
	if err != nil {
		return errStore()
	}
	_, err = s.applier.ReplaceBusiness(ctx, g.OrganizerID, v.PlanID, v.StopID, *v.Winner)
	var appErr *apperr.Error
	switch {
	case err == nil:
		return s.record(ctx, v.ID, true, "", now)
	case errors.As(err, &appErr) && appErr.Kind == apperr.KindNotFound:
		return s.record(ctx, v.ID, false, "el plan o la parada ya no existen", now)
	case errors.As(err, &appErr) && appErr.Kind == apperr.KindValidation:
		return s.record(ctx, v.ID, false, "el lugar ganador no cabe en el plan a esa hora", now)
	default:
		s.log.Warn("votaciones: aplicar la ganadora quedó pendiente", "evento", "votacion_pendiente", "votacion_id", v.ID.Hex())
		return errStore()
	}
}

func (s *Service) record(ctx context.Context, id bson.ObjectID, applied bool, reason string, now time.Time) error {
	if err := s.repo.SetApplied(ctx, id, applied, reason, now); err != nil {
		return errStore()
	}
	return nil
}

// FinishDue cierra y aplica hasta 50 votaciones vencidas o pendientes; lo llama el
// trabajador de cada instancia (Closer). Devuelve cuántas terminó.
func (s *Service) FinishDue(ctx context.Context) (int, error) {
	rows, err := s.repo.Due(ctx, s.now().UTC(), 50)
	if err != nil {
		return 0, err
	}
	done := 0
	for _, v := range rows {
		if _, err := s.finish(ctx, v, ClosedByDeadline); err == nil {
			done++
		}
	}
	return done, nil
}

func (s *Service) reload(ctx context.Context, id, me bson.ObjectID) (View, error) {
	v, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return View{}, errStore()
	}
	return s.view(ctx, v, me)
}

// view arma la votación con la tarjeta de cada opción y el conteo; no dice quién votó qué.
func (s *Service) view(ctx context.Context, v Vote, me bson.ObjectID) (View, error) {
	cards, err := s.places.FindCards(ctx, v.Options, s.now())
	if err != nil {
		return View{}, errStore()
	}
	return newView(v, me, cards), nil
}

package amistades

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/notificaciones"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/text"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// Users es lo que amistades necesita del repositorio de usuarios.
type Users interface {
	FindByID(ctx context.Context, id bson.ObjectID) (usuarios.User, error)
	FindPublicByIDs(ctx context.Context, ids []bson.ObjectID) ([]usuarios.User, error)
}

// Service maneja las amistades. Avisa `solicitud_amistad` a quien la recibe y
// `amistad_aceptada` a quien la envió (nv-40).
type Service struct {
	repo   *Repository
	users  Users
	graph  Graph
	notify notificaciones.Notifier
	log    *slog.Logger
	now    func() time.Time
}

// WithNotifier guarda los avisos en el centro de notificaciones (nv-40).
func (s *Service) WithNotifier(n notificaciones.Notifier) *Service {
	s.notify = n
	return s
}

// Graph recibe a quien aceptó, eliminó o bloqueó una amistad para reflejarlo en Neo4j
// (nv-52). Recalcular a esa persona basta: la relación AMIGO_DE es una por par. Es
// eventual: un fallo no cambia la respuesta.
type Graph interface {
	Touch(ctx context.Context, userID bson.ObjectID) error
}

type noGraph struct{}

func (noGraph) Touch(context.Context, bson.ObjectID) error { return nil }

// WithGraph refleja las amistades aceptadas en el grafo (nv-52).
func (s *Service) WithGraph(g Graph) *Service {
	s.graph = g
	return s
}

// NewService recibe los repositorios por constructor.
func NewService(repo *Repository, users Users, log *slog.Logger) (*Service, error) {
	if repo == nil || users == nil {
		return nil, errors.New("amistades: faltan los repositorios")
	}
	return &Service{repo: repo, users: users, graph: noGraph{}, notify: notificaciones.Noop{}, log: log, now: func() time.Time { return time.Now().UTC() }}, nil
}

func unavailable(err error) error {
	return apperr.Unavailable("servicio no disponible").WithDetail(err.Error())
}

func notFound() error { return apperr.NotFound("no encontramos a esa persona o esa solicitud") }

func self(subject string) (bson.ObjectID, error) {
	id, err := bson.ObjectIDFromHex(subject)
	if err != nil {
		return bson.ObjectID{}, apperr.Unauthorized("credenciales inválidas")
	}
	return id, nil
}

// pathID lee `{id}` de la ruta; uno inválido es como uno que no existe.
func pathID(raw string) (bson.ObjectID, error) {
	id, err := bson.ObjectIDFromHex(raw)
	if err != nil {
		return bson.ObjectID{}, notFound()
	}
	return id, nil
}

// Send envía una solicitud a req.UserID. Si esa persona ya me había enviado una, quedan como
// amigos. A quien me bloqueó responde 404, igual que a una cuenta que no existe.
func (s *Service) Send(ctx context.Context, subject string, req SendRequest) (Item, error) {
	me, err := self(subject)
	if err != nil {
		return Item{}, err
	}
	person, err := s.target(ctx, me, req.UserID)
	if err != nil {
		return Item{}, err
	}
	target := person.ID
	// Dos vueltas: si otra solicitud del mismo par gana la carrera, se resuelve contra ella.
	for range 2 {
		existing, ok, err := s.repo.FindByPair(ctx, me, target)
		if err != nil {
			return Item{}, unavailable(err)
		}
		if ok {
			return s.resolveExisting(ctx, me, existing, person)
		}
		now := s.now()
		f := Friendship{
			ID: bson.NewObjectID(), From: me, To: target, Status: StatusPending,
			Pair: PairKey(me, target), CreatedAt: now, UpdatedAt: now,
		}
		err = s.repo.Insert(ctx, &f)
		if err == nil {
			s.notify.Emit(ctx, target, notificaciones.KindFriendRequest, map[string]string{"usuario_id": me.Hex()})
			return newItem(f, me, person), nil
		}
		if !errors.Is(err, ErrDuplicate) {
			return Item{}, unavailable(err)
		}
	}
	return Item{}, apperr.Conflict("la solicitud cambió; inténtalo de nuevo")
}

// target lee a quien va la solicitud: a uno mismo es 422; una cuenta que no existe o no está
// activa, 404.
func (s *Service) target(ctx context.Context, me bson.ObjectID, raw string) (usuarios.User, error) {
	id, err := bson.ObjectIDFromHex(raw)
	if err != nil || id == me {
		return usuarios.User{}, apperr.Validation("solicitud inválida",
			apperr.FieldError{Field: "usuario_id", Message: "elige a otra persona"})
	}
	person, err := s.users.FindByID(ctx, id)
	if errors.Is(err, usuarios.ErrNotFound) || (err == nil && person.Status != usuarios.StatusActive) {
		return usuarios.User{}, notFound()
	}
	if err != nil {
		return usuarios.User{}, unavailable(err)
	}
	return person, nil
}

// resolveExisting decide qué pasa al enviar una solicitud a quien ya tiene relación conmigo.
func (s *Service) resolveExisting(ctx context.Context, me bson.ObjectID, f Friendship, person usuarios.User) (Item, error) {
	switch {
	case f.Status == StatusPending && f.To == me:
		accepted, ok, err := s.repo.Accept(ctx, f.ID, me, s.now())
		if err != nil {
			return Item{}, unavailable(err)
		}
		if !ok {
			return Item{}, apperr.Conflict("la solicitud cambió; inténtalo de nuevo")
		}
		_ = s.graph.Touch(ctx, me)
		s.notify.Emit(ctx, f.From, notificaciones.KindFriendAccepted, map[string]string{"usuario_id": me.Hex()})
		return newItem(accepted, me, person), nil
	case f.Status == StatusBlocked && f.From != me:
		return Item{}, notFound()
	case f.Status == StatusAccepted:
		return Item{}, apperr.Conflict("ya son amigos")
	case f.Status == StatusBlocked:
		return Item{}, apperr.Conflict("bloqueaste a esta persona")
	default:
		return Item{}, apperr.Conflict("ya le enviaste una solicitud")
	}
}

// List devuelve amigos, solicitudes recibidas o enviadas (de la más reciente a la más
// antigua) y el contador de solicitudes recibidas pendientes. `q` filtra por nombre o nombre
// de usuario, sin acentos ni mayúsculas.
func (s *Service) List(ctx context.Context, subject string, q ListQuery) (ListResponse, error) {
	me, err := self(subject)
	if err != nil {
		return ListResponse{}, err
	}
	kind := Kind(q.Status)
	if kind == "" {
		kind = KindFriends
	}
	rows, err := s.repo.List(ctx, me, kind)
	if err != nil {
		return ListResponse{}, unavailable(err)
	}
	pending, err := s.repo.CountReceived(ctx, me)
	if err != nil {
		return ListResponse{}, unavailable(err)
	}
	byID, err := s.people(ctx, me, rows)
	if err != nil {
		return ListResponse{}, err
	}
	needle := text.Normalize(strings.TrimPrefix(strings.TrimSpace(q.Query), "@"))
	out := ListResponse{Results: make([]Item, 0, len(rows)), Pending: pending}
	for _, f := range rows {
		// Una cuenta eliminada o suspendida no aparece en la lista.
		p, ok := byID[f.Other(me)]
		if !ok || !matches(p, needle) {
			continue
		}
		out.Results = append(out.Results, newItem(f, me, p))
	}
	return out, nil
}

// people lee lo público de la otra persona de cada relación, por id; solo cuentas activas.
func (s *Service) people(ctx context.Context, me bson.ObjectID, rows []Friendship) (map[bson.ObjectID]usuarios.User, error) {
	ids := make([]bson.ObjectID, 0, len(rows))
	for _, f := range rows {
		ids = append(ids, f.Other(me))
	}
	found, err := s.users.FindPublicByIDs(ctx, ids)
	if err != nil {
		return nil, unavailable(err)
	}
	byID := make(map[bson.ObjectID]usuarios.User, len(found))
	for _, p := range found {
		byID[p.ID] = p
	}
	return byID, nil
}

func matches(p usuarios.User, needle string) bool {
	if needle == "" {
		return true
	}
	return strings.Contains(text.Normalize(p.Name), needle) || strings.Contains(p.Username, needle)
}

// Respond acepta o rechaza una solicitud recibida. Rechazar la borra sin avisar a quien envió.
// Solo quien la recibió responde: para cualquier otra persona es 404.
func (s *Service) Respond(ctx context.Context, subject, rawID string, req RespondRequest) (Item, error) {
	me, err := self(subject)
	if err != nil {
		return Item{}, err
	}
	f, err := s.incoming(ctx, me, rawID)
	if err != nil {
		return Item{}, err
	}
	person, err := s.person(ctx, f.From)
	if err != nil {
		return Item{}, err
	}
	answered := apperr.Conflict("la solicitud ya fue respondida")
	if req.Action == ActionReject {
		removed, err := s.repo.Reject(ctx, f.ID, me)
		if err != nil {
			return Item{}, unavailable(err)
		}
		if !removed {
			return Item{}, answered
		}
		f.Status, f.UpdatedAt = StatusRejected, s.now()
		return newItem(f, me, person), nil
	}
	accepted, ok, err := s.repo.Accept(ctx, f.ID, me, s.now())
	if err != nil {
		return Item{}, unavailable(err)
	}
	if !ok {
		return Item{}, answered
	}
	_ = s.graph.Touch(ctx, me)
	s.notify.Emit(ctx, f.From, notificaciones.KindFriendAccepted, map[string]string{"usuario_id": me.Hex()})
	return newItem(accepted, me, person), nil
}

// incoming lee la solicitud pendiente que recibió me. Si no la recibió me (o es un bloqueo)
// es 404; si ya se respondió, 409.
func (s *Service) incoming(ctx context.Context, me bson.ObjectID, rawID string) (Friendship, error) {
	id, err := pathID(rawID)
	if err != nil {
		return Friendship{}, err
	}
	f, ok, err := s.repo.FindForParticipant(ctx, id, me)
	if err != nil {
		return Friendship{}, unavailable(err)
	}
	if !ok || f.To != me || f.Status == StatusBlocked {
		return Friendship{}, notFound()
	}
	if f.Status != StatusPending {
		return Friendship{}, apperr.Conflict("la solicitud ya fue respondida")
	}
	return f, nil
}

// person lee lo público de la otra persona; si ya no está activa, la relación es 404.
func (s *Service) person(ctx context.Context, id bson.ObjectID) (usuarios.User, error) {
	people, err := s.users.FindPublicByIDs(ctx, []bson.ObjectID{id})
	if err != nil {
		return usuarios.User{}, unavailable(err)
	}
	if len(people) == 0 {
		return usuarios.User{}, notFound()
	}
	return people[0], nil
}

// Remove elimina a un amigo o cancela una solicitud (enviada o recibida).
func (s *Service) Remove(ctx context.Context, subject, rawID string) error {
	me, err := self(subject)
	if err != nil {
		return err
	}
	id, err := pathID(rawID)
	if err != nil {
		return err
	}
	removed, err := s.repo.Remove(ctx, id, me)
	if err != nil {
		return unavailable(err)
	}
	if !removed {
		return notFound()
	}
	_ = s.graph.Touch(ctx, me)
	return nil
}

// Block bloquea a la otra persona de la relación sin avisarle: deja de verme en la búsqueda y
// no puede enviarme solicitudes. Bloquear otra vez no cambia nada.
func (s *Service) Block(ctx context.Context, subject, rawID string) error {
	me, err := self(subject)
	if err != nil {
		return err
	}
	id, err := pathID(rawID)
	if err != nil {
		return err
	}
	f, ok, err := s.repo.FindForParticipant(ctx, id, me)
	if err != nil {
		return unavailable(err)
	}
	if !ok || (f.Status == StatusBlocked && f.From != me) {
		return notFound()
	}
	if f.Status == StatusBlocked {
		return nil
	}
	_, ok, err = s.repo.Block(ctx, f, me, s.now())
	if err != nil {
		return unavailable(err)
	}
	if !ok {
		return apperr.Conflict("la relación cambió; inténtalo de nuevo")
	}
	_ = s.graph.Touch(ctx, me)
	return nil
}

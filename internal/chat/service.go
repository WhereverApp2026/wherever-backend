package chat

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/grupos"
	"github.com/WhereverApp2026/wherever-backend/internal/notificaciones"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// Groups lee el grupo y guarda su último mensaje (grupos.Repository).
type Groups interface {
	FindByID(ctx context.Context, id bson.ObjectID) (grupos.Group, error)
	SetLastMessage(ctx context.Context, groupID bson.ObjectID, last grupos.LastMessage) error
	ClearLastMessage(ctx context.Context, groupID, messageID bson.ObjectID) error
}

// Plans dice qué planes siguen activos: el grupo de un plan borrado deja de verse.
type Plans interface {
	ActiveIDs(ctx context.Context, ids []bson.ObjectID) (map[bson.ObjectID]bool, error)
}

// Users da lo público de los autores activos.
type Users interface {
	FindPublicByIDs(ctx context.Context, ids []bson.ObjectID) ([]usuarios.User, error)
}

// Service maneja el chat. Solo los integrantes confirmados leen y escriben; se comprueba en
// cada petición contra el grupo, así quien sale, es sacado o declina deja de leer lo nuevo.
type Service struct {
	repo   *Repository
	groups Groups
	plans  Plans
	users  Users
	cursor *pagination.Signer
	notify notificaciones.Notifier
	log    *slog.Logger
	now    func() time.Time
}

// NewService recibe sus dependencias por constructor.
func NewService(repo *Repository, groups Groups, plans Plans, users Users, cursor *pagination.Signer, log *slog.Logger) *Service {
	return &Service{repo: repo, groups: groups, plans: plans, users: users, cursor: cursor, notify: notificaciones.Noop{}, log: log, now: time.Now}
}

// WithNotifier avisa `mensaje_grupo` a los demás confirmados (nv-40/41), agrupado por NoticeEvery.
func (s *Service) WithNotifier(n notificaciones.Notifier) *Service {
	s.notify = n
	return s
}

func errGroupNotFound() error   { return apperr.NotFound("grupo no encontrado") }
func errMessageNotFound() error { return apperr.NotFound("mensaje no encontrado") }
func errStore() error           { return apperr.Unavailable("servicio no disponible") }

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

// access carga el grupo si me es integrante confirmado y su plan sigue activo; si no, 404
// (no se distingue de un grupo que no existe).
func (s *Service) access(ctx context.Context, subject, rawGroupID string) (bson.ObjectID, grupos.Group, error) {
	me, err := self(subject)
	if err != nil {
		return me, grupos.Group{}, err
	}
	groupID, err := bson.ObjectIDFromHex(rawGroupID)
	if err != nil {
		return me, grupos.Group{}, errGroupNotFound()
	}
	g, err := s.visibleGroup(ctx, groupID)
	if err != nil {
		return me, grupos.Group{}, err
	}
	if m, in := g.Member(me); !in || m.Status != grupos.StatusConfirmed {
		return me, grupos.Group{}, errGroupNotFound()
	}
	return me, g, nil
}

func (s *Service) visibleGroup(ctx context.Context, groupID bson.ObjectID) (grupos.Group, error) {
	g, err := s.groups.FindByID(ctx, groupID)
	if errors.Is(err, grupos.ErrNotFound) {
		return grupos.Group{}, errGroupNotFound()
	}
	if err != nil {
		return grupos.Group{}, errStore()
	}
	active, err := s.plans.ActiveIDs(ctx, []bson.ObjectID{g.PlanID})
	if err != nil {
		return grupos.Group{}, errStore()
	}
	if !active[g.PlanID] {
		return grupos.Group{}, errGroupNotFound()
	}
	return g, nil
}

// List trae una página del chat. Sin `despues_de`, del más nuevo al más viejo con cursor
// hacia atrás; con `despues_de`, lo nuevo del más viejo al más nuevo (ponerse al día).
func (s *Service) List(ctx context.Context, subject, rawGroupID string, q ListQuery) (pagination.PageResult[View], error) {
	me, g, err := s.access(ctx, subject, rawGroupID)
	if err != nil {
		return pagination.PageResult[View]{}, err
	}
	query := strings.TrimSpace(q.Query)
	scope := s.cursor.Scope("chat", g.ID.Hex(), me.Hex(), query)
	page, err := s.page(scope, query, q)
	if err != nil {
		return pagination.PageResult[View]{}, err
	}
	rows, err := s.repo.List(ctx, g.ID, page)
	if err != nil {
		return pagination.PageResult[View]{}, errStore()
	}
	next := ""
	if len(rows) > q.Limit {
		rows = rows[:q.Limit]
		dir := "antes"
		if page.After != nil {
			dir = "despues"
		}
		next, err = s.cursor.Encode(pagination.Cursor{ID: rows[len(rows)-1].ID.Hex(), Sort: dir, Scope: scope})
		if err != nil {
			return pagination.PageResult[View]{}, errStore()
		}
	}
	views, err := s.views(ctx, rows, me)
	if err != nil {
		return pagination.PageResult[View]{}, err
	}
	return pagination.NewPageResult(views, next, q.Limit), nil
}

// page arma la consulta: `despues_de` o el cursor firmado (que manda si vienen los dos).
func (s *Service) page(scope, query string, q ListQuery) (Page, error) {
	page := Page{Query: query, Limit: q.Limit}
	if q.After != "" {
		after, _ := bson.ObjectIDFromHex(q.After) // el DTO ya exige 24 hex
		page.After = &after
	}
	c, ok, err := validation.Position(s.cursor, scope, q.Cursor)
	if err != nil || !ok {
		return page, err
	}
	return fromCursor(c, page)
}

// fromCursor sigue en la dirección que guardó el cursor; uno manipulado es 422.
func fromCursor(c pagination.Cursor, page Page) (Page, error) {
	id, err := bson.ObjectIDFromHex(c.ID)
	if err != nil {
		return page, invalid("cursor", "cursor inválido")
	}
	switch c.Sort {
	case "antes":
		page.Before, page.After = &id, nil
	case "despues":
		page.After, page.Before = &id, nil
	default:
		return page, invalid("cursor", "cursor inválido")
	}
	return page, nil
}

func (s *Service) views(ctx context.Context, rows []Message, me bson.ObjectID) ([]View, error) {
	ids := []bson.ObjectID{}
	for _, m := range rows {
		if m.AuthorID != nil {
			ids = append(ids, *m.AuthorID)
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
	out := make([]View, 0, len(rows))
	for _, m := range rows {
		out = append(out, newView(m, me, people))
	}
	return out, nil
}

func (s *Service) one(ctx context.Context, m Message, me bson.ObjectID) (View, error) {
	views, err := s.views(ctx, []Message{m}, me)
	if err != nil {
		return View{}, err
	}
	return views[0], nil
}

// CleanText quita los espacios a los lados y exige 1 a MaxText caracteres.
func CleanText(raw string) (string, error) {
	text := strings.TrimSpace(raw)
	if text == "" || utf8.RuneCountInString(text) > MaxText {
		return "", invalid("texto", "el texto debe tener de 1 a 1000 caracteres")
	}
	return text, nil
}

// Send guarda el mensaje (201) o devuelve el ya guardado con el mismo `id_cliente` (200).
func (s *Service) Send(ctx context.Context, subject, rawGroupID string, req SendRequest) (View, bool, error) {
	me, g, err := s.access(ctx, subject, rawGroupID)
	if err != nil {
		return View{}, false, err
	}
	text, err := CleanText(req.Text)
	if err != nil {
		return View{}, false, err
	}
	if prev, err := s.repo.FindByClientID(ctx, g.ID, req.ClientID); err == nil {
		v, err := s.one(ctx, prev, me)
		return v, false, err
	} else if !errors.Is(err, ErrNotFound) {
		return View{}, false, errStore()
	}
	now := s.now().UTC()
	sent, err := s.repo.CountSince(ctx, me, now.Add(-SendWindow))
	if err != nil {
		return View{}, false, errStore()
	}
	if sent >= SendLimit {
		return View{}, false, apperr.RateLimited(int(SendWindow.Seconds()))
	}
	m := Message{ID: bson.NewObjectID(), GroupID: g.ID, AuthorID: &me, Kind: KindText, Text: text, Reactions: []Reaction{}, ClientID: req.ClientID, CreatedAt: now}
	if err := s.repo.Insert(ctx, &m); errors.Is(err, ErrDuplicate) {
		prev, err := s.repo.FindByClientID(ctx, g.ID, req.ClientID)
		if err != nil {
			return View{}, false, errStore()
		}
		v, err := s.one(ctx, prev, me)
		return v, false, err
	} else if err != nil {
		return View{}, false, errStore()
	}
	s.afterInsert(ctx, g, m)
	v, err := s.one(ctx, m, me)
	return v, true, err
}

// afterInsert actualiza `ultimo_mensaje` y avisa. Si falla, el mensaje ya quedó: solo se registra.
func (s *Service) afterInsert(ctx context.Context, g grupos.Group, m Message) {
	last := grupos.LastMessage{MessageID: m.ID, Text: Preview(m.Text), AuthorID: m.AuthorID, At: m.CreatedAt}
	if err := s.groups.SetLastMessage(ctx, g.ID, last); err != nil {
		s.log.WarnContext(ctx, "chat_ultimo_mensaje_fallo", slog.String("grupo_id", g.ID.Hex()), slog.String("mensaje_id", m.ID.Hex()))
	}
	s.announce(ctx, g, m)
}

// announce manda `mensaje_grupo` a los demás confirmados, uno por grupo y persona cada
// NoticeEvery. `datos` solo lleva el id del grupo: el push es genérico (sin texto ni autor).
func (s *Service) announce(ctx context.Context, g grupos.Group, m Message) {
	now := s.now().UTC()
	data := map[string]string{"grupo_id": g.ID.Hex()}
	for _, member := range g.Members {
		if member.Status != grupos.StatusConfirmed || (m.AuthorID != nil && member.UserID == *m.AuthorID) {
			continue
		}
		claimed, err := s.repo.ClaimNotice(ctx, g.ID, member.UserID, now)
		if err != nil {
			s.log.WarnContext(ctx, "chat_aviso_fallo", slog.String("grupo_id", g.ID.Hex()))
			continue
		}
		if claimed {
			s.notify.Emit(ctx, member.UserID, notificaciones.KindGroupMessage, data)
		}
	}
}

func (s *Service) message(ctx context.Context, groupID bson.ObjectID, rawID string) (Message, error) {
	id, err := bson.ObjectIDFromHex(rawID)
	if err != nil {
		return Message{}, errMessageNotFound()
	}
	m, err := s.repo.FindInGroup(ctx, groupID, id)
	if errors.Is(err, ErrNotFound) {
		return Message{}, errMessageNotFound()
	}
	if err != nil {
		return Message{}, errStore()
	}
	return m, nil
}

// React pone, cambia o quita (emoji null) la reacción de quien llama. Un mensaje borrado o
// del sistema no recibe reacciones (404).
func (s *Service) React(ctx context.Context, subject, rawGroupID, rawID string, req ReactRequest) (View, error) {
	me, g, err := s.access(ctx, subject, rawGroupID)
	if err != nil {
		return View{}, err
	}
	emoji := ""
	if req.Emoji != nil {
		if !IsEmoji(*req.Emoji) {
			return View{}, invalid("emoji", "emoji no permitido")
		}
		emoji = *req.Emoji
	}
	id, err := bson.ObjectIDFromHex(rawID)
	if err != nil {
		return View{}, errMessageNotFound()
	}
	ok, err := s.repo.SetReaction(ctx, g.ID, id, me, emoji)
	if err != nil {
		return View{}, errStore()
	}
	if !ok {
		return View{}, errMessageNotFound()
	}
	m, err := s.message(ctx, g.ID, rawID)
	if err != nil {
		return View{}, err
	}
	return s.one(ctx, m, me)
}

// Delete vacía un mensaje propio (204, también si ya estaba borrado). El de otra persona es 404.
func (s *Service) Delete(ctx context.Context, subject, rawGroupID, rawID string) error {
	me, g, err := s.access(ctx, subject, rawGroupID)
	if err != nil {
		return err
	}
	id, err := bson.ObjectIDFromHex(rawID)
	if err != nil {
		return errMessageNotFound()
	}
	ok, err := s.repo.SoftDelete(ctx, g.ID, id, me, s.now().UTC())
	if err != nil {
		return errStore()
	}
	if !ok {
		return errMessageNotFound()
	}
	if err := s.groups.ClearLastMessage(ctx, g.ID, id); err != nil {
		s.log.WarnContext(ctx, "chat_ultimo_mensaje_fallo", slog.String("grupo_id", g.ID.Hex()), slog.String("mensaje_id", id.Hex()))
	}
	return nil
}

// PublicarSistema agrega un aviso del sistema al chat del grupo (p. ej. el cierre de una
// votación, nv-39). No avisa por push: el módulo que lo origina ya avisa con su propio tipo.
func (s *Service) PublicarSistema(ctx context.Context, groupID bson.ObjectID, text string) error {
	text, err := CleanText(text)
	if err != nil {
		return err
	}
	g, err := s.groups.FindByID(ctx, groupID)
	if err != nil {
		return err
	}
	m := Message{ID: bson.NewObjectID(), GroupID: g.ID, Kind: KindSystem, Text: text, Reactions: []Reaction{}, CreatedAt: s.now().UTC()}
	if err := s.repo.Insert(ctx, &m); err != nil {
		return err
	}
	last := grupos.LastMessage{MessageID: m.ID, Text: Preview(m.Text), At: m.CreatedAt}
	return s.groups.SetLastMessage(ctx, g.ID, last)
}

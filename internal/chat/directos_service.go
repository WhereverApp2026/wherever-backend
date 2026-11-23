package chat

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/grupos"
	"github.com/WhereverApp2026/wherever-backend/internal/moderacion"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/notificaciones"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// Friends son las relaciones de nv-36 (lo cumple *amistades.Repository): amistad aceptada,
// bloqueo y silencio (nv-70).
type Friends interface {
	FindByPair(ctx context.Context, a, b bson.ObjectID) (amistades.Friendship, bool, error)
	BlockedWith(ctx context.Context, me bson.ObjectID) ([]bson.ObjectID, error)
}

// VisiblePlans decide si quien llama ve el plan (lo cumple *planes.Service, nv-83/nv-84).
type VisiblePlans interface {
	Visible(ctx context.Context, subject, rawID string) (bson.ObjectID, error)
}

// PlanReader da el nombre del plan activo (lo cumple *planes.Repository).
type PlanReader interface {
	FindActive(ctx context.Context, id bson.ObjectID) (planes.Plan, error)
}

// Places da el lugar visible (lo cumple *negocios.Repository).
type Places interface {
	FindVisible(ctx context.Context, id bson.ObjectID) (negocios.Business, bool, error)
}

// Moments decide si quien llama ve el momento (lo cumple *postales.Service, nv-63/nv-82).
type Moments interface {
	Visible(ctx context.Context, me bson.ObjectID, raw string) (bson.ObjectID, error)
}

// PlanGroups da el grupo del plan para la tarjeta de invitación (lo cumple *grupos.Repository).
type PlanGroups interface {
	FindByPlan(ctx context.Context, planID bson.ObjectID) (grupos.Group, error)
}

// Moderator evalúa el texto con las reglas de nv-71, sin IA (moderacion.New sin modelo).
type Moderator interface {
	Evaluar(ctx context.Context, c moderacion.Contenido) moderacion.Resultado
}

// Grouped avisa agrupado por objeto (lo cumple *notificaciones.Agrupador).
type Grouped interface {
	Emit(ctx context.Context, to bson.ObjectID, kind, objeto string, data map[string]string)
}

// DirectDeps son las dependencias de DirectService.
type DirectDeps struct {
	Messages      *Repository
	Conversations *ConversationRepository
	Friends       Friends
	Users         Users
	Plans         VisiblePlans
	PlanReader    PlanReader
	Places        Places
	Moments       Moments
	Groups        PlanGroups
	Moderator     Moderator
	Notices       Grouped
	Cursor        *pagination.Signer
	Log           *slog.Logger
}

// DirectService maneja los mensajes directos entre amigos (nv-87). Solo los dos participantes
// leen; escribir pide la amistad aceptada; un bloqueo corta la conversación para los dos. El
// permiso se revisa en cada petición, así el bloqueo surte efecto de inmediato.
type DirectService struct {
	d   DirectDeps
	now func() time.Time
}

// NewDirectService recibe sus dependencias por constructor.
func NewDirectService(d DirectDeps) *DirectService {
	if d.Log == nil {
		d.Log = slog.Default()
	}
	return &DirectService{d: d, now: time.Now}
}

func errConversationNotFound() error { return apperr.NotFound("conversación no encontrada") }

func errAttachment() error {
	return invalid("adjunto", "adjunto no disponible")
}

// friendship es la relación entre me y other; found = false si no hay.
func (s *DirectService) friendship(ctx context.Context, me, other bson.ObjectID) (amistades.Friendship, bool, error) {
	f, found, err := s.d.Friends.FindByPair(ctx, me, other)
	if err != nil {
		return amistades.Friendship{}, false, errStore()
	}
	return f, found, nil
}

// access carga la conversación si me participa y no hay bloqueo; con write además pide la
// amistad aceptada. Todo lo demás es 404, igual que una conversación que no existe.
func (s *DirectService) access(ctx context.Context, subject, rawID string, write bool) (bson.ObjectID, Conversation, amistades.Friendship, error) {
	me, c, err := s.conversation(ctx, subject, rawID)
	if err != nil {
		return me, Conversation{}, amistades.Friendship{}, err
	}
	f, found, err := s.friendship(ctx, me, c.Other(me))
	if err != nil {
		return me, Conversation{}, amistades.Friendship{}, err
	}
	if found && f.Status == amistades.StatusBlocked {
		return me, Conversation{}, amistades.Friendship{}, errConversationNotFound()
	}
	if write && (!found || f.Status != amistades.StatusAccepted) {
		return me, Conversation{}, amistades.Friendship{}, errConversationNotFound()
	}
	return me, c, f, nil
}

// conversation carga la conversación de quien llama; si no participa, 404.
func (s *DirectService) conversation(ctx context.Context, subject, rawID string) (bson.ObjectID, Conversation, error) {
	me, err := self(subject)
	if err != nil {
		return me, Conversation{}, err
	}
	id, err := bson.ObjectIDFromHex(rawID)
	if err != nil {
		return me, Conversation{}, errConversationNotFound()
	}
	c, err := s.d.Conversations.FindFor(ctx, id, me)
	if errors.Is(err, ErrConversationNotFound) {
		return me, Conversation{}, errConversationNotFound()
	}
	if err != nil {
		return me, Conversation{}, errStore()
	}
	return me, c, nil
}

// Open trae (200) o crea (201) la conversación con un amigo activo. Sin amistad aceptada, con
// bloqueo o con la otra cuenta inactiva: 404.
func (s *DirectService) Open(ctx context.Context, subject string, req OpenRequest) (ConversationView, bool, error) {
	me, err := self(subject)
	if err != nil {
		return ConversationView{}, false, err
	}
	other, people, err := s.activeFriend(ctx, me, req.UserID)
	if err != nil {
		return ConversationView{}, false, err
	}
	c, created, err := s.d.Conversations.Open(ctx, me, other, s.now().UTC())
	if err != nil {
		return ConversationView{}, false, errStore()
	}
	unread, err := s.d.Messages.UnreadCounts(ctx, me, []Conversation{c})
	if err != nil {
		return ConversationView{}, false, errStore()
	}
	return newConversationView(c, me, people, unread), created, nil
}

// activeFriend es la persona si es amiga aceptada de me (sin bloqueo) y su cuenta está activa;
// si no, 404 sin decir por qué.
func (s *DirectService) activeFriend(ctx context.Context, me bson.ObjectID, raw string) (bson.ObjectID, map[bson.ObjectID]usuarios.User, error) {
	notFound := apperr.NotFound("persona no encontrada")
	other, err := bson.ObjectIDFromHex(raw)
	if err != nil || other == me {
		return other, nil, notFound
	}
	f, found, err := s.friendship(ctx, me, other)
	if err != nil {
		return other, nil, err
	}
	if !found || f.Status != amistades.StatusAccepted {
		return other, nil, notFound
	}
	people, err := s.people(ctx, []bson.ObjectID{other})
	if err != nil {
		return other, nil, err
	}
	if _, ok := people[other]; !ok {
		return other, nil, notFound
	}
	return other, people, nil
}

func (s *DirectService) people(ctx context.Context, ids []bson.ObjectID) (map[bson.ObjectID]usuarios.User, error) {
	rows, err := s.d.Users.FindPublicByIDs(ctx, ids)
	if err != nil {
		return nil, errStore()
	}
	out := make(map[bson.ObjectID]usuarios.User, len(rows))
	for _, u := range rows {
		out[u.ID] = u
	}
	return out, nil
}

// List trae las conversaciones con al menos un mensaje, la más reciente primero (hasta
// MaxConversations), sin las de personas bloqueadas en cualquier sentido.
func (s *DirectService) List(ctx context.Context, subject string) (ConversationList, error) {
	me, err := self(subject)
	if err != nil {
		return ConversationList{}, err
	}
	blocked, err := s.d.Friends.BlockedWith(ctx, me)
	if err != nil {
		return ConversationList{}, errStore()
	}
	rows, err := s.d.Conversations.ListFor(ctx, me, MaxConversations)
	if err != nil {
		return ConversationList{}, errStore()
	}
	kept := make([]Conversation, 0, len(rows))
	others := []bson.ObjectID{}
	for _, c := range rows {
		if c.Last == nil || contains(blocked, c.Other(me)) {
			continue
		}
		kept = append(kept, c)
		others = append(others, c.Other(me))
	}
	people, err := s.people(ctx, others)
	if err != nil {
		return ConversationList{}, err
	}
	unread, err := s.d.Messages.UnreadCounts(ctx, me, kept)
	if err != nil {
		return ConversationList{}, errStore()
	}
	out := ConversationList{Items: make([]ConversationView, 0, len(kept))}
	for _, c := range kept {
		out.Items = append(out.Items, newConversationView(c, me, people, unread))
	}
	return out, nil
}

// Messages trae una página de la conversación: sin `despues_de`, del más nuevo al más viejo
// con cursor firmado; con `despues_de`, lo nuevo (como el chat de grupo, nv-57).
func (s *DirectService) Messages(ctx context.Context, subject, rawID string, q DirectListQuery) (pagination.PageResult[DirectView], error) {
	me, c, _, err := s.access(ctx, subject, rawID, false)
	if err != nil {
		return pagination.PageResult[DirectView]{}, err
	}
	scope := s.d.Cursor.Scope("directos", c.ID.Hex(), me.Hex())
	page, err := pageFor(s.d.Cursor, scope, q.Cursor, q.After, q.Limit)
	if err != nil {
		return pagination.PageResult[DirectView]{}, err
	}
	rows, err := s.d.Messages.List(ctx, directThread(c.ID), page)
	if err != nil {
		return pagination.PageResult[DirectView]{}, errStore()
	}
	next := ""
	if len(rows) > q.Limit {
		rows = rows[:q.Limit]
		dir := "antes"
		if page.After != nil {
			dir = "despues"
		}
		next, err = s.d.Cursor.Encode(pagination.Cursor{ID: rows[len(rows)-1].ID.Hex(), Sort: dir, Scope: scope})
		if err != nil {
			return pagination.PageResult[DirectView]{}, errStore()
		}
	}
	return pagination.NewPageResult(s.views(ctx, c, rows, me), next, q.Limit), nil
}

// views arma los mensajes con el estado de la invitación de cada plan adjunto: el de la
// persona que lo recibió en el grupo del plan.
func (s *DirectService) views(ctx context.Context, c Conversation, rows []Message, me bson.ObjectID) []DirectView {
	invites := map[bson.ObjectID]string{}
	groups := map[bson.ObjectID]*grupos.Group{}
	for _, m := range rows {
		a := m.Attachment
		if a == nil || a.Kind != AttachPlan || m.AuthorID == nil || m.DeletedAt != nil {
			continue
		}
		g, seen := groups[a.ID]
		if !seen {
			g = s.planGroup(ctx, a.ID)
			groups[a.ID] = g
		}
		if state := inviteState(g, c.Other(*m.AuthorID)); state != "" {
			invites[m.ID] = state
		}
	}
	out := make([]DirectView, 0, len(rows))
	for _, m := range rows {
		out = append(out, newDirectView(m, me, invites))
	}
	return out
}

// inviteState es el estado de quien recibió el plan en su grupo: invitado = enviada,
// confirmado = aceptada; si no está o no hay grupo, vacío.
func inviteState(g *grupos.Group, to bson.ObjectID) string {
	if g == nil {
		return ""
	}
	member, in := g.Member(to)
	switch {
	case !in:
		return ""
	case member.Status == grupos.StatusInvited:
		return InviteSent
	case member.Status == grupos.StatusConfirmed:
		return InviteAccepted
	}
	return ""
}

// planGroup es el grupo del plan; si no hay o falla, la tarjeta sale sin estado (no es un error
// del chat).
func (s *DirectService) planGroup(ctx context.Context, planID bson.ObjectID) *grupos.Group {
	if s.d.Groups == nil {
		return nil
	}
	g, err := s.d.Groups.FindByPlan(ctx, planID)
	if err != nil {
		if !errors.Is(err, grupos.ErrNotFound) {
			s.d.Log.WarnContext(ctx, "directos_invitacion_fallo", slog.String("plan_id", planID.Hex()))
		}
		return nil
	}
	return &g
}

// Send guarda el mensaje (201) o devuelve el ya guardado con el mismo `id_cliente` (200).
func (s *DirectService) Send(ctx context.Context, subject, rawID string, req DirectSendRequest) (DirectView, bool, error) {
	me, c, f, err := s.access(ctx, subject, rawID, true)
	if err != nil {
		return DirectView{}, false, err
	}
	text, err := CleanText(req.Text)
	if err != nil {
		return DirectView{}, false, err
	}
	if prev, found, err := s.resent(ctx, c, me, req.ClientID); found || err != nil {
		return prev, false, err
	}
	now := s.now().UTC()
	if err := s.allowed(ctx, me, text, now); err != nil {
		return DirectView{}, false, err
	}
	attachment, err := s.attachment(ctx, subject, me, req.Attachment)
	if err != nil {
		return DirectView{}, false, err
	}
	m := Message{ID: bson.NewObjectID(), ConversationID: c.ID, AuthorID: &me, Kind: KindText, Text: text, Attachment: attachment, Reactions: []Reaction{}, ClientID: req.ClientID, CreatedAt: now}
	if err := s.d.Messages.Insert(ctx, &m); errors.Is(err, ErrDuplicate) {
		prev, _, err := s.resent(ctx, c, me, req.ClientID)
		return prev, false, err
	} else if err != nil {
		return DirectView{}, false, errStore()
	}
	s.afterSend(ctx, c, f, m)
	return s.views(ctx, c, []Message{m}, me)[0], true, nil
}

// resent trae el mensaje ya guardado con ese `id_cliente` (reenvío); found = false si no hay.
func (s *DirectService) resent(ctx context.Context, c Conversation, me bson.ObjectID, clientID string) (DirectView, bool, error) {
	prev, err := s.d.Messages.FindByClientID(ctx, directThread(c.ID), clientID)
	if errors.Is(err, ErrNotFound) {
		return DirectView{}, false, nil
	}
	if err != nil {
		return DirectView{}, false, errStore()
	}
	return s.views(ctx, c, []Message{prev}, me)[0], true, nil
}

// allowed aplica el límite por hora (MongoDB) y las reglas de moderación (rechazado = 422).
func (s *DirectService) allowed(ctx context.Context, me bson.ObjectID, text string, now time.Time) error {
	sent, err := s.d.Messages.CountSince(ctx, fieldDirect, me, now.Add(-DirectSendWindow))
	if err != nil {
		return errStore()
	}
	if sent >= DirectSendLimit {
		return apperr.RateLimited(int(DirectSendWindow.Seconds()))
	}
	if s.d.Moderator != nil && s.d.Moderator.Evaluar(ctx, moderacion.Contenido{Texto: text}).Veredicto == moderacion.VeredictoRechazado {
		return invalid("texto", "contenido no permitido")
	}
	return nil
}

// attachment resuelve el adjunto y comprueba que quien envía lo vea. Lo que no ve es 422 sin
// decir si existe.
func (s *DirectService) attachment(ctx context.Context, subject string, me bson.ObjectID, req *AttachmentRequest) (*Attachment, error) {
	if req == nil {
		return nil, nil
	}
	id, err := bson.ObjectIDFromHex(req.ID)
	if err != nil {
		return nil, errAttachment()
	}
	switch req.Kind {
	case AttachPlan:
		return s.planAttachment(ctx, subject, id)
	case AttachPlace:
		b, ok, err := s.d.Places.FindVisible(ctx, id)
		if err != nil {
			return nil, errStore()
		}
		if !ok {
			return nil, errAttachment()
		}
		return &Attachment{Kind: AttachPlace, ID: id, Name: b.Name}, nil
	case AttachMoment:
		if _, err := s.d.Moments.Visible(ctx, me, req.ID); err != nil {
			return nil, hidden(err)
		}
		return &Attachment{Kind: AttachMoment, ID: id}, nil
	}
	return nil, errAttachment()
}

// planAttachment es el plan si quien envía lo ve (nv-83/nv-84), con su nombre.
func (s *DirectService) planAttachment(ctx context.Context, subject string, id bson.ObjectID) (*Attachment, error) {
	if _, err := s.d.Plans.Visible(ctx, subject, id.Hex()); err != nil {
		return nil, hidden(err)
	}
	p, err := s.d.PlanReader.FindActive(ctx, id)
	if errors.Is(err, planes.ErrNotFound) {
		return nil, errAttachment()
	}
	if err != nil {
		return nil, errStore()
	}
	return &Attachment{Kind: AttachPlan, ID: id, Name: p.Nombre}, nil
}

// hidden convierte el 404 del módulo dueño en 422 del adjunto; lo demás sigue igual.
func hidden(err error) error {
	var appErr *apperr.Error
	if errors.As(err, &appErr) && appErr.Kind == apperr.KindNotFound {
		return errAttachment()
	}
	return err
}

// afterSend actualiza la vista previa y avisa `mensaje_directo` a la otra persona, uno por
// conversación cada GroupEvery, salvo que haya silenciado a quien escribe (nv-70). El aviso no
// lleva texto ni autor. Si algo falla, el mensaje ya quedó: solo se registra.
func (s *DirectService) afterSend(ctx context.Context, c Conversation, f amistades.Friendship, m Message) {
	last := DirectLast{MessageID: m.ID, Text: Preview(m.Text), AuthorID: m.AuthorID, At: m.CreatedAt}
	if err := s.d.Conversations.SetLast(ctx, c.ID, last); err != nil {
		s.d.Log.WarnContext(ctx, "directos_ultimo_mensaje_fallo", slog.String("conversacion_id", c.ID.Hex()))
	}
	to := c.Other(*m.AuthorID)
	if s.d.Notices == nil || f.MutedByUser(to) {
		return
	}
	s.d.Notices.Emit(ctx, to, notificaciones.KindDirectMessage, "conversacion:"+c.ID.Hex(), map[string]string{"conversacion_id": c.ID.Hex()})
}

// React pone, cambia o quita (emoji null) la reacción; pide poder escribir.
func (s *DirectService) React(ctx context.Context, subject, rawID, rawMessageID string, req ReactRequest) (DirectView, error) {
	me, c, _, err := s.access(ctx, subject, rawID, true)
	if err != nil {
		return DirectView{}, err
	}
	emoji := ""
	if req.Emoji != nil {
		if !IsEmoji(*req.Emoji) {
			return DirectView{}, invalid("emoji", "emoji no permitido")
		}
		emoji = *req.Emoji
	}
	id, err := bson.ObjectIDFromHex(rawMessageID)
	if err != nil {
		return DirectView{}, errMessageNotFound()
	}
	ok, err := s.d.Messages.SetReaction(ctx, directThread(c.ID), id, me, emoji)
	if err != nil {
		return DirectView{}, errStore()
	}
	if !ok {
		return DirectView{}, errMessageNotFound()
	}
	m, err := s.d.Messages.FindIn(ctx, directThread(c.ID), id)
	if err != nil {
		return DirectView{}, errStore()
	}
	return s.views(ctx, c, []Message{m}, me)[0], nil
}

// Delete vacía un mensaje propio (204, también si ya estaba borrado); el de la otra persona es
// 404. Basta participar sin bloqueo: se puede borrar lo propio aunque ya no sean amigos.
func (s *DirectService) Delete(ctx context.Context, subject, rawID, rawMessageID string) error {
	me, c, _, err := s.access(ctx, subject, rawID, false)
	if err != nil {
		return err
	}
	id, err := bson.ObjectIDFromHex(rawMessageID)
	if err != nil {
		return errMessageNotFound()
	}
	ok, err := s.d.Messages.SoftDelete(ctx, directThread(c.ID), id, me, s.now().UTC())
	if err != nil {
		return errStore()
	}
	if !ok {
		return errMessageNotFound()
	}
	if err := s.d.Conversations.ClearLast(ctx, c.ID, id); err != nil {
		s.d.Log.WarnContext(ctx, "directos_ultimo_mensaje_fallo", slog.String("conversacion_id", c.ID.Hex()))
	}
	return nil
}

// MarkRead marca leída la conversación hasta su último mensaje (204).
func (s *DirectService) MarkRead(ctx context.Context, subject, rawID string) error {
	me, c, _, err := s.access(ctx, subject, rawID, false)
	if err != nil {
		return err
	}
	newest, err := s.d.Messages.Newest(ctx, c.ID)
	if err != nil {
		return errStore()
	}
	if newest.IsZero() {
		return nil
	}
	if err := s.d.Conversations.MarkRead(ctx, c.ID, me, newest); err != nil {
		return errStore()
	}
	return nil
}

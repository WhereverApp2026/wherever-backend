package planes

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// Ventana de «Planes de tus amigos» (nv-84): de hoy a 30 días o creados en los últimos 7.
const (
	friendsAheadDays   = 30
	friendsCreatedDays = 7
	// friendsGoingShown son los amigos apuntados que salen con nombre («Jorge y 3 más van»).
	friendsGoingShown = 3
	tabFriends        = "amigos"
)

// Friends son las amistades aceptadas de quien llama, con su marca de silencio (nv-36, nv-70);
// lo cumple amistades.Repository.
type Friends interface {
	List(ctx context.Context, me bson.ObjectID, kind amistades.Kind) ([]amistades.Friendship, error)
}

// People trae nombre, foto y privacidad de cuentas activas (lo cumple usuarios.Repository).
type People interface {
	FindPublicByIDs(ctx context.Context, ids []bson.ObjectID) ([]usuarios.User, error)
}

// Going da, por plan, los integrantes confirmados de su grupo sin el organizador: los que
// «van» (nv-69); lo cumple grupos.Repository.
type Going interface {
	GoingTo(ctx context.Context, planIDs []bson.ObjectID) (map[bson.ObjectID][]bson.ObjectID, error)
}

// Likes dice a qué planes les dio me gusta quien llama (nv-83); lo cumple reacciones.Repository.
type Likes interface {
	LikedPlans(ctx context.Context, me bson.ObjectID, planIDs []bson.ObjectID) (map[bson.ObjectID]bool, error)
}

type friendsDeps struct {
	friends Friends
	people  People
	going   Going
	likes   Likes
}

// WithFriendsFeed conecta «Planes de tus amigos»; sin él la ruta responde 503. Va después de
// WithExtras (usa el catálogo para el nombre de cada lugar).
func (s *Service) WithFriendsFeed(f Friends, p People, g Going, l Likes) *Service {
	s.feed = friendsDeps{friends: f, people: p, going: g, likes: l}
	return s
}

// FriendsQuery es `?cursor=&limite=` de GET /planes/amigos.
type FriendsQuery struct {
	Cursor string `query:"cursor" validate:"omitempty,max=300,cursor"`
	Limit  int    `query:"limite" validate:"min=1,max=50"`
}

// NewFriendsQuery trae 20 por página.
func NewFriendsQuery() FriendsQuery { return FriendsQuery{Limit: pagination.DefaultLimit} }

// PersonCard es la ficha corta de una persona en el feed.
type PersonCard struct {
	ID            string `json:"id"`
	Nombre        string `json:"nombre"`
	NombreUsuario string `json:"nombre_usuario"`
	FotoURL       string `json:"foto_url"`
}

// PlaceRef es el lugar de una parada: id y nombre.
type PlaceRef struct {
	ID     string `json:"id"`
	Nombre string `json:"nombre"`
}

// FriendStop es la hora y el lugar de una parada, sin notas ni precios.
type FriendStop struct {
	Hora  time.Time `json:"hora"`
	Lugar *PlaceRef `json:"lugar"`
}

// GoingView es «Jorge y 3 más van»: el total y hasta 3 amigos de quien mira.
type GoingView struct {
	Total  int          `json:"total"`
	Amigos []PersonCard `json:"amigos"`
}

// MineView es lo de quien mira: si se apuntó y si le dio me gusta.
type MineView struct {
	Apuntado bool `json:"apuntado"`
	MeGusta  bool `json:"me_gusta"`
}

// FriendPlan es un plan de una amistad: sin integrantes completos, grupo, notas ni ubicación.
type FriendPlan struct {
	ID               string       `json:"id"`
	Nombre           string       `json:"nombre"`
	Fecha            time.Time    `json:"fecha"`
	Autor            PersonCard   `json:"autor"`
	Paradas          []FriendStop `json:"paradas"`
	CostoPorPersona  int          `json:"costo_por_persona"`
	Apuntados        GoingView    `json:"apuntados"`
	MeGustaTotal     int          `json:"me_gusta_total"`
	ComentariosTotal int          `json:"comentarios_total"`
	Yo               MineView     `json:"yo"`
}

func personCard(u usuarios.User) PersonCard {
	return PersonCard{ID: u.ID.Hex(), Nombre: u.Name, NombreUsuario: u.Username, FotoURL: u.PhotoURL}
}

// FriendsWindow es el rango de nv-84 con el día de hoy en México: fecha en [desde, hasta) o
// creado desde `creados`.
type FriendsWindow struct {
	From, To, CreatedSince time.Time
}

// NewFriendsWindow calcula la ventana del feed para el instante now.
func NewFriendsWindow(now time.Time) FriendsWindow {
	local := now.In(negocios.MexicoCity())
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, local.Location())
	return FriendsWindow{From: today, To: today.AddDate(0, 0, friendsAheadDays+1), CreatedSince: now.Add(-friendsCreatedDays * 24 * time.Hour)}
}

// sharingFriends son las amistades aceptadas sin silenciar que comparten sus planes (nv-82),
// con su ficha.
func (s *Service) sharingFriends(ctx context.Context, me bson.ObjectID) (map[bson.ObjectID]usuarios.User, error) {
	fs, err := s.feed.friends.List(ctx, me, amistades.KindFriends)
	if err != nil {
		return nil, errStore()
	}
	ids := make([]bson.ObjectID, 0, len(fs))
	for _, f := range fs {
		if !f.MutedByUser(me) {
			ids = append(ids, f.Other(me))
		}
	}
	us, err := s.feed.people.FindPublicByIDs(ctx, ids)
	if err != nil {
		return nil, errStore()
	}
	out := make(map[bson.ObjectID]usuarios.User, len(us))
	for _, u := range us {
		out[u.ID] = u
	}
	return out, nil
}

// Friends pagina los planes activos `amigos` o `publico` de las amistades aceptadas de quien
// llama, de la fecha más cercana a la más lejana. Quedan fuera los silenciados, los bloqueos
// (no son amistad aceptada) y quien no comparte sus planes u oculta su actividad (nv-82).
func (s *Service) Friends(ctx context.Context, subject string, q FriendsQuery) (pagination.PageResult[FriendPlan], error) {
	var vacio pagination.PageResult[FriendPlan]
	if s.feed.friends == nil || s.extras.places == nil {
		return vacio, apperr.Unavailable("servicio no disponible")
	}
	me, err := owner(subject)
	if err != nil {
		return vacio, err
	}
	scope := s.cursor.Scope("planes_amigos", me.Hex())
	after, err := position(s.cursor, scope, tabFriends, q.Cursor)
	if err != nil {
		return vacio, err
	}
	friends, err := s.sharingFriends(ctx, me)
	if err != nil {
		return vacio, err
	}
	owners := sharingPlans(friends)
	if len(owners) == 0 {
		return pagination.NewPageResult([]FriendPlan{}, "", q.Limit), nil
	}
	rows, err := s.repo.FriendsFeed(ctx, FriendsFeedQuery{Owners: owners, Window: NewFriendsWindow(s.now().UTC()), After: after, Limit: q.Limit})
	if err != nil {
		return vacio, errStore()
	}
	rows, next, err := s.friendsPage(rows, q.Limit, scope)
	if err != nil {
		return vacio, err
	}
	items, err := s.friendPlans(ctx, me, rows, friends)
	if err != nil {
		return vacio, err
	}
	return pagination.NewPageResult(items, next, q.Limit), nil
}

// sharingPlans son las amistades que comparten sus planes y no ocultan su actividad (nv-82).
func sharingPlans(friends map[bson.ObjectID]usuarios.User) []bson.ObjectID {
	out := make([]bson.ObjectID, 0, len(friends))
	for id, u := range friends {
		if u.Privacy.Social.SharesPlans() {
			out = append(out, id)
		}
	}
	return out
}

// friendsPage recorta a limit y firma el cursor del último si hay otra página.
func (s *Service) friendsPage(rows []Plan, limit int, scope string) ([]Plan, string, error) {
	if len(rows) <= limit {
		return rows, "", nil
	}
	rows = rows[:limit]
	last := rows[len(rows)-1]
	next, err := s.cursor.Encode(pagination.Cursor{ID: last.ID.Hex(), Sort: tabFriends + ":" + last.Fecha.UTC().Format(time.RFC3339Nano), Scope: scope})
	if err != nil {
		return nil, "", errStore()
	}
	return rows, next, nil
}

// friendPlans arma la página: lugares, apuntados (solo amigos de quien mira y con actividad
// visible salen con nombre) y lo de quien mira.
func (s *Service) friendPlans(ctx context.Context, me bson.ObjectID, rows []Plan, friends map[bson.ObjectID]usuarios.User) ([]FriendPlan, error) {
	planIDs := make([]bson.ObjectID, 0, len(rows))
	placeIDs := []bson.ObjectID{}
	for _, p := range rows {
		planIDs = append(planIDs, p.ID)
		for _, st := range p.Paradas {
			placeIDs = append(placeIDs, st.NegocioID)
		}
	}
	places, err := s.extras.places.FindVisibleSummaries(ctx, placeIDs)
	if err != nil {
		return nil, errStore()
	}
	names := make(map[bson.ObjectID]string, len(places))
	for _, b := range places {
		names[b.ID] = b.Name
	}
	going, err := s.feed.going.GoingTo(ctx, planIDs)
	if err != nil {
		return nil, errStore()
	}
	liked, err := s.feed.likes.LikedPlans(ctx, me, planIDs)
	if err != nil {
		return nil, errStore()
	}
	out := make([]FriendPlan, 0, len(rows))
	for _, p := range rows {
		out = append(out, NewFriendPlan(p, friends[p.OwnerID], names, going[p.ID], friends, me, liked[p.ID]))
	}
	return out, nil
}

// NewFriendPlan arma un plan del feed. going son quienes van (sin el dueño); con nombre salen
// hasta 3 que sean amigos de quien mira y no oculten su actividad (nv-82).
func NewFriendPlan(p Plan, autor usuarios.User, names map[bson.ObjectID]string, going []bson.ObjectID,
	friends map[bson.ObjectID]usuarios.User, me bson.ObjectID, liked bool) FriendPlan {
	out := FriendPlan{
		ID: p.ID.Hex(), Nombre: p.Nombre, Fecha: p.Fecha.UTC(), Autor: personCard(autor), Paradas: make([]FriendStop, 0, len(p.Paradas)),
		CostoPorPersona: p.CostoPorPersona, MeGustaTotal: p.MeGustaTotal, ComentariosTotal: p.ComentariosTotal,
		Apuntados: GoingView{Total: len(going), Amigos: []PersonCard{}}, Yo: MineView{MeGusta: liked},
	}
	for _, st := range p.Paradas {
		stop := FriendStop{Hora: st.Llega.UTC()}
		if name, ok := names[st.NegocioID]; ok {
			stop.Lugar = &PlaceRef{ID: st.NegocioID.Hex(), Nombre: name}
		}
		out.Paradas = append(out.Paradas, stop)
	}
	for _, id := range going {
		if id == me {
			out.Yo.Apuntado = true
			continue
		}
		u, ok := friends[id]
		if ok && !u.Privacy.Social.HidesActivity() && len(out.Apuntados.Amigos) < friendsGoingShown {
			out.Apuntados.Amigos = append(out.Apuntados.Amigos, personCard(u))
		}
	}
	return out
}

// friendsRoutes registra GET /planes/amigos (antes de /planes/:id).
func (h *Handler) friendsRoutes(g fiber.Router) {
	chain := []fiber.Handler{}
	if h.active != nil {
		chain = append(chain, h.active)
	}
	g.Get("/amigos", append(chain, h.friends)...)
}

func (h *Handler) friends(c *fiber.Ctx) error {
	q := NewFriendsQuery()
	if err := c.QueryParser(&q); err != nil {
		return apperr.BadRequest("parámetros inválidos")
	}
	if err := h.validator.Struct(q); err != nil {
		return err
	}
	out, err := h.svc.Friends(c.UserContext(), subject(c), q)
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.JSON(out)
}

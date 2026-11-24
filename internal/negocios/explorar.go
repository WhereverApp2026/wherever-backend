package negocios

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// Límites de los filtros de Explorar (nv-85).
const (
	MaxCategories = 6
	// FriendsWindow es cuánto atrás cuenta un check-in de un amigo para «De amigos».
	FriendsWindow = 180 * 24 * time.Hour
	// MaxFriendInitials es cuántos amigos muestra la tarjeta.
	MaxFriendInitials = 3
	minPriceLevel     = 1
	maxPriceLevel     = 4
)

// Niveles de `afluencia_ahora` (curva 0–100 de nv-66 en la hora actual).
const (
	CrowdLow    = "poco"
	CrowdMedium = "moderado"
	CrowdPeak   = "pico"
	crowdLowMax = 33
	crowdMedMax = 66
)

// ExploreFilters son los filtros de Explorar que comparten GET /lugares, /lugares/conteo y
// /lugares/mapa (nv-21, nv-66, nv-85). `categorias` y `precio` son listas separadas por comas.
type ExploreFilters struct {
	Category   string `query:"categoria" validate:"omitempty,max=40"`
	Categories string `query:"categorias" validate:"omitempty,max=300"`
	MaxPrice   int    `query:"precio_max" validate:"omitempty,min=1,max=4"`
	Prices     string `query:"precio" validate:"omitempty,max=20"`
	OpenNow    bool   `query:"abierto_ahora"`
	Seal       bool   `query:"con_sello"`
	Coupon     bool   `query:"con_cupon"`
	Friends    bool   `query:"de_amigos"`
	// Experiencia filtra por tipo de experiencia (nv-90).
	Experience string `query:"experiencia" validate:"omitempty,oneof=desayuno comida cena paseo romantico familia amigos"`
}

// CardCoupon es el mejor cupón vigente del lugar en la tarjeta.
type CardCoupon struct {
	ID    string `json:"id"`
	Title string `json:"titulo"`
}

// FriendInitials es un amigo en la tarjeta: solo su id y sus iniciales, nunca el nombre.
type FriendInitials struct {
	ID       string `json:"id"`
	Initials string `json:"iniciales"`
}

// FriendsWent es «amigos que fueron»: cuántos y hasta 3, del check-in más reciente al más viejo.
type FriendsWent struct {
	Total   int              `json:"total"`
	Friends []FriendInitials `json:"iniciales"`
}

// CouponOffer es un cupón vigente de un lugar, con lo necesario para elegir el mejor.
type CouponOffer struct {
	Place       bson.ObjectID
	ID          bson.ObjectID
	Title       string
	Percent     int64
	AmountCents int64
	TwoForOne   bool
	Ends        time.Time
}

// FriendVisit es el check-in válido más reciente de una persona en un lugar.
type FriendVisit struct {
	Place bson.ObjectID `bson:"negocio_id"`
	User  bson.ObjectID `bson:"usuario_id"`
	Last  time.Time     `bson:"ultimo"`
}

// FriendIDs da las amistades aceptadas de una persona; lo cumple amistades.Repository.
type FriendIDs interface {
	FriendIDs(ctx context.Context, me bson.ObjectID) ([]bson.ObjectID, error)
}

// Visits lee los check-ins válidos; lo cumple resenas.Repository.
type Visits interface {
	VisitedPlaces(ctx context.Context, users []bson.ObjectID, since time.Time) ([]bson.ObjectID, error)
	VisitsAt(ctx context.Context, users, places []bson.ObjectID, since time.Time) ([]FriendVisit, error)
}

// Coupons lee los cupones vigentes de varios lugares; lo cumple cupones.Repository.
type Coupons interface {
	OffersFor(ctx context.Context, places []bson.ObjectID, now time.Time) ([]CouponOffer, error)
}

type exploreDeps struct {
	friends FriendIDs
	visits  Visits
	coupons Coupons
}

// WithExplore conecta lo social y los cupones de Explorar (nv-85). Sin él, `de_amigos` = 503
// y la tarjeta va sin cupón ni amigos.
func (s *Service) WithExplore(f FriendIDs, v Visits, c Coupons) *Service {
	s.explore = &exploreDeps{friends: f, visits: v, coupons: c}
	return s
}

// viewer es quien consulta y, si hizo falta, sus amigos que comparten su actividad.
type viewer struct {
	ID      bson.ObjectID
	Friends map[bson.ObjectID]usuarios.User
	loaded  bool
	// active = su cuenta está activa; en eliminación pendiente no ve a sus amigos (nv-05).
	active bool
}

// loadViewer lee al que consulta. Sin sesión = nil; `de_amigos` sin sesión = 401. Los amigos
// se leen si el filtro los pide o si la tarjeta los muestra (card).
func (s *Service) loadViewer(ctx context.Context, subject string, f ExploreFilters, card bool) (*viewer, error) {
	v, err := viewerOfSubject(subject, f.Friends)
	if v == nil || err != nil {
		return nil, err
	}
	switch {
	case s.explore == nil && f.Friends:
		return nil, apperr.Unavailable("servicio no disponible")
	case s.explore == nil, !f.Friends && !card:
		return v, nil
	}
	if err := s.loadFriends(ctx, v); err != nil {
		return nil, apperr.Unavailable("servicio no disponible")
	}
	if f.Friends && !v.active {
		return nil, apperr.Forbidden("la cuenta no está activa")
	}
	return v, nil
}

// viewerOfSubject lee el sujeto del token: "" = sin sesión (401 si pide amigos).
func viewerOfSubject(subject string, friends bool) (*viewer, error) {
	if subject == "" {
		if friends {
			return nil, apperr.Unauthorized("de_amigos requiere sesión")
		}
		return nil, nil
	}
	id, err := bson.ObjectIDFromHex(subject)
	if err != nil {
		return nil, apperr.Unauthorized("credenciales inválidas")
	}
	return &viewer{ID: id}, nil
}

// loadFriends son las amistades aceptadas activas que no ocultan su actividad (nv-82). Quien
// consulta va en la misma lectura de cuentas: solo vuelve si su cuenta está activa.
func (s *Service) loadFriends(ctx context.Context, v *viewer) error {
	ids, err := s.explore.friends.FriendIDs(ctx, v.ID)
	if err != nil {
		return err
	}
	people, err := s.users.FindPublicByIDs(ctx, append(ids, v.ID))
	if err != nil {
		return err
	}
	v.Friends = map[bson.ObjectID]usuarios.User{}
	v.loaded = true
	for _, u := range people {
		if u.ID == v.ID {
			v.active = true
		}
	}
	if !v.active {
		return nil
	}
	for _, u := range people {
		if u.ID != v.ID && !u.Privacy.Social.HidesActivity() {
			v.Friends[u.ID] = u
		}
	}
	return nil
}

func (v *viewer) friendIDs() []bson.ObjectID {
	out := make([]bson.ObjectID, 0, len(v.Friends))
	for id := range v.Friends {
		out = append(out, id)
	}
	slices.SortFunc(out, func(a, b bson.ObjectID) int { return strings.Compare(a.Hex(), b.Hex()) })
	return out
}

// categoriesOf une `categoria` y `categorias` sin repetir; hasta 6 y todas del catálogo.
func categoriesOf(f ExploreFilters) ([]string, error) {
	var out []string
	add := func(raw string) {
		id := strings.TrimSpace(raw)
		if id != "" && !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	add(f.Category)
	if f.Categories != "" {
		for _, part := range strings.Split(f.Categories, ",") {
			add(part)
		}
	}
	if len(out) > MaxCategories {
		return nil, apperr.Validation("demasiadas categorías",
			apperr.FieldError{Field: "categorias", Message: "hasta 6 categorías"})
	}
	for _, id := range out {
		if !knownCategory(id) {
			return nil, unknownCategory()
		}
	}
	return out, nil
}

// pricesOf lee `precio` (niveles 1–4 separados por comas, sin repetir).
func pricesOf(raw string) ([]int, error) {
	if raw == "" {
		return nil, nil
	}
	invalid := apperr.Validation("precio inválido",
		apperr.FieldError{Field: "precio", Message: "niveles de 1 a 4 separados por comas"})
	var out []int
	for _, part := range strings.Split(raw, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || n < minPriceLevel || n > maxPriceLevel {
			return nil, invalid
		}
		if !slices.Contains(out, n) {
			out = append(out, n)
		}
	}
	slices.Sort(out)
	return out, nil
}

// matchConditions traduce los filtros a condiciones de MongoDB; todos van antes del límite,
// así lista, conteo y mapa cuentan lo mismo.
func (s *Service) matchConditions(ctx context.Context, f ExploreFilters, v *viewer) (bson.A, error) {
	var match bson.A
	categories, err := categoriesOf(f)
	if err != nil {
		return nil, err
	}
	switch len(categories) {
	case 0:
	case 1:
		match = append(match, bson.D{{Key: "categoria", Value: categories[0]}})
	default:
		match = append(match, bson.D{{Key: "categoria", Value: bson.D{{Key: "$in", Value: categories}}}})
	}
	prices, err := pricesOf(f.Prices)
	if err != nil {
		return nil, err
	}
	if len(prices) > 0 {
		match = append(match, bson.D{{Key: "nivel_precio", Value: bson.D{{Key: "$in", Value: prices}}}})
	}
	if f.OpenNow {
		match = append(match, bson.D{openAtFilter(s.now())})
	}
	match = append(match, flagConditions(f)...)
	if f.Friends {
		places, err := s.friendPlaces(ctx, v)
		if err != nil {
			return nil, err
		}
		match = append(match, bson.D{{Key: "_id", Value: bson.D{{Key: "$in", Value: places}}}})
	}
	return match, nil
}

// flagConditions son precio máximo, experiencia, Sello y cupón.
func flagConditions(f ExploreFilters) bson.A {
	var match bson.A
	if f.MaxPrice > 0 {
		match = append(match, bson.D{{Key: "nivel_precio", Value: bson.D{{Key: "$lte", Value: f.MaxPrice}}}})
	}
	if f.Experience != "" {
		match = append(match, bson.D{{Key: "experiencias", Value: f.Experience}})
	}
	if f.Seal {
		match = append(match, bson.D{{Key: "sello_excelencia", Value: true}})
	}
	if f.Coupon {
		// `cupones_activos` lo recalcula cupones al cambiar un cupón y su trabajo cada 5 min.
		match = append(match, bson.D{{Key: "cupones_activos", Value: bson.D{{Key: "$gt", Value: 0}}}})
	}
	return match
}

// friendPlaces son los lugares con check-in válido de un amigo en los últimos 180 días.
func (s *Service) friendPlaces(ctx context.Context, v *viewer) ([]bson.ObjectID, error) {
	if v == nil || !v.loaded {
		return nil, apperr.Unauthorized("de_amigos requiere sesión")
	}
	if len(v.Friends) == 0 {
		return []bson.ObjectID{}, nil
	}
	places, err := s.explore.visits.VisitedPlaces(ctx, v.friendIDs(), s.now().Add(-FriendsWindow))
	if err != nil {
		return nil, apperr.Unavailable("servicio no disponible")
	}
	if places == nil {
		places = []bson.ObjectID{}
	}
	return places, nil
}

// filtersScope son los filtros en el ámbito del cursor firmado; con `de_amigos` también quien
// consulta, porque el resultado depende de sus amigos.
func filtersScope(f ExploreFilters, v *viewer) []string {
	parts := []string{f.Category, f.Categories, strconv.Itoa(f.MaxPrice), f.Prices,
		strconv.FormatBool(f.OpenNow), strconv.FormatBool(f.Seal), strconv.FormatBool(f.Coupon), strconv.FormatBool(f.Friends), f.Experience}
	if f.Friends && v != nil {
		parts = append(parts, v.ID.Hex())
	}
	return parts
}

// CrowdLevel es la afluencia de la hora actual en México: `poco` (0–33), `moderado` (34–66) o
// `pico` (67–100). Sin curva o con el día en ceros = nil.
func CrowdLevel(matrix [][]int, now time.Time) *string {
	day := int(now.In(MexicoCity()).Weekday())
	t := TrafficOf(matrix, day, now)
	if !t.Enough || t.CurrentHour == nil {
		return nil
	}
	level := crowdOf(t.Hours[*t.CurrentHour])
	return &level
}

// savingCents estima el ahorro del cupón en centavos con el precio promedio del lugar (pesos):
// porcentaje sobre el promedio, monto tal cual, 2×1 = un promedio, regalo = 0.
func (o CouponOffer) savingCents(averagePrice int) int64 {
	switch {
	case o.Percent > 0:
		return int64(averagePrice) * o.Percent
	case o.AmountCents > 0:
		return o.AmountCents
	case o.TwoForOne:
		return int64(averagePrice) * 100
	default:
		return 0
	}
}

// BestCoupon elige el de mayor ahorro; empate = el que vence antes y luego el id menor.
func BestCoupon(offers []CouponOffer, averagePrice int) (CouponOffer, bool) {
	if len(offers) == 0 {
		return CouponOffer{}, false
	}
	best := slices.MinFunc(offers, func(a, b CouponOffer) int {
		if sa, sb := a.savingCents(averagePrice), b.savingCents(averagePrice); sa != sb {
			if sa > sb {
				return -1
			}
			return 1
		}
		if c := a.Ends.Compare(b.Ends); c != 0 {
			return c
		}
		return strings.Compare(a.ID.Hex(), b.ID.Hex())
	})
	return best, true
}

// Initials son la primera letra del primer y del último nombre, en mayúsculas; sin nombre, la
// del nombre de usuario.
func Initials(name, username string) string {
	words := strings.Fields(name)
	if len(words) == 0 {
		words = strings.Fields(username)
	}
	if len(words) == 0 {
		return ""
	}
	first := func(w string) string {
		for _, r := range w {
			return string(unicode.ToUpper(r))
		}
		return ""
	}
	out := first(words[0])
	if len(words) > 1 {
		out += first(words[len(words)-1])
	}
	return out
}

// decorate agrega a la página el mejor cupón y, con sesión, los amigos que fueron.
func (s *Service) decorate(ctx context.Context, rows []placeRow, items []ExploreItem, v *viewer) error {
	if s.explore == nil || len(rows) == 0 {
		return nil
	}
	if err := s.decorateCoupons(ctx, rows, items); err != nil {
		return err
	}
	if v == nil {
		return nil
	}
	return s.decorateFriends(ctx, rows, items, v)
}

func (s *Service) decorateCoupons(ctx context.Context, rows []placeRow, items []ExploreItem) error {
	var ids []bson.ObjectID
	for _, r := range rows {
		if r.Coupons > 0 {
			ids = append(ids, r.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	offers, err := s.explore.coupons.OffersFor(ctx, ids, s.now())
	if err != nil {
		return err
	}
	byPlace := map[bson.ObjectID][]CouponOffer{}
	for _, o := range offers {
		byPlace[o.Place] = append(byPlace[o.Place], o)
	}
	for i, r := range rows {
		if best, ok := BestCoupon(byPlace[r.ID], r.AveragePrice); ok {
			items[i].Coupon = &CardCoupon{ID: best.ID.Hex(), Title: best.Title}
		}
	}
	return nil
}

func (s *Service) decorateFriends(ctx context.Context, rows []placeRow, items []ExploreItem, v *viewer) error {
	for i := range items {
		items[i].FriendsWent = &FriendsWent{Friends: []FriendInitials{}}
	}
	if !v.loaded || len(v.Friends) == 0 {
		return nil
	}
	places := make([]bson.ObjectID, 0, len(rows))
	for _, r := range rows {
		places = append(places, r.ID)
	}
	visits, err := s.explore.visits.VisitsAt(ctx, v.friendIDs(), places, s.now().Add(-FriendsWindow))
	if err != nil {
		return err
	}
	addFriends(rows, items, v, visits)
	return nil
}

// addFriends suma las visitas a cada tarjeta, de la más reciente a la más vieja.
func addFriends(rows []placeRow, items []ExploreItem, v *viewer, visits []FriendVisit) {
	slices.SortFunc(visits, func(a, b FriendVisit) int {
		if c := b.Last.Compare(a.Last); c != 0 {
			return c
		}
		return strings.Compare(a.User.Hex(), b.User.Hex())
	})
	index := make(map[bson.ObjectID]int, len(rows))
	for i, r := range rows {
		index[r.ID] = i
	}
	for _, visit := range visits {
		i, okPlace := index[visit.Place]
		friend, okFriend := v.Friends[visit.User]
		if !okPlace || !okFriend {
			continue
		}
		went := items[i].FriendsWent
		went.Total++
		if len(went.Friends) < MaxFriendInitials {
			went.Friends = append(went.Friends, FriendInitials{ID: friend.ID.Hex(), Initials: Initials(friend.Name, friend.Username)})
		}
	}
}

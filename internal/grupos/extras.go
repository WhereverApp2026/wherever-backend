package grupos

import (
	"context"
	"errors"
	"math"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// Constantes de nv-69: sin variables de entorno.
const (
	nameMax           = 40
	meetingNameMax    = 60
	locationTTL       = 6 * time.Hour
	locationDecimals  = 1000.0 // 3 decimales ≈ 110 m
	gridDeg           = 0.01   // ≈ 1 km
	suggestRadiusM    = 1500.0
	suggestTop        = 3
	suggestCandidates = 40
	minSharers        = 2
	carPesosPerKm     = 4
	transitFare       = 15
)

// Balances dice si La Vaca (nv-58) tiene saldos sin liquidar en el grupo.
type Balances interface {
	HasPending(ctx context.Context, groupID bson.ObjectID) (bool, error)
}

// MeetingPlaces es lo que el punto de encuentro lee de negocios.
type MeetingPlaces interface {
	Candidates(ctx context.Context, q negocios.CandidateQuery) ([]negocios.Candidate, error)
	FindVisible(ctx context.Context, id bson.ObjectID) (negocios.Business, bool, error)
}

// Mobilities trae la `movilidad_default` de cada cuenta (nv-05).
type Mobilities interface {
	Mobilities(ctx context.Context, ids []bson.ObjectID) (map[bson.ObjectID]string, error)
}

type extraDeps struct {
	balances Balances
	places   MeetingPlaces
	mobility Mobilities
	speeds   planes.Velocidades
}

// WithExtras conecta La Vaca, negocios y las preferencias de movilidad (nv-69).
func (s *Service) WithExtras(b Balances, p MeetingPlaces, m Mobilities) *Service {
	s.extras = extraDeps{balances: b, places: p, mobility: m, speeds: planes.DefaultVelocidades()}
	return s
}

func errStale() error { return apperr.Conflict("el grupo cambió; vuelve a cargarlo") }

func versionError(err error) error {
	switch {
	case errors.Is(err, ErrStale):
		return errStale()
	case errors.Is(err, ErrNotFound):
		return errGroupNotFound()
	default:
		return errStore()
	}
}

// organizerGroup carga el grupo y exige que quien llama sea el organizador (integrante = 403).
func (s *Service) organizerGroup(ctx context.Context, subject, rawID string) (bson.ObjectID, Group, error) {
	me, g, err := s.load(ctx, subject, rawID)
	if err == nil && g.OrganizerID != me {
		err = errNotOrganizer()
	}
	return me, g, err
}

// Rename cambia el nombre del grupo con If-Match.
func (s *Service) Rename(ctx context.Context, subject, rawID string, version int, req RenameRequest) (View, error) {
	me, g, err := s.organizerGroup(ctx, subject, rawID)
	if err != nil {
		return View{}, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" || utf8.RuneCountInString(name) > nameMax {
		return View{}, invalid("nombre", "el nombre debe tener de 1 a 40 caracteres")
	}
	if err := s.repo.Rename(ctx, g.ID, me, version, name, s.now().UTC()); err != nil {
		return View{}, versionError(err)
	}
	return s.view(ctx, g.ID, me)
}

// Dissolve disuelve el grupo con If-Match. Con saldos de La Vaca sin liquidar = 409.
func (s *Service) Dissolve(ctx context.Context, subject, rawID string, version int) error {
	me, g, err := s.organizerGroup(ctx, subject, rawID)
	if err != nil {
		return err
	}
	if g.CurrentVersion() != version {
		return errStale()
	}
	if s.extras.balances != nil {
		pending, err := s.extras.balances.HasPending(ctx, g.ID)
		if err != nil {
			return errStore()
		}
		if pending {
			return apperr.Conflict("saldos_pendientes: liquida La Vaca antes de disolver el grupo")
		}
	}
	if err := s.repo.Dissolve(ctx, g.ID, me, version, s.now().UTC()); err != nil {
		return versionError(err)
	}
	return nil
}

// planDay es el día del plan en la hora de México: [inicio, fin).
func planDay(fecha time.Time) (time.Time, time.Time) {
	loc := negocios.MexicoCity()
	f := fecha.In(loc)
	start := time.Date(f.Year(), f.Month(), f.Day(), 0, 0, 0, 0, loc)
	return start, start.AddDate(0, 0, 1)
}

// parseTime lee una hora RFC 3339; mal formada = 422 en ese campo.
func parseTime(field, raw string) (time.Time, error) {
	at, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, invalid(field, "usa una fecha y hora RFC 3339")
	}
	return at.UTC(), nil
}

// SetMeetingPoint fija el punto de encuentro; cualquier integrante confirmado. El último en
// fijarlo gana (sin If-Match).
func (s *Service) SetMeetingPoint(ctx context.Context, subject, rawID string, req MeetingPointRequest) (View, error) {
	me, g, err := s.load(ctx, subject, rawID)
	if err != nil {
		return View{}, err
	}
	if mine, _ := g.Member(me); mine.Status != StatusConfirmed {
		return View{}, apperr.Forbidden("solo los integrantes confirmados fijan el punto de encuentro")
	}
	mp, err := s.meetingPoint(ctx, g, me, req)
	if err != nil {
		return View{}, err
	}
	if err := s.repo.SetMeetingPoint(ctx, g.ID, mp, s.now().UTC()); err != nil {
		return View{}, versionError(err)
	}
	return s.view(ctx, g.ID, me)
}

func (s *Service) meetingPoint(ctx context.Context, g Group, me bson.ObjectID, req MeetingPointRequest) (MeetingPoint, error) {
	at, err := parseTime("hora", req.At)
	if err != nil {
		return MeetingPoint{}, err
	}
	p, err := s.plans.FindActive(ctx, g.PlanID)
	if err != nil {
		return MeetingPoint{}, errStore()
	}
	start, end := planDay(p.Fecha)
	if at.Before(s.now()) || at.Before(start) || !at.Before(end) {
		return MeetingPoint{}, invalid("hora", "la hora debe ser del día del plan y no en el pasado")
	}
	mp := MeetingPoint{At: at, ProposedBy: me}
	if req.BusinessID == "" {
		return manualPoint(mp, req)
	}
	return s.businessPoint(ctx, mp, req.BusinessID)
}

func manualPoint(mp MeetingPoint, req MeetingPointRequest) (MeetingPoint, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" || utf8.RuneCountInString(name) > meetingNameMax || req.Lat == nil || req.Lng == nil {
		return MeetingPoint{}, invalid("nombre", "el punto necesita nombre (1 a 60 caracteres), lat y lng")
	}
	mp.Name, mp.Location = name, Coords{Lat: *req.Lat, Lng: *req.Lng}
	return mp, nil
}

func (s *Service) businessPoint(ctx context.Context, mp MeetingPoint, raw string) (MeetingPoint, error) {
	bad := invalid("negocio_id", "el negocio no está disponible")
	id, err := bson.ObjectIDFromHex(raw)
	if err != nil || s.extras.places == nil {
		return MeetingPoint{}, bad
	}
	b, ok, err := s.extras.places.FindVisible(ctx, id)
	if err != nil {
		return MeetingPoint{}, errStore()
	}
	if !ok || len(b.Location.Coordinates) != 2 {
		return MeetingPoint{}, bad
	}
	mp.Name, mp.BusinessID = b.Name, &id
	mp.Location = Coords{Lat: b.Location.Coordinates[1], Lng: b.Location.Coordinates[0]}
	return mp, nil
}

// roundLocation deja la ubicación a 3 decimales (≈110 m): es lo único que se guarda.
func roundLocation(lat, lng float64) Coords {
	return Coords{Lat: math.Round(lat*locationDecimals) / locationDecimals, Lng: math.Round(lng*locationDecimals) / locationDecimals}
}

// snap ajusta un punto a la cuadrícula de 0.01° (≈1 km).
func snap(v float64) float64 { return math.Round(v/gridDeg) * gridDeg }

// shareLocation guarda o retira la ubicación de quien llama (PATCH integrantes/yo).
func (s *Service) shareLocation(ctx context.Context, g Group, me bson.ObjectID, req RespondRequest) error {
	var at *Coords
	if req.Location != nil && (req.Share == nil || *req.Share) {
		c := roundLocation(*req.Location.Lat, *req.Location.Lng)
		at = &c
	}
	if err := s.repo.ShareLocation(ctx, g.ID, me, at, s.now().UTC().Add(locationTTL)); err != nil {
		return errStore()
	}
	return nil
}

// sharers son las ubicaciones vigentes de los integrantes confirmados.
func sharers(g Group, now time.Time) []Coords {
	out := []Coords{}
	for _, m := range g.Members {
		if m.Status == StatusConfirmed && m.Visible(now) {
			out = append(out, *m.SharedLocation)
		}
	}
	return out
}

// center es el promedio de las ubicaciones, ajustado a la cuadrícula.
func center(points []Coords) Coords {
	var lat, lng float64
	for _, p := range points {
		lat += p.Lat
		lng += p.Lng
	}
	n := float64(len(points))
	return Coords{Lat: snap(lat / n), Lng: snap(lng / n)}
}

// sumDistance es la suma de distancias (km) de un lugar a todas las ubicaciones.
func sumDistance(b negocios.Business, points []Coords) float64 {
	if len(b.Location.Coordinates) != 2 {
		return math.Inf(1)
	}
	total := 0.0
	for _, p := range points {
		total += planes.Haversine(p.Lat, p.Lng, b.Location.Coordinates[1], b.Location.Coordinates[0])
	}
	return total
}

// Suggest propone 3 lugares justos para todos sin decir dónde está nadie: ni el centro ni
// distancias por persona.
func (s *Service) Suggest(ctx context.Context, subject, rawID string) (SuggestResponse, error) {
	_, g, err := s.load(ctx, subject, rawID)
	if err != nil {
		return SuggestResponse{}, err
	}
	now := s.now()
	points := sharers(g, now)
	if len(points) < minSharers {
		return SuggestResponse{}, invalid("ubicaciones", "faltan_ubicaciones")
	}
	at, err := s.meetingTime(ctx, g)
	if err != nil {
		return SuggestResponse{}, err
	}
	c := center(points)
	found, err := s.extras.places.Candidates(ctx, negocios.CandidateQuery{
		Lat: c.Lat, Lng: c.Lng, RadiusM: suggestRadiusM, Conditions: bson.A{negocios.OpenAtCondition(at)}, Limit: suggestCandidates,
	})
	if err != nil {
		return SuggestResponse{}, errStore()
	}
	sort.SliceStable(found, func(i, j int) bool {
		return sumDistance(found[i].Business, points) < sumDistance(found[j].Business, points)
	})
	out := SuggestResponse{Places: []negocios.PlaceItem{}, Participants: len(points)}
	for _, cand := range found {
		if len(out.Places) == suggestTop {
			break
		}
		out.Places = append(out.Places, negocios.CardOf(cand.Business, now))
	}
	return out, nil
}

// meetingTime es la hora del punto o, sin punto, la llegada a la primera parada.
func (s *Service) meetingTime(ctx context.Context, g Group) (time.Time, error) {
	if g.MeetingPoint != nil {
		return g.MeetingPoint.At, nil
	}
	p, err := s.plans.FindActive(ctx, g.PlanID)
	if err != nil {
		return time.Time{}, errStore()
	}
	at := p.Fecha
	for i, st := range p.Paradas {
		if i == 0 || st.Llega.Before(at) {
			at = st.Llega
		}
	}
	return at, nil
}

// Transfers dice cuánto tarda y gasta cada quien en llegar al punto: exacto en la fila propia
// y en rangos para los demás.
func (s *Service) Transfers(ctx context.Context, subject, rawID string) (TransfersResponse, error) {
	me, g, err := s.load(ctx, subject, rawID)
	if err != nil {
		return TransfersResponse{}, err
	}
	if g.MeetingPoint == nil {
		return TransfersResponse{}, apperr.NotFound("sin_punto: el grupo no tiene punto de encuentro")
	}
	ids := []bson.ObjectID{}
	for _, m := range g.Members {
		if m.Status == StatusConfirmed {
			ids = append(ids, m.UserID)
		}
	}
	people, mobility, err := s.travellers(ctx, ids)
	if err != nil {
		return TransfersResponse{}, err
	}
	now := s.now()
	out := TransfersResponse{Items: []TransferRow{}}
	for _, m := range g.Members {
		if m.Status != StatusConfirmed {
			continue
		}
		row := TransferRow{ID: m.UserID.Hex(), Mine: m.UserID == me}
		if u, ok := people[m.UserID]; ok {
			row.Name, row.PhotoURL = u.Name, u.PhotoURL
		}
		s.fillTransfer(&row, m, g.MeetingPoint.Location, mobility[m.UserID], now)
		out.Items = append(out.Items, row)
	}
	sort.SliceStable(out.Items, func(i, j int) bool { return out.Items[i].Mine && !out.Items[j].Mine })
	return out, nil
}

func (s *Service) travellers(ctx context.Context, ids []bson.ObjectID) (map[bson.ObjectID]usuarios.User, map[bson.ObjectID]string, error) {
	users, err := s.users.FindPublicByIDs(ctx, ids)
	if err != nil {
		return nil, nil, errStore()
	}
	people := make(map[bson.ObjectID]usuarios.User, len(users))
	for _, u := range users {
		people[u.ID] = u
	}
	mobility := map[bson.ObjectID]string{}
	if s.extras.mobility != nil {
		if mobility, err = s.extras.mobility.Mobilities(ctx, ids); err != nil {
			return nil, nil, errStore()
		}
	}
	return people, mobility, nil
}

func (s *Service) fillTransfer(row *TransferRow, m Member, to Coords, mobility string, now time.Time) {
	if !m.Visible(now) {
		row.NoLocation = true
		return
	}
	minutes, cost := s.trip(*m.SharedLocation, to, mobility)
	if row.Mine {
		row.Minutes, row.Cost = &minutes, &cost
		return
	}
	row.Range, row.CostRange = minutesRange(minutes), costRange(cost)
}

// trip estima minutos (con `Velocidades`) y pesos (tabla fija) de un punto a otro.
func (s *Service) trip(from, to Coords, mobility string) (int, int) {
	kmh, ok := s.extras.speeds[mobility]
	if !ok {
		mobility, kmh = planes.MobilityWalk, s.extras.speeds[planes.MobilityWalk]
	}
	km := planes.Haversine(from.Lat, from.Lng, to.Lat, to.Lng) * planes.StreetFactor
	return int(math.Ceil(km / kmh * 60)), tripCost(mobility, km)
}

// tripCost: a pie y en bici 0; auto por km; transporte público tarifa fija.
func tripCost(mobility string, km float64) int {
	switch mobility {
	case planes.MobilityCar:
		return int(math.Ceil(km * carPesosPerKm))
	case planes.MobilityTransit:
		return transitFare
	default:
		return 0
	}
}

func minutesRange(m int) string {
	switch {
	case m <= 15:
		return "hasta_15"
	case m <= 30:
		return "15_30"
	case m <= 60:
		return "30_60"
	default:
		return "mas_60"
	}
}

// costRange usa los mismos cortes que los minutos, de 50 en 50 pesos.
func costRange(pesos int) string {
	switch {
	case pesos <= 50:
		return "hasta_50"
	case pesos <= 100:
		return "50_100"
	case pesos <= 200:
		return "100_200"
	default:
		return "mas_200"
	}
}

// SetSurprise activa o apaga el modo sorpresa; solo el organizador.
func (s *Service) SetSurprise(ctx context.Context, subject, rawID string, req SurpriseRequest) (View, error) {
	me, g, err := s.organizerGroup(ctx, subject, rawID)
	if err != nil {
		return View{}, err
	}
	sp := Surprise{}
	if *req.Active {
		if sp, err = s.newSurprise(ctx, g, req); err != nil {
			return View{}, err
		}
	}
	if err := s.repo.SetSurprise(ctx, g.ID, me, sp, s.now().UTC()); err != nil {
		return View{}, versionError(err)
	}
	return s.view(ctx, g.ID, me)
}

func (s *Service) newSurprise(ctx context.Context, g Group, req SurpriseRequest) (Surprise, error) {
	target, err := bson.ObjectIDFromHex(req.For)
	m, in := g.Member(target)
	if err != nil || !in || m.Status != StatusConfirmed || target == g.OrganizerID {
		return Surprise{}, invalid("para_usuario_id", "la sorpresa es para un integrante confirmado que no sea el organizador")
	}
	at, err := parseTime("revelar_en", req.RevealAt)
	if err != nil {
		return Surprise{}, err
	}
	p, err := s.plans.FindActive(ctx, g.PlanID)
	if err != nil {
		return Surprise{}, errStore()
	}
	if _, end := planDay(p.Fecha); !at.After(s.now()) || at.After(end) {
		return Surprise{}, invalid("revelar_en", "revelar_en va entre ahora y el fin del día del plan")
	}
	return Surprise{Active: true, For: target, RevealAt: at}, nil
}

// MeToo apunta a quien llama al plan público de una amistad (nv-69): entra confirmado y, si
// el plan no tenía grupo, se crea con el dueño de organizador.
func (s *Service) MeToo(ctx context.Context, subject, rawPlanID string) (View, error) {
	me, err := self(subject)
	if err != nil {
		return View{}, err
	}
	planID, err := bson.ObjectIDFromHex(rawPlanID)
	if err != nil {
		return View{}, errPlanNotFound()
	}
	p, err := s.plans.FindActive(ctx, planID)
	if errors.Is(err, planes.ErrNotFound) {
		return View{}, errPlanNotFound()
	}
	if err != nil {
		return View{}, errStore()
	}
	if p.OwnerID == me {
		g, err := s.groupForPlan(ctx, me, planID)
		if err != nil {
			return View{}, errStore()
		}
		return s.view(ctx, g.ID, me)
	}
	if err := s.friendOfOwner(ctx, p, me); err != nil {
		return View{}, err
	}
	return s.joinPlan(ctx, p, me)
}

// joinPlan trae o crea el grupo del plan (el dueño de organizador) y deja a me confirmado.
func (s *Service) joinPlan(ctx context.Context, p planes.Plan, me bson.ObjectID) (View, error) {
	g, err := s.groupForPlan(ctx, p.OwnerID, p.ID)
	if err != nil {
		return View{}, errStore()
	}
	if err := s.repo.Join(ctx, g.ID, me, s.now().UTC()); err != nil {
		if err := s.joinError(ctx, g.ID, me, err); err != nil {
			return View{}, err
		}
	}
	return s.view(ctx, g.ID, me)
}

// friendOfOwner: sin amistad aceptada (o con bloqueo) = 404; amistad con plan privado = 403.
func (s *Service) friendOfOwner(ctx context.Context, p planes.Plan, me bson.ObjectID) error {
	f, ok, err := s.friends.FindByPair(ctx, p.OwnerID, me)
	if err != nil {
		return errStore()
	}
	if !ok || f.Status != amistades.StatusAccepted {
		return errPlanNotFound()
	}
	if !p.Public() {
		return apperr.Forbidden("el plan es privado")
	}
	return nil
}

// LeavePlan es salir del grupo del plan (DELETE /planes/{id}/me-apunto). El organizador no
// puede (409): que disuelva el grupo.
func (s *Service) LeavePlan(ctx context.Context, subject, rawPlanID string) error {
	me, err := self(subject)
	if err != nil {
		return err
	}
	planID, err := bson.ObjectIDFromHex(rawPlanID)
	if err != nil {
		return errPlanNotFound()
	}
	g, err := s.repo.FindByPlan(ctx, planID)
	if errors.Is(err, ErrNotFound) {
		return errPlanNotFound()
	}
	if err != nil {
		return errStore()
	}
	if g.OrganizerID == me {
		return apperr.Conflict("el organizador no puede salir; disuelve el grupo")
	}
	return s.Leave(ctx, subject, g.ID.Hex())
}

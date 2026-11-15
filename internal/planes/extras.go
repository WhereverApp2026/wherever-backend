package planes

import (
	"context"
	"math"
	"regexp"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/text"
)

// Places es lo que los extras leen del catálogo (lo cumple negocios.Repository).
type Places interface {
	Candidates(ctx context.Context, q negocios.CandidateQuery) ([]negocios.Candidate, error)
	FindCards(ctx context.Context, ids []bson.ObjectID, now time.Time) ([]negocios.PlaceItem, error)
	FindDetails(ctx context.Context, ids []bson.ObjectID, now time.Time) ([]negocios.PlaceDetail, error)
	FindVisibleSummaries(ctx context.Context, ids []bson.ObjectID) ([]negocios.Business, error)
}

// Blocks dice si dos personas tienen un bloqueo (lo cumple amistades.Repository).
type Blocks interface {
	Blocked(ctx context.Context, a, b bson.ObjectID) (bool, error)
}

// Reservas trae las reservas propias de un plan para el paquete (lo cumple reservas).
type Reservas interface {
	DelPlan(ctx context.Context, usuario, plan bson.ObjectID) ([]ReservaPaquete, error)
}

type extraDeps struct {
	places   Places
	blocks   Blocks
	reservas Reservas
}

// WithExtras activa los extras de nv-68: alternativas, cambiar parada, populares, paquete y
// planes públicos. reservas puede ser nil.
func (s *Service) WithExtras(places Places, blocks Blocks, reservas Reservas) *Service {
	s.extras = extraDeps{places: places, blocks: blocks, reservas: reservas}
	return s
}

// Constantes de nv-68.
const (
	altRadiusM    = 3000
	altCandidates = 20
	altLimit      = 3
)

// publicPlan trae un plan `publico` ajeno, activo y sin bloqueo entre su dueño y quien lo
// pide; si no, 404 como si no existiera.
func (s *Service) publicPlan(ctx context.Context, subject, rawID string) (Plan, error) {
	reader, id, err := ids(subject, rawID)
	if err != nil {
		return Plan{}, err
	}
	p, err := s.repo.FindActive(ctx, id)
	if err != nil {
		return Plan{}, storeError(err)
	}
	if !p.Public() || s.extras.blocks == nil {
		return Plan{}, errPlanNotFound()
	}
	blocked, err := s.extras.blocks.Blocked(ctx, reader, p.OwnerID)
	if err != nil {
		return Plan{}, errStore()
	}
	if blocked {
		return Plan{}, errPlanNotFound()
	}
	return p, nil
}

// clonePublic copia un plan público ajeno: la copia es privada, `origen: clonado`, con
// `plan_origen_id`, y el original suma uno a `veces_clonado`.
func (s *Service) clonePublic(ctx context.Context, subject, rawID string, req CloneRequest) (Plan, error) {
	src, err := s.publicPlan(ctx, subject, rawID)
	if err != nil {
		return Plan{}, err
	}
	me, _ := owner(subject) // publicPlan ya lo validó
	day, err := parseDay("fecha", req.Fecha)
	if err != nil {
		return Plan{}, err
	}
	origin := src.ID
	p := Plan{
		OwnerID: me, Nombre: src.Nombre, Fecha: src.Fecha, Personas: src.Personas, Presupuesto: src.Presupuesto,
		Movilidad: src.Movilidad, Origen: OriginCloned, Visibilidad: VisibilityPrivate, PlanOrigenID: &origin,
		Paradas: make([]Parada, len(src.Paradas)),
	}
	copy(p.Paradas, publicView(src).Paradas)
	for i := range p.Paradas {
		p.Paradas[i].ID = bson.NewObjectID()
	}
	if req.Nombre != "" {
		p.Nombre = req.Nombre
	}
	moveToDay(&p, day)
	out, err := s.insert(ctx, &p)
	if err != nil {
		return Plan{}, err
	}
	if err := s.repo.IncClones(ctx, src.ID); err != nil {
		return Plan{}, errStore()
	}
	return out, nil
}

// Alternatives da hasta 3 negocios parecidos a la parada (mismo giro, precio ±1) o, con
// `q`, que coinciden por nombre, abiertos durante toda la estancia y fuera del plan. Sin IA.
func (s *Service) Alternatives(ctx context.Context, subject, rawID, rawStopID string, q AlternativesQuery) (AlternativesResponse, error) {
	p, err := s.getOwned(ctx, subject, rawID)
	if err != nil {
		return AlternativesResponse{}, err
	}
	i, err := stopIndex(&p, rawStopID)
	if err != nil {
		return AlternativesResponse{}, err
	}
	stop := p.Paradas[i]
	near, err := s.placesByID(ctx, p)
	if err != nil {
		return AlternativesResponse{}, err
	}
	current, ok := near[stop.NegocioID]
	if !ok {
		return AlternativesResponse{}, invalid("parada_id", "el lugar de la parada ya no está disponible")
	}
	found, err := s.extras.places.Candidates(ctx, negocios.CandidateQuery{
		Lat: current.Location.Coordinates[1], Lng: current.Location.Coordinates[0], RadiusM: altRadiusM,
		Conditions: alternativeConditions(p, stop, current, q.Q), Limit: altCandidates,
	})
	if err != nil {
		return AlternativesResponse{}, errStore()
	}
	out := AlternativesResponse{Items: []Alternative{}}
	end := stop.Llega.Add(time.Duration(stop.Minutos-1) * time.Minute)
	for _, c := range found {
		if len(out.Items) == altLimit {
			break
		}
		if !negocios.OpenAt(c.ScheduleMin, end) {
			continue
		}
		out.Items = append(out.Items, Alternative{
			Lugar: negocios.ItemOf(c, s.now()), DistanciaM: math.Round(c.DistanceM),
			MinutosExtra: s.extraMinutes(p, i, near, c.Business),
		})
	}
	return out, nil
}

// alternativeConditions son los filtros duros de la búsqueda (van en `$geoNear.query`).
func alternativeConditions(p Plan, stop Parada, current negocios.Business, q string) bson.A {
	inPlan := make(bson.A, 0, len(p.Paradas))
	for _, st := range p.Paradas {
		inPlan = append(inPlan, st.NegocioID)
	}
	conds := bson.A{
		negocios.OpenAtCondition(stop.Llega),
		bson.D{{Key: "_id", Value: bson.D{{Key: "$nin", Value: inPlan}}}},
	}
	if q != "" {
		pattern := regexp.QuoteMeta(text.Normalize(q))
		return append(conds, bson.D{{Key: "nombre_normalizado", Value: bson.D{{Key: "$regex", Value: pattern}}}})
	}
	return append(conds,
		bson.D{{Key: "categoria", Value: current.Category}},
		bson.D{{Key: "nivel_precio", Value: bson.D{{Key: "$gte", Value: current.PriceLevel - 1}, {Key: "$lte", Value: current.PriceLevel + 1}}}},
	)
}

// placesByID trae los negocios visibles del plan por id.
func (s *Service) placesByID(ctx context.Context, p Plan) (map[bson.ObjectID]negocios.Business, error) {
	ids := make([]bson.ObjectID, 0, len(p.Paradas))
	for _, st := range p.Paradas {
		ids = append(ids, st.NegocioID)
	}
	found, err := s.extras.places.FindVisibleSummaries(ctx, ids)
	if err != nil {
		return nil, errStore()
	}
	out := make(map[bson.ObjectID]negocios.Business, len(found))
	for _, b := range found {
		if len(b.Location.Coordinates) == 2 {
			out[b.ID] = b
		}
	}
	return out, nil
}

// extraMinutes es cuánto cambia el traslado (de la parada anterior y hacia la siguiente) si
// el candidato reemplaza a la parada i.
func (s *Service) extraMinutes(p Plan, i int, near map[bson.ObjectID]negocios.Business, cand negocios.Business) int {
	kmh := s.calc.velocidades[p.Movilidad]
	if kmh <= 0 || len(cand.Location.Coordinates) != 2 {
		return 0
	}
	current := near[p.Paradas[i].NegocioID]
	legs := func(b negocios.Business) int {
		total := 0
		for _, j := range []int{i - 1, i + 1} {
			if j < 0 || j >= len(p.Paradas) {
				continue
			}
			other, ok := near[p.Paradas[j].NegocioID]
			if !ok {
				continue
			}
			_, minutes := Tramo(lugarOf(other), lugarOf(b), kmh)
			total += minutes
		}
		return total
	}
	return legs(cand) - legs(current)
}

func lugarOf(b negocios.Business) Lugar {
	return Lugar{Lng: b.Location.Coordinates[0], Lat: b.Location.Coordinates[1], Horario: b.ScheduleMin}
}

// ChangeStop cambia el negocio de una parada con If-Match: conserva la hora y la duración,
// toma el precio promedio del negocio nuevo y recalcula. Negocio no visible, cerrado a esa
// hora o ya en el plan = 422.
func (s *Service) ChangeStop(ctx context.Context, subject, rawID, rawStopID string, version int, req ChangeStopRequest) (Plan, error) {
	id, err := bson.ObjectIDFromHex(req.NegocioID)
	if err != nil {
		return Plan{}, invalid("negocio_id", "negocio inválido")
	}
	found, err := s.extras.places.FindVisibleSummaries(ctx, []bson.ObjectID{id})
	if err != nil {
		return Plan{}, errStore()
	}
	if len(found) == 0 {
		return Plan{}, invalid("negocio_id", "el lugar no está disponible")
	}
	price := found[0].AveragePrice
	return s.edit(ctx, subject, rawID, version, func(p *Plan) error {
		i, err := stopIndex(p, rawStopID)
		if err != nil {
			return err
		}
		for _, st := range p.Paradas {
			if st.NegocioID == id {
				return invalid("negocio_id", "ese lugar ya está en el plan")
			}
		}
		p.Paradas[i].NegocioID, p.Paradas[i].Precio = id, price
		return nil
	})
}

// roundCoord deja 2 decimales (~1 km): la posición exacta no se usa ni se registra.
func roundCoord(v float64) float64 { return math.Round(v*100) / 100 }

// Popular da 3 planes públicos populares de la semana cerca del punto, sin datos del dueño.
func (s *Service) Popular(ctx context.Context, q PopularQuery) (PopularResponse, error) {
	rows, err := s.repo.Popular(ctx, roundCoord(*q.Lat), roundCoord(*q.Lng), s.now().UTC())
	if err != nil {
		return PopularResponse{}, errStore()
	}
	ids := []bson.ObjectID{}
	for _, p := range rows {
		for _, st := range p.Paradas {
			ids = append(ids, st.NegocioID)
		}
	}
	cards, err := s.extras.places.FindCards(ctx, ids, s.now())
	if err != nil {
		return PopularResponse{}, errStore()
	}
	byID := make(map[bson.ObjectID]negocios.PlaceItem, len(cards))
	for _, c := range cards {
		if id, err := bson.ObjectIDFromHex(c.ID); err == nil {
			byID[id] = c
		}
	}
	out := PopularResponse{Items: make([]PublicPlan, 0, len(rows))}
	for _, p := range rows {
		out.Items = append(out.Items, newPublicPlan(p, byID))
	}
	return out, nil
}

// ErrOutOfWindow: el paquete sin conexión solo es del plan de hoy o de mañana.
func errOutOfWindow() error {
	return apperr.Validation("el paquete sin conexión solo se descarga para el plan de hoy o de mañana",
		apperr.FieldError{Field: "fecha", Message: "fuera_de_ventana"})
}

// Package arma el paquete sin conexión: el plan, la ficha de cada lugar y las reservas
// propias del plan. Lo piden el dueño y los integrantes, solo para hoy o mañana.
func (s *Service) Package(ctx context.Context, subject, rawID string) (PackageResponse, error) {
	p, err := s.getMember(ctx, subject, rawID)
	if err != nil {
		return PackageResponse{}, err
	}
	now := s.now()
	if !inWindow(p.Fecha, now) {
		return PackageResponse{}, errOutOfWindow()
	}
	p.normalize()
	ids := make([]bson.ObjectID, 0, len(p.Paradas))
	for _, st := range p.Paradas {
		ids = append(ids, st.NegocioID)
	}
	details, err := s.extras.places.FindDetails(ctx, ids, now)
	if err != nil {
		return PackageResponse{}, errStore()
	}
	out := PackageResponse{Plan: p, Lugares: details, Reservas: []ReservaPaquete{}, GeneradoEn: now.UTC(), Version: p.Version, SyncSeq: p.SyncSeq}
	if s.extras.reservas != nil {
		me, _ := owner(subject) // getMember ya lo validó
		if out.Reservas, err = s.extras.reservas.DelPlan(ctx, me, p.ID); err != nil {
			return PackageResponse{}, errStore()
		}
	}
	return out, nil
}

// inWindow: la fecha del plan es hoy o mañana en la hora de México.
func inWindow(fecha, now time.Time) bool {
	loc := negocios.MexicoCity()
	n := now.In(loc)
	today := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, loc)
	f := fecha.In(loc)
	day := time.Date(f.Year(), f.Month(), f.Day(), 0, 0, 0, 0, loc)
	return day.Equal(today) || day.Equal(today.AddDate(0, 0, 1))
}

// PackageETag es `"{version}-{sync_seq}"`: cambia con cualquier edición del plan.
func PackageETag(p PackageResponse) string {
	return `"` + strconv.Itoa(p.Version) + "-" + strconv.FormatInt(p.SyncSeq, 10) + `"`
}

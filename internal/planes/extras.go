package planes

import (
	"context"
	"errors"
	"math"
	"regexp"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
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

// Blocks da la relación entre dos personas (lo cumple amistades.Repository): un bloqueo
// esconde los planes públicos y una amistad aceptada abre los de `amigos` (nv-84).
type Blocks interface {
	Relation(ctx context.Context, me, other bson.ObjectID) (status string, mine bool, err error)
}

// Sharing dice qué dueños no comparten sus planes (nv-82); lo cumple usuarios.Repository.
type Sharing interface {
	HidingPlans(ctx context.Context, ids []bson.ObjectID) (map[bson.ObjectID]bool, error)
}

// Reservas trae las reservas propias de un plan para el paquete (lo cumple reservas).
type Reservas interface {
	DelPlan(ctx context.Context, usuario, plan bson.ObjectID) ([]ReservaPaquete, error)
}

type extraDeps struct {
	places   Places
	blocks   Blocks
	reservas Reservas
	sharing  Sharing
}

// WithExtras activa los extras de nv-68: alternativas, cambiar parada, populares, paquete y
// planes públicos. reservas puede ser nil.
func (s *Service) WithExtras(places Places, blocks Blocks, reservas Reservas) *Service {
	s.extras = extraDeps{places: places, blocks: blocks, reservas: reservas}
	return s
}

// WithSharing quita de populares los planes de quien no los comparte (nv-82). Va después de
// WithExtras.
func (s *Service) WithSharing(sh Sharing) *Service {
	s.extras.sharing = sh
	return s
}

// Constantes de nv-68.
const (
	altRadiusM    = 3000
	altCandidates = 20
	altLimit      = 3
)

// sharedPlan trae un plan ajeno activo que quien lo pide puede ver: `publico` sin bloqueo con
// el dueño o `amigos` con amistad aceptada y el dueño compartiendo sus planes (nv-82, nv-84);
// si no, 404 como si no existiera.
func (s *Service) sharedPlan(ctx context.Context, subject, rawID string) (Plan, error) {
	reader, id, err := ids(subject, rawID)
	if err != nil {
		return Plan{}, err
	}
	p, err := s.repo.FindActive(ctx, id)
	if err != nil {
		return Plan{}, storeError(err)
	}
	if !p.SharedWithFriends() || s.extras.blocks == nil {
		return Plan{}, errPlanNotFound()
	}
	status, _, err := s.extras.blocks.Relation(ctx, reader, p.OwnerID)
	if err != nil {
		return Plan{}, errStore()
	}
	if status == amistades.StatusBlocked {
		return Plan{}, errPlanNotFound()
	}
	if p.Public() {
		return p, nil
	}
	if status != amistades.StatusAccepted {
		return Plan{}, errPlanNotFound()
	}
	if err := s.ownerShares(ctx, p.OwnerID); err != nil {
		return Plan{}, err
	}
	return p, nil
}

// ownerShares da 404 si el dueño no comparte sus planes u oculta su actividad (nv-82).
func (s *Service) ownerShares(ctx context.Context, ownerID bson.ObjectID) error {
	if s.extras.sharing == nil {
		return nil
	}
	hidden, err := s.extras.sharing.HidingPlans(ctx, []bson.ObjectID{ownerID})
	if err != nil {
		return errStore()
	}
	if hidden[ownerID] {
		return errPlanNotFound()
	}
	return nil
}

// Visible es el permiso de nv-83 para reaccionar y comentar: el dueño, un integrante del grupo
// o quien ve el plan compartido (sharedPlan) si el dueño comparte sus planes (nv-82). Devuelve
// el dueño; si no, 404 como si no existiera.
func (s *Service) Visible(ctx context.Context, subject, rawID string) (bson.ObjectID, error) {
	p, err := s.getMember(ctx, subject, rawID)
	if err == nil {
		return p.OwnerID, nil
	}
	var appErr *apperr.Error
	if !errors.As(err, &appErr) || appErr.Kind != apperr.KindNotFound {
		return bson.ObjectID{}, err
	}
	shared, err := s.sharedPlan(ctx, subject, rawID)
	if err != nil {
		return bson.ObjectID{}, err
	}
	if shared.Public() {
		// Los de `amigos` ya lo revisó sharedPlan.
		if err := s.ownerShares(ctx, shared.OwnerID); err != nil {
			return bson.ObjectID{}, err
		}
	}
	return shared.OwnerID, nil
}

// clonePublic copia un plan compartido ajeno («Hacer mi versión»): la copia es privada, `origen: clonado`, con
// `plan_origen_id`, y el original suma uno a `veces_clonado`.
func (s *Service) clonePublic(ctx context.Context, subject, rawID string, req CloneRequest) (Plan, error) {
	src, err := s.sharedPlan(ctx, subject, rawID)
	if err != nil {
		return Plan{}, err
	}
	me, _ := owner(subject) // sharedPlan ya lo validó
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
	p, err := s.Editable(ctx, subject, rawID)
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
	if rows, err = s.shared(ctx, rows); err != nil {
		return PopularResponse{}, errStore()
	}
	items, err := s.publicPlans(ctx, rows)
	if err != nil {
		return PopularResponse{}, err
	}
	return PopularResponse{Items: items}, nil
}

// shared quita los planes de quien no comparte sus planes u oculta su actividad (nv-82) y
// deja los primeros popularTop.
func (s *Service) shared(ctx context.Context, rows []Plan) ([]Plan, error) {
	if s.extras.sharing != nil && len(rows) > 0 {
		owners := make([]bson.ObjectID, 0, len(rows))
		for _, p := range rows {
			owners = append(owners, p.OwnerID)
		}
		hidden, err := s.extras.sharing.HidingPlans(ctx, owners)
		if err != nil {
			return nil, err
		}
		kept := rows[:0]
		for _, p := range rows {
			if !hidden[p.OwnerID] {
				kept = append(kept, p)
			}
		}
		rows = kept
	}
	return rows[:min(len(rows), popularTop)], nil
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
	if s.extras.reservas != nil && p.Sorpresa == nil {
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

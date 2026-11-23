package negocios

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

// Conteo y afluencia de GET /lugares (nv-66).
const (
	// MaxConteo es el tope del conteo; más = «1000+».
	MaxConteo     = 1000
	conteoTimeout = 2 * time.Second
	earthRadiusM  = 6378100.0
	diasSemana    = 7
	horasDia      = 24
)

// CountQuery son los filtros de GET /lugares/conteo: los mismos de la lista, sin orden,
// cursor ni límite.
type CountQuery struct {
	ExploreFilters
	Lat      *float64 `query:"lat" validate:"omitempty,latitude"`
	Lng      *float64 `query:"lng" validate:"omitempty,longitude"`
	RadiusM  int      `query:"radio_m" validate:"omitempty,min=1,max=20000"`
}

// CountResponse es el conteo; `exacto: false` = «1000+».
type CountResponse struct {
	Total int64 `json:"total"`
	Exact bool  `json:"exacto"`
}

// TrafficResponse son las barras de afluencia de un día (0 = domingo).
type TrafficResponse struct {
	Day         int   `json:"dia"`
	Hours       []int `json:"horas"`
	CurrentHour *int  `json:"hora_actual"`
	Enough      bool  `json:"suficiente"`
}

// countFilter arma el mismo filtro que la lista: visibilidad, categoría, precio, abierto y,
// con ubicación, un círculo con `$geoWithin` (CountDocuments no admite `$near`).
func (s *Service) countFilter(ctx context.Context, subject string, q CountQuery) (bson.D, error) {
	if (q.Lat == nil) != (q.Lng == nil) {
		return nil, apperr.Validation("lat y lng van juntos", apperr.FieldError{Field: "lat", Message: "lat y lng van juntos"})
	}
	v, err := s.loadViewer(ctx, subject, q.ExploreFilters, false)
	if err != nil {
		return nil, err
	}
	match, err := s.matchConditions(ctx, q.ExploreFilters, v)
	if err != nil {
		return nil, err
	}
	if q.Lat != nil {
		radius := float64(DefaultRadiusM)
		if q.RadiusM > 0 {
			radius = float64(q.RadiusM)
		}
		match = append(match, bson.D{{Key: "ubicacion", Value: bson.D{{Key: "$geoWithin", Value: bson.D{
			{Key: "$centerSphere", Value: bson.A{bson.A{*q.Lng, *q.Lat}, radius / earthRadiusM}},
		}}}}})
	}
	return visibleAnd(match), nil
}

// Count cuenta los lugares de los filtros hasta MaxConteo con plazo de 2 s. Plazo vencido o
// MongoDB caído = 503 (sin el texto del error: podría repetir la ubicación).
func (s *Service) Count(ctx context.Context, subject string, q CountQuery) (CountResponse, error) {
	filter, err := s.countFilter(ctx, subject, q)
	if err != nil {
		return CountResponse{}, err
	}
	n, err := s.repo.Count(ctx, filter, MaxConteo+1)
	if err != nil {
		return CountResponse{}, apperr.Unavailable("servicio no disponible")
	}
	if n > MaxConteo {
		return CountResponse{Total: MaxConteo, Exact: false}, nil
	}
	return CountResponse{Total: n, Exact: true}, nil
}

// Count cuenta hasta limit documentos de filter con plazo de 2 s.
func (r *Repository) Count(ctx context.Context, filter bson.D, limit int64) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, conteoTimeout)
	defer cancel()
	return r.col.CountDocuments(ctx, filter, options.Count().SetLimit(limit))
}

// ParseDay lee `dia` (0 = domingo … 6); vacío = hoy en la hora de México.
func ParseDay(raw string, now time.Time) (int, error) {
	if raw == "" {
		return int(now.In(MexicoCity()).Weekday()), nil
	}
	if len(raw) != 1 || raw[0] < '0' || raw[0] > '6' {
		return 0, apperr.Validation("datos inválidos", apperr.FieldError{Field: "dia", Message: "dia va de 0 (domingo) a 6"})
	}
	return int(raw[0] - '0'), nil
}

// TrafficOf arma las barras de un día. Sin matriz 7 × 24 o con el día en ceros va en ceros y
// `suficiente: false`. `hora_actual` solo si el día es hoy.
func TrafficOf(matrix [][]int, day int, now time.Time) TrafficResponse {
	out := TrafficResponse{Day: day, Hours: make([]int, horasDia)}
	local := now.In(MexicoCity())
	if int(local.Weekday()) == day {
		h := local.Hour()
		out.CurrentHour = &h
	}
	if len(matrix) != diasSemana || len(matrix[day]) != horasDia {
		return out
	}
	for h, v := range matrix[day] {
		if v > 0 {
			out.Enough = true
		}
		out.Hours[h] = v
	}
	if !out.Enough {
		out.Hours = make([]int, horasDia)
	}
	return out
}

// Traffic da la afluencia de un lugar visible; si no, 404.
func (s *Service) Traffic(ctx context.Context, rawID, rawDay string) (TrafficResponse, error) {
	now := s.now()
	day, err := ParseDay(rawDay, now)
	if err != nil {
		return TrafficResponse{}, err
	}
	id, err := bson.ObjectIDFromHex(rawID)
	if err != nil {
		return TrafficResponse{}, apperr.NotFound("lugar no encontrado")
	}
	b, ok, err := s.repo.FindVisible(ctx, id)
	if err != nil {
		return TrafficResponse{}, apperr.Unavailable("servicio no disponible")
	}
	if !ok {
		return TrafficResponse{}, apperr.NotFound("lugar no encontrado")
	}
	return TrafficOf(b.Traffic, day, now), nil
}

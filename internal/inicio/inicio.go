// Package inicio arma la pantalla de Inicio en una llamada (spec nv-25): saludo y chips
// según la franja del día, el clima y cinco recomendaciones. La sesión es opcional.
package inicio

import (
	"context"
	"time"

	"github.com/WhereverApp2026/wherever-backend/internal/clima"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/recomendaciones"
)

// Recomendaciones que muestra Inicio.
const Recomendaciones = 5

// Franjas del día en la zona de los horarios (America/Mexico_City).
const (
	FranjaManana = "manana"
	FranjaTarde  = "tarde"
	FranjaNoche  = "noche"
)

// Chip es un atajo de Inicio; la app lo abre como GET /lugares?categoria=….
type Chip struct {
	ID        string `json:"id"`
	Nombre    string `json:"nombre"`
	Categoria string `json:"categoria"`
}

var saludos = map[string]string{
	FranjaManana: "Buenos días",
	FranjaTarde:  "Buenas tardes",
	FranjaNoche:  "Buenas noches",
}

// chips por franja, sobre las categorías de nv-20.
var chips = map[string][]Chip{
	FranjaManana: {
		{ID: "desayunar", Nombre: "Desayunar", Categoria: "restaurante"},
		{ID: "cafe", Nombre: "Un café", Categoria: "cafeteria"},
		{ID: "pendientes", Nombre: "Pendientes", Categoria: "servicios"},
	},
	FranjaTarde: {
		{ID: "comer", Nombre: "Comer", Categoria: "restaurante"},
		{ID: "cafe", Nombre: "Un café", Categoria: "cafeteria"},
		{ID: "compras", Nombre: "De compras", Categoria: "comercio"},
	},
	FranjaNoche: {
		{ID: "cenar", Nombre: "Cenar", Categoria: "restaurante"},
		{ID: "bares", Nombre: "Bares", Categoria: "bar"},
		{ID: "salir", Nombre: "Salir", Categoria: "entretenimiento"},
	},
}

// FranjaDe: mañana de 05:00 a 11:59, tarde de 12:00 a 18:59 y noche el resto.
func FranjaDe(t time.Time) string {
	switch h := t.In(negocios.MexicoCity()).Hour(); {
	case h >= 5 && h < 12:
		return FranjaManana
	case h >= 12 && h < 19:
		return FranjaTarde
	default:
		return FranjaNoche
	}
}

// Query es `?lat=&lng=&hora=&compania=`. La ubicación solo se usa en la consulta.
type Query struct {
	Lat     *float64 `query:"lat" validate:"required,latitude"`
	Lng     *float64 `query:"lng" validate:"required,longitude"`
	Company string   `query:"compania" validate:"omitempty,oneof=solo pareja amigos familia"`
	Hour    string   `query:"hora" validate:"omitempty,hhmm"`
}

// Response es la respuesta de GET /inicio.
type Response struct {
	Saludo          string                 `json:"saludo"`
	Franja          string                 `json:"franja"`
	Momento         time.Time              `json:"momento"`
	Clima           clima.Clima            `json:"clima"`
	Chips           []Chip                 `json:"chips"`
	Recomendaciones []recomendaciones.Card `json:"recomendaciones"`
}

// Recomendador lo implementa *recomendaciones.Service.
type Recomendador interface {
	ForHome(ctx context.Context, subject string, q recomendaciones.Query, adverso *bool) (recomendaciones.Response, error)
}

// Clima lo implementa *clima.Servicio.
type Clima interface {
	Actual(ctx context.Context, lat, lng float64) clima.Clima
}

// Service arma Inicio.
type Service struct {
	reco  Recomendador
	clima Clima
}

// NewService recibe las recomendaciones y el clima por constructor.
func NewService(r Recomendador, c Clima) *Service {
	return &Service{reco: r, clima: c}
}

// Home arma Inicio. El clima va primero (a lo más 3 s, casi siempre de la caché) porque
// mueve el puntaje; si no llega, las recomendaciones salen sin ese factor. La franja sale
// del momento de las recomendaciones (`hora` o el reloj de ese service).
func (s *Service) Home(ctx context.Context, subject string, q Query) (Response, error) {
	cl := s.clima.Actual(ctx, *q.Lat, *q.Lng)
	rq := recomendaciones.Query{Lat: q.Lat, Lng: q.Lng, Company: q.Company, Hour: q.Hour, Limit: Recomendaciones}
	recs, err := s.reco.ForHome(ctx, subject, rq, cl.Adverso)
	if err != nil {
		return Response{}, err
	}
	franja := FranjaDe(recs.Moment)
	return Response{
		Saludo: saludos[franja], Franja: franja, Momento: recs.Moment, Clima: cl,
		Chips: chips[franja], Recomendaciones: recs.Items,
	}, nil
}

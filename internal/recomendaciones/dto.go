package recomendaciones

import (
	"time"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
)

// Límites de la lista.
const (
	DefaultLimit = 10
	MaxLimit     = 50
)

// Query es `?lat=&lng=&radio_m=&compania=&hora=&limite=`. La ubicación solo se usa en la
// consulta: no se guarda ni se registra.
type Query struct {
	Lat     *float64 `query:"lat" validate:"required,latitude"`
	Lng     *float64 `query:"lng" validate:"required,longitude"`
	RadiusM int      `query:"radio_m" validate:"omitempty,min=1,max=20000"`
	Company string   `query:"compania" validate:"omitempty,oneof=solo pareja amigos familia"`
	Hour    string   `query:"hora" validate:"omitempty,hhmm"`
	Limit   int      `query:"limite" validate:"min=1,max=50"`
}

// Card es una tarjeta de «Para ti ahora»: la misma de GET /lugares más el puntaje y el
// porqué. No lleva factores ni datos del usuario.
type Card struct {
	negocios.PlaceItem
	Score   float64  `json:"puntaje"`
	Reasons []string `json:"razones"`
	Why     string   `json:"por_que"`
}

// Response es la respuesta de GET /recomendaciones.
type Response struct {
	Items   []Card    `json:"items"`
	Company string    `json:"compania,omitempty"`
	Moment  time.Time `json:"momento"`
}

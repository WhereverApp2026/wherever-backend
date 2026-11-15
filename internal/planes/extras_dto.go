package planes

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
)

// AlternativesQuery es GET /planes/{id}/paradas/{parada_id}/alternativas: `q` busca por texto.
type AlternativesQuery struct {
	Q string `query:"q" validate:"omitempty,min=2,max=60"`
}

// ChangeStopRequest es POST /planes/{id}/paradas/{parada_id}/cambiar.
type ChangeStopRequest struct {
	NegocioID string `json:"negocio_id" validate:"required,objectid"`
}

// PopularQuery es GET /planes/populares.
type PopularQuery struct {
	Lat *float64 `query:"lat" validate:"required,latitude"`
	Lng *float64 `query:"lng" validate:"required,longitude"`
}

// Alternative es un negocio parecido a la parada, con cuánto cambia el traslado.
type Alternative struct {
	Lugar        negocios.PlaceItem `json:"lugar"`
	DistanciaM   float64            `json:"distancia_m"`
	MinutosExtra int                `json:"minutos_extra"`
}

// AlternativesResponse son hasta 3 alternativas (puede haber menos).
type AlternativesResponse struct {
	Items []Alternative `json:"items"`
}

// PublicStop es una parada de un plan público: el lugar y la hora, sin notas.
type PublicStop struct {
	Lugar   *negocios.PlaceItem `json:"lugar,omitempty"`
	Llega   time.Time           `json:"llega"`
	Minutos int                 `json:"minutos"`
}

// PublicPlan es un plan popular: sin dueño, integrantes, notas ni grupo.
type PublicPlan struct {
	ID              string       `json:"id"`
	Nombre          string       `json:"nombre"`
	Personas        int          `json:"personas"`
	Movilidad       string       `json:"movilidad"`
	CostoPorPersona int          `json:"costo_por_persona"`
	VecesClonado    int          `json:"veces_clonado"`
	Paradas         []PublicStop `json:"paradas"`
}

// PopularResponse son hasta 3 planes populares.
type PopularResponse struct {
	Items []PublicPlan `json:"items"`
}

// ReservaPaquete es una reserva propia del plan dentro del paquete sin conexión (nv-59): el
// código es lo que muestra el QR.
type ReservaPaquete struct {
	ID        string    `json:"id"`
	Codigo    string    `json:"codigo"`
	NegocioID string    `json:"negocio_id"`
	ParadaID  string    `json:"parada_id,omitempty"`
	FechaHora time.Time `json:"fecha_hora"`
	Personas  int       `json:"personas"`
	Estado    string    `json:"estado"`
}

// PackageResponse es el paquete sin conexión del plan del día.
type PackageResponse struct {
	Plan       Plan                   `json:"plan"`
	Lugares    []negocios.PlaceDetail `json:"lugares"`
	Reservas   []ReservaPaquete       `json:"reservas"`
	GeneradoEn time.Time              `json:"generado_en"`
	Version    int                    `json:"version"`
	SyncSeq    int64                  `json:"sync_seq"`
}

// publicView quita lo que otra persona no debe ver de un plan público: las notas (el dueño
// ya no sale en el JSON).
func publicView(p Plan) Plan {
	stops := make([]Parada, len(p.Paradas))
	copy(stops, p.Paradas)
	for i := range stops {
		stops[i].Nota = ""
	}
	p.Paradas = stops
	return p
}

func newPublicPlan(p Plan, cards map[bson.ObjectID]negocios.PlaceItem) PublicPlan {
	out := PublicPlan{
		ID: p.ID.Hex(), Nombre: p.Nombre, Personas: p.Personas, Movilidad: p.Movilidad,
		CostoPorPersona: p.CostoPorPersona, VecesClonado: p.VecesClonado, Paradas: make([]PublicStop, 0, len(p.Paradas)),
	}
	for _, s := range p.Paradas {
		stop := PublicStop{Llega: s.Llega, Minutos: s.Minutos}
		if c, ok := cards[s.NegocioID]; ok {
			stop.Lugar = &c
		}
		out.Paradas = append(out.Paradas, stop)
	}
	return out
}

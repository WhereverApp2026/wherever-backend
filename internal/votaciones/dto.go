package votaciones

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
)

// OpenRequest es el cuerpo de POST /grupos/{id}/votaciones.
type OpenRequest struct {
	StopID   string    `json:"parada_id" validate:"required,len=24,hexadecimal"`
	Options  []string  `json:"opciones" validate:"required,min=2,max=3,unique,dive,len=24,hexadecimal"`
	ClosesAt time.Time `json:"cierra_en" validate:"required"`
}

// CastRequest es el cuerpo de PUT /votaciones/{id}/voto.
type CastRequest struct {
	BusinessID string `json:"negocio_id" validate:"required,len=24,hexadecimal"`
}

// OptionView es una opción con su lugar (si sigue disponible) y sus votos.
type OptionView struct {
	BusinessID string              `json:"negocio_id"`
	Place      *negocios.PlaceItem `json:"lugar,omitempty"`
	Votes      int                 `json:"votos"`
}

// View es una votación vista por un integrante: conteos, no quién votó qué.
type View struct {
	ID         string       `json:"id"`
	GroupID    string       `json:"grupo_id"`
	StopID     string       `json:"parada_id"`
	Options    []OptionView `json:"opciones"`
	TotalVotes int          `json:"total_votos"`
	MyVote     string       `json:"mi_voto,omitempty"`
	ClosesAt   time.Time    `json:"cierra_en"`
	Status     string       `json:"estado"`
	Winner     string       `json:"ganadora,omitempty"`
	ClosedBy   string       `json:"cerrada_por,omitempty"`
	ClosedAt   *time.Time   `json:"cerrada_en,omitempty"`
	Applied    *bool        `json:"aplicada,omitempty"`
	Reason     string       `json:"motivo,omitempty"`
	CreatedAt  time.Time    `json:"creada_en"`
}

// ListResponse es la respuesta de GET /grupos/{id}/votaciones.
type ListResponse struct {
	Items []View `json:"items"`
}

func newView(v Vote, me bson.ObjectID, cards []negocios.PlaceItem) View {
	byID := make(map[string]negocios.PlaceItem, len(cards))
	for _, c := range cards {
		byID[c.ID] = c
	}
	counts := v.Counts()
	out := View{
		ID: v.ID.Hex(), GroupID: v.GroupID.Hex(), StopID: v.StopID.Hex(), Options: make([]OptionView, 0, len(v.Options)),
		ClosesAt: v.ClosesAt, Status: v.Status, ClosedBy: v.ClosedBy, ClosedAt: v.ClosedAt, Applied: v.Applied,
		Reason: v.Reason, CreatedAt: v.CreatedAt,
	}
	for i, id := range v.Options {
		opt := OptionView{BusinessID: id.Hex(), Votes: counts[i]}
		if c, ok := byID[id.Hex()]; ok {
			opt.Place = &c
		}
		out.Options = append(out.Options, opt)
		out.TotalVotes += counts[i]
	}
	if mine, ok := v.Ballots[me.Hex()]; ok {
		out.MyVote = mine.Hex()
	}
	if v.Winner != nil {
		out.Winner = v.Winner.Hex()
	}
	return out
}

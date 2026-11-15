package grupos

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
)

// RenameRequest es el cuerpo de PATCH /grupos/{id}: solo cambia el nombre (nv-69).
type RenameRequest struct {
	Name string `json:"nombre" validate:"required,max=40"`
}

// LocationInput es una ubicación que la persona comparte para el punto de encuentro.
type LocationInput struct {
	Lat *float64 `json:"lat" validate:"required,min=-90,max=90"`
	Lng *float64 `json:"lng" validate:"required,min=-180,max=180"`
}

// MeetingPointRequest es el cuerpo de PUT /grupos/{id}/punto-encuentro. Con `negocio_id`, el
// nombre y la ubicación salen del negocio.
type MeetingPointRequest struct {
	Name       string   `json:"nombre" validate:"required_without=BusinessID,max=60"`
	Lat        *float64 `json:"lat" validate:"required_without=BusinessID,omitempty,min=-90,max=90"`
	Lng        *float64 `json:"lng" validate:"required_without=BusinessID,omitempty,min=-180,max=180"`
	At         string   `json:"hora" validate:"required,max=40"`
	BusinessID string   `json:"negocio_id" validate:"omitempty,len=24,hexadecimal"`
}

// SurpriseRequest es el cuerpo de PUT /grupos/{id}/sorpresa; `activo: false` la apaga.
type SurpriseRequest struct {
	Active *bool `json:"activo" validate:"required"`
	// Con `activo: true` el service exige los dos (422).
	For      string `json:"para_usuario_id" validate:"omitempty,len=24,hexadecimal"`
	RevealAt string `json:"revelar_en" validate:"omitempty,max=40"`
}

// MeetingPointView es el punto de encuentro en la vista del grupo.
type MeetingPointView struct {
	Name       string    `json:"nombre"`
	Lat        float64   `json:"lat"`
	Lng        float64   `json:"lng"`
	At         time.Time `json:"hora"`
	ProposedBy string    `json:"propuesto_por"`
	BusinessID string    `json:"negocio_id,omitempty"`
}

// SurpriseView es el modo sorpresa en la vista del grupo; a la persona sorprendida solo le
// llega `revelar_en`.
type SurpriseView struct {
	For      string    `json:"para_usuario_id,omitempty"`
	RevealAt time.Time `json:"revelar_en"`
}

// SuggestResponse es GET /punto-encuentro/sugerido: solo los lugares y cuántos compartieron;
// nunca el centro ni distancias por persona.
type SuggestResponse struct {
	Places       []negocios.PlaceItem `json:"lugares"`
	Participants int                  `json:"participantes"`
}

// TransferRow es una persona en GET /punto-encuentro/traslados. La fila propia lleva minutos y
// costo exactos; las demás solo rangos.
type TransferRow struct {
	ID         string `json:"id"`
	Name       string `json:"nombre,omitempty"`
	PhotoURL   string `json:"foto_url,omitempty"`
	Mine       bool   `json:"propio"`
	Minutes    *int   `json:"minutos,omitempty"`
	Cost       *int   `json:"costo_estimado,omitempty"`
	Range      string `json:"rango,omitempty"`
	CostRange  string `json:"costo_rango,omitempty"`
	NoLocation bool   `json:"sin_ubicacion,omitempty"`
}

// TransfersResponse es GET /punto-encuentro/traslados.
type TransfersResponse struct {
	Items []TransferRow `json:"traslados"`
}

func meetingPointView(mp *MeetingPoint) *MeetingPointView {
	if mp == nil {
		return nil
	}
	out := &MeetingPointView{Name: mp.Name, Lat: mp.Location.Lat, Lng: mp.Location.Lng, At: mp.At, ProposedBy: mp.ProposedBy.Hex()}
	if mp.BusinessID != nil {
		out.BusinessID = mp.BusinessID.Hex()
	}
	return out
}

func surpriseView(s *Surprise, me bson.ObjectID, now time.Time) *SurpriseView {
	if s == nil || !s.Active || !now.Before(s.RevealAt) {
		return nil
	}
	if s.For == me {
		return &SurpriseView{RevealAt: s.RevealAt}
	}
	return &SurpriseView{For: s.For.Hex(), RevealAt: s.RevealAt}
}

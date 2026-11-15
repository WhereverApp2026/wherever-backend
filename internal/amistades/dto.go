package amistades

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/personas"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// Acciones de PATCH /amistades/{id}.
const (
	ActionAccept = "aceptar"
	ActionReject = "rechazar"
)

// StatusRejected solo aparece en la respuesta de rechazar: la solicitud ya no existe.
const StatusRejected = "rechazada"

// Direcciones de una relación vista por quien pregunta.
const (
	DirectionSent     = "enviada"
	DirectionReceived = "recibida"
)

// SendRequest es el cuerpo de POST /amistades.
type SendRequest struct {
	UserID string `json:"usuario_id" validate:"required,len=24,hexadecimal"`
}

// RespondRequest es el cuerpo de PATCH /amistades/{id}.
type RespondRequest struct {
	Action string `json:"accion" validate:"required,oneof=aceptar rechazar"`
}

// MuteRequest es el cuerpo de PUT /amistades/{id}/silencio (nv-70).
type MuteRequest struct {
	Muted *bool `json:"silenciado" validate:"required"`
}

// MuteResponse es la marca propia tras el cambio.
type MuteResponse struct {
	ID    string `json:"id"`
	Muted bool   `json:"silenciado"`
}

// ListQuery es `?estado=amigos|recibidas|enviadas&q=`.
type ListQuery struct {
	Status string `query:"estado" validate:"omitempty,oneof=amigos recibidas enviadas"`
	Query  string `query:"q" validate:"omitempty,max=50"`
}

// Item es una relación vista por quien pregunta. La persona lleva solo lo público.
type Item struct {
	ID        string          `json:"id"`
	Status    string          `json:"estado"`
	Direction string          `json:"direccion"`
	Person    personas.Person `json:"persona"`
	// Muted es mi marca de silencio (nv-70); la del otro nunca sale.
	Muted     bool      `json:"silenciado"`
	CreatedAt time.Time `json:"creada_en"`
	UpdatedAt time.Time `json:"actualizada_en"`
}

// ListResponse es la respuesta de GET /amistades.
type ListResponse struct {
	Results []Item `json:"resultados"`
	// Pending cuenta las solicitudes recibidas sin responder, sea cual sea la lista.
	Pending int64 `json:"pendientes"`
}

func newItem(f Friendship, me bson.ObjectID, p usuarios.User) Item {
	direction := DirectionReceived
	if f.From == me {
		direction = DirectionSent
	}
	return Item{
		ID: f.ID.Hex(), Status: f.Status, Direction: direction, Muted: f.MutedByUser(me),
		Person:    personas.Person{ID: p.ID.Hex(), Name: p.Name, Username: p.Username, PhotoURL: p.PhotoURL},
		CreatedAt: f.CreatedAt, UpdatedAt: f.UpdatedAt,
	}
}

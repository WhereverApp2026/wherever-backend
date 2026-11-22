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

// De dónde sale una solicitud de amistad (nv-82).
const (
	OriginSearch     = "busqueda"
	OriginCode       = "codigo"
	OriginContacts   = "contactos"
	OriginSuggestion = "sugerencia"
)

// SendRequest es el cuerpo de POST /amistades. `origen` es opcional; con `codigo` llega el
// código de amigo leído y con `contactos` el teléfono de la agenda: si quien la recibe solo
// acepta solicitudes por código (nv-82), se validan contra su cuenta, nunca se confía en
// `origen`. El teléfono no se guarda ni se registra.
type SendRequest struct {
	UserID string `json:"usuario_id" validate:"required,len=24,hexadecimal"`
	Origin string `json:"origen" validate:"omitempty,oneof=busqueda codigo contactos sugerencia"`
	Code   string `json:"codigo" validate:"omitempty,max=20"`
	Phone  string `json:"telefono" validate:"omitempty,max=40"`
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
	Muted bool `json:"silenciado"`
	// Plans y PlanningNow solo llevan valor en la lista de amigos (nv-81); null en las
	// solicitudes y en quien no comparte sus planes.
	Plans       *int64    `json:"planes"`
	PlanningNow *bool     `json:"planeando_ahora"`
	CreatedAt   time.Time `json:"creada_en"`
	UpdatedAt   time.Time `json:"actualizada_en"`
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

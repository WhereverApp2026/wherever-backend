package chat

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// ConversationCollection guarda las conversaciones directas entre dos amigos (nv-87). Sus
// mensajes viven en `mensajes` con `conversacion_id`.
const ConversationCollection = "conversaciones"

// Tipos de adjunto de un mensaje directo.
const (
	AttachPlan   = "plan"
	AttachPlace  = "lugar"
	AttachMoment = "momento"
)

// Estados de la invitación que acompaña a un plan adjunto (la otra persona en el grupo del plan).
const (
	InviteSent     = "enviada"
	InviteAccepted = "aceptada"
)

// Límites de nv-87.
const (
	// DirectSendLimit mensajes directos por DirectSendWindow y por persona, contados en MongoDB.
	DirectSendLimit  = 120
	DirectSendWindow = time.Hour
	// MaxConversations es el tope de la lista de conversaciones (las más recientes).
	MaxConversations = 100
)

// Attachment es el plan, lugar o momento de un mensaje directo. `nombre` es el del plan o el
// lugar al enviarlo (el momento no tiene); el permiso se revisa al enviar.
type Attachment struct {
	Kind string        `bson:"tipo"`
	ID   bson.ObjectID `bson:"id"`
	Name string        `bson:"nombre,omitempty"`
}

// DirectLast es la vista previa de la conversación. Borrar ese mensaje la deja vacía.
type DirectLast struct {
	MessageID bson.ObjectID  `bson:"mensaje_id"`
	Text      string         `bson:"texto"`
	AuthorID  *bson.ObjectID `bson:"autor_id,omitempty"`
	Deleted   bool           `bson:"eliminado,omitempty"`
	At        time.Time      `bson:"creado_en"`
}

// Conversation es un documento de `conversaciones`: una por par de personas (`par` único).
// `leido_hasta` guarda, por persona (id en hex), el último mensaje que leyó; `eliminada_por`
// son quienes borraron su cuenta (cuando están las dos, la conversación desaparece).
type Conversation struct {
	ID           bson.ObjectID            `bson:"_id"`
	Pair         string                   `bson:"par"`
	Participants []bson.ObjectID          `bson:"participantes"`
	Last         *DirectLast              `bson:"ultimo_mensaje,omitempty"`
	ReadUpTo     map[string]bson.ObjectID `bson:"leido_hasta,omitempty"`
	DeletedBy    []bson.ObjectID          `bson:"eliminada_por,omitempty"`
	UpdatedAt    time.Time                `bson:"actualizada_en"`
	CreatedAt    time.Time                `bson:"creada_en"`
}

// ConversationPair son los dos ids ordenados: el mismo valor sin importar quién la abrió.
func ConversationPair(a, b bson.ObjectID) string {
	x, y := a.Hex(), b.Hex()
	if x > y {
		x, y = y, x
	}
	return x + ":" + y
}

// Other es la otra persona de la conversación vista por me.
func (c Conversation) Other(me bson.ObjectID) bson.ObjectID {
	for _, p := range c.Participants {
		if p != me {
			return p
		}
	}
	return me
}

// ReadBy es el último mensaje que leyó me (cero si ninguno).
func (c Conversation) ReadBy(me bson.ObjectID) bson.ObjectID {
	return c.ReadUpTo[me.Hex()]
}

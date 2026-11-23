package chat

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// OpenRequest es el cuerpo de POST /conversaciones.
type OpenRequest struct {
	UserID string `json:"usuario_id" validate:"required,len=24,hexadecimal"`
}

// AttachmentRequest es el adjunto opcional de un mensaje directo.
type AttachmentRequest struct {
	Kind string `json:"tipo" validate:"required,oneof=plan lugar momento"`
	ID   string `json:"id" validate:"required,len=24,hexadecimal"`
}

// DirectSendRequest es el cuerpo de POST /conversaciones/{id}/mensajes.
type DirectSendRequest struct {
	Text       string             `json:"texto" validate:"required,max=1000"`
	ClientID   string             `json:"id_cliente" validate:"required,uuid"`
	Attachment *AttachmentRequest `json:"adjunto" validate:"omitempty"`
}

// DirectListQuery es GET /conversaciones/{id}/mensajes: como el chat de grupo, sin búsqueda.
type DirectListQuery struct {
	Cursor string `query:"cursor" validate:"omitempty,max=300,cursor"`
	After  string `query:"despues_de" validate:"omitempty,len=24,hexadecimal"`
	Limit  int    `query:"limite" validate:"min=1,max=100"`
}

// NewDirectListQuery trae el límite por defecto (30).
func NewDirectListQuery() DirectListQuery { return DirectListQuery{Limit: DefaultLimit} }

// Person es lo público de la otra persona de la conversación.
type Person struct {
	ID       string `json:"id"`
	Name     string `json:"nombre"`
	Username string `json:"nombre_usuario,omitempty"`
	PhotoURL string `json:"foto_url,omitempty"`
}

// LastView es la vista previa de la conversación.
type LastView struct {
	Text     string    `json:"texto"`
	AuthorID string    `json:"autor_id,omitempty"`
	Deleted  bool      `json:"eliminado"`
	At       time.Time `json:"creado_en"`
}

// ConversationView es una conversación vista por un participante. `otro` es null si la otra
// persona ya no está activa.
type ConversationView struct {
	ID        string    `json:"id"`
	Other     *Person   `json:"otro"`
	Last      *LastView `json:"ultimo_mensaje"`
	Unread    int       `json:"no_leidos"`
	CreatedAt time.Time `json:"creada_en"`
}

// ConversationList es la respuesta de GET /conversaciones.
type ConversationList struct {
	Items []ConversationView `json:"items"`
}

// AttachmentView es el adjunto visto en el mensaje. `invitacion` dice, para un plan, si la otra
// persona está invitada (`enviada`) o ya se unió (`aceptada`).
type AttachmentView struct {
	Kind   string `json:"tipo"`
	ID     string `json:"id"`
	Name   string `json:"nombre,omitempty"`
	Invite string `json:"invitacion,omitempty"`
}

// DirectView es un mensaje directo. Uno borrado sale sin texto, adjunto ni reacciones.
type DirectView struct {
	ID         string          `json:"id"`
	AuthorID   string          `json:"autor_id,omitempty"`
	Text       string          `json:"texto"`
	Attachment *AttachmentView `json:"adjunto"`
	Reactions  []ReactionView  `json:"reacciones"`
	Deleted    bool            `json:"eliminado"`
	CreatedAt  time.Time       `json:"creado_en"`
}

func newDirectView(m Message, me bson.ObjectID, invites map[bson.ObjectID]string) DirectView {
	v := DirectView{ID: m.ID.Hex(), Text: m.Text, Reactions: reactionViews(m.Reactions, me), CreatedAt: m.CreatedAt}
	if m.AuthorID != nil {
		v.AuthorID = m.AuthorID.Hex()
	}
	if m.DeletedAt != nil {
		v.Deleted, v.Text, v.Reactions = true, "", []ReactionView{}
		return v
	}
	if a := m.Attachment; a != nil {
		v.Attachment = &AttachmentView{Kind: a.Kind, ID: a.ID.Hex(), Name: a.Name, Invite: invites[m.ID]}
	}
	return v
}

func newPerson(u usuarios.User) *Person {
	return &Person{ID: u.ID.Hex(), Name: u.Name, Username: u.Username, PhotoURL: u.PhotoURL}
}

func newConversationView(c Conversation, me bson.ObjectID, people map[bson.ObjectID]usuarios.User, unread map[bson.ObjectID]int) ConversationView {
	v := ConversationView{ID: c.ID.Hex(), Unread: unread[c.ID], CreatedAt: c.CreatedAt}
	if u, ok := people[c.Other(me)]; ok {
		v.Other = newPerson(u)
	}
	if l := c.Last; l != nil {
		v.Last = &LastView{Text: l.Text, Deleted: l.Deleted, At: l.At}
		if l.AuthorID != nil {
			v.Last.AuthorID = l.AuthorID.Hex()
		}
	}
	return v
}

package chat

import (
	"slices"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// SendRequest es el cuerpo de POST /grupos/{id}/mensajes. `id_cliente` (UUID del teléfono)
// hace idempotente el reenvío.
type SendRequest struct {
	Text     string `json:"texto" validate:"required,max=1000"`
	ClientID string `json:"id_cliente" validate:"required,uuid"`
}

// ReactRequest es el cuerpo de PUT …/reacciones. `emoji: null` quita la reacción.
type ReactRequest struct {
	Emoji *string `json:"emoji" validate:"omitempty,oneof=👍 ❤️ 😂 😮 😢 🎉"`
}

// ListQuery es GET /grupos/{id}/mensajes. `cursor` sigue hacia atrás (firmado, nv-17);
// `despues_de` es el id del último mensaje que la app tiene y trae lo nuevo, del más viejo
// al más nuevo.
type ListQuery struct {
	Cursor string `query:"cursor" validate:"omitempty,max=300,cursor"`
	After  string `query:"despues_de" validate:"omitempty,len=24,hexadecimal"`
	Query  string `query:"q" validate:"omitempty,min=2,max=100"`
	Limit  int    `query:"limite" validate:"min=1,max=100"`
}

// NewListQuery trae el límite por defecto (30).
func NewListQuery() ListQuery { return ListQuery{Limit: DefaultLimit} }

// IsEmoji dice si el emoji está en la lista fija.
func IsEmoji(e string) bool { return slices.Contains(Emojis, e) }

// Author es lo público del autor; falta si la cuenta ya no está activa o es un aviso del sistema.
type Author struct {
	ID       string `json:"id"`
	Name     string `json:"nombre"`
	PhotoURL string `json:"foto_url,omitempty"`
}

// ReactionView cuenta una reacción: cuántos y si una es mía. No dice quién.
type ReactionView struct {
	Emoji string `json:"emoji"`
	Total int    `json:"total"`
	Mine  bool   `json:"mia"`
}

// View es un mensaje visto por un integrante. Uno borrado sale sin texto ni reacciones.
type View struct {
	ID        string         `json:"id"`
	Author    *Author        `json:"autor,omitempty"`
	Kind      string         `json:"tipo"`
	Text      string         `json:"texto"`
	Reactions []ReactionView `json:"reacciones"`
	Deleted   bool           `json:"eliminado"`
	CreatedAt time.Time      `json:"creado_en"`
}

// reactionViews agrupa por emoji en el orden de la lista fija.
func reactionViews(rs []Reaction, me bson.ObjectID) []ReactionView {
	out := []ReactionView{}
	for _, e := range Emojis {
		v := ReactionView{Emoji: e}
		for _, r := range rs {
			if r.Emoji == e {
				v.Total++
				v.Mine = v.Mine || r.UserID == me
			}
		}
		if v.Total > 0 {
			out = append(out, v)
		}
	}
	return out
}

func newView(m Message, me bson.ObjectID, people map[bson.ObjectID]usuarios.User) View {
	v := View{ID: m.ID.Hex(), Kind: m.Kind, Text: m.Text, Reactions: reactionViews(m.Reactions, me), CreatedAt: m.CreatedAt}
	if m.DeletedAt != nil {
		v.Deleted, v.Text, v.Reactions = true, "", []ReactionView{}
	}
	if m.AuthorID != nil {
		if u, ok := people[*m.AuthorID]; ok {
			v.Author = &Author{ID: u.ID.Hex(), Name: u.Name, PhotoURL: u.PhotoURL}
		}
	}
	return v
}

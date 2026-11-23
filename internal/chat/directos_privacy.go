package chat

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// DirectPrivacy es la parte de los mensajes directos en ARCO (nv-87): cumple
// privacidad.Purger y privacidad.Exporter sobre `conversaciones` y los mensajes con
// `conversacion_id`.
type DirectPrivacy struct {
	messages *Repository
	convs    *ConversationRepository
	now      func() time.Time
}

// NewDirectPrivacy recibe los repositorios por constructor.
func NewDirectPrivacy(messages *Repository, convs *ConversationRepository) *DirectPrivacy {
	return &DirectPrivacy{messages: messages, convs: convs, now: time.Now}
}

// Name identifica al módulo en el registro y en el JSON exportado.
func (p *DirectPrivacy) Name() string { return ConversationCollection }

// PurgeHistory no toca las conversaciones: son con otra persona, no historial de uso.
func (p *DirectPrivacy) PurgeHistory(context.Context, bson.ObjectID) error { return nil }

// PurgeAccount vacía los mensajes de la cuenta (quedan «Mensaje eliminado», sin autor) y sus
// reacciones; la conversación sigue para la otra persona y desaparece, con sus mensajes,
// cuando las dos borraron su cuenta.
func (p *DirectPrivacy) PurgeAccount(ctx context.Context, userID bson.ObjectID) error {
	if err := p.messages.PurgeAuthor(ctx, userID, p.now().UTC()); err != nil {
		return err
	}
	gone, err := p.convs.Forget(ctx, userID)
	if err != nil {
		return err
	}
	return p.messages.DeleteConversations(ctx, gone)
}

type exportedConversation struct {
	ID        string    `json:"id"`
	OtherID   string    `json:"otro_id"`
	CreatedAt time.Time `json:"creada_en"`
}

type exportedDirect struct {
	ID             string     `json:"id"`
	ConversationID string     `json:"conversacion_id"`
	Text           string     `json:"texto"`
	Attachment     *exportRef `json:"adjunto,omitempty"`
	CreatedAt      time.Time  `json:"creado_en"`
}

type exportRef struct {
	Kind string `json:"tipo"`
	ID   string `json:"id"`
}

type exportedDirectReaction struct {
	MessageID      string `json:"mensaje_id"`
	ConversationID string `json:"conversacion_id"`
	Emoji          string `json:"emoji"`
}

type exportedDirects struct {
	Conversations []exportedConversation   `json:"conversaciones"`
	Messages      []exportedDirect         `json:"mensajes"`
	Reactions     []exportedDirectReaction `json:"reacciones"`
}

// Export devuelve mis conversaciones (sin datos de la otra persona más que su id), mis
// mensajes no borrados y mis reacciones; nunca los mensajes de la otra persona.
func (p *DirectPrivacy) Export(ctx context.Context, userID bson.ObjectID) (any, error) {
	convs, err := p.convs.ListFor(ctx, userID, 0)
	if err != nil {
		return nil, err
	}
	mine, err := p.messages.ListByAuthor(ctx, fieldDirect, userID)
	if err != nil {
		return nil, err
	}
	reacted, err := p.messages.ListReactedBy(ctx, fieldDirect, userID)
	if err != nil {
		return nil, err
	}
	out := exportedDirects{
		Conversations: make([]exportedConversation, 0, len(convs)),
		Messages:      make([]exportedDirect, 0, len(mine)),
		Reactions:     []exportedDirectReaction{},
	}
	for _, c := range convs {
		out.Conversations = append(out.Conversations, exportedConversation{ID: c.ID.Hex(), OtherID: c.Other(userID).Hex(), CreatedAt: c.CreatedAt})
	}
	for _, m := range mine {
		e := exportedDirect{ID: m.ID.Hex(), ConversationID: m.ConversationID.Hex(), Text: m.Text, CreatedAt: m.CreatedAt}
		if a := m.Attachment; a != nil {
			e.Attachment = &exportRef{Kind: a.Kind, ID: a.ID.Hex()}
		}
		out.Messages = append(out.Messages, e)
	}
	for _, m := range reacted {
		for _, r := range m.Reactions {
			if r.UserID == userID {
				out.Reactions = append(out.Reactions, exportedDirectReaction{MessageID: m.ID.Hex(), ConversationID: m.ConversationID.Hex(), Emoji: r.Emoji})
			}
		}
	}
	return out, nil
}

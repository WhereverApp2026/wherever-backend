package chat

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/grupos"
)

// GroupsOf trae los grupos del usuario y limpia su `ultimo_mensaje` (grupos.Repository).
type GroupsOf interface {
	ListByUser(ctx context.Context, userID bson.ObjectID) ([]grupos.Group, error)
	ClearLastMessagesBy(ctx context.Context, userID bson.ObjectID) error
}

// Privacy es la parte del chat en ARCO (nv-05): cumple privacidad.Purger y
// privacidad.Exporter sobre `mensajes`. Se registra antes que grupos: borra el chat de los
// grupos que organizaba la cuenta antes de que esos grupos desaparezcan.
type Privacy struct {
	repo   *Repository
	groups GroupsOf
	now    func() time.Time
}

// NewPrivacy recibe el repositorio y los grupos por constructor.
func NewPrivacy(repo *Repository, groups GroupsOf) *Privacy {
	return &Privacy{repo: repo, groups: groups, now: time.Now}
}

// Name identifica al módulo en el registro y en el JSON exportado.
func (p *Privacy) Name() string { return Collection }

// PurgeHistory no toca el chat: es conversación con otras personas, no historial de uso.
func (p *Privacy) PurgeHistory(context.Context, bson.ObjectID) error { return nil }

// PurgeAccount borra el chat de los grupos que organizaba, vacía sus mensajes en los demás
// (sin autor), quita sus reacciones y limpia `ultimo_mensaje` donde era suyo.
func (p *Privacy) PurgeAccount(ctx context.Context, userID bson.ObjectID) error {
	rows, err := p.groups.ListByUser(ctx, userID)
	if err != nil {
		return err
	}
	owned := []bson.ObjectID{}
	for _, g := range rows {
		if g.OrganizerID == userID {
			owned = append(owned, g.ID)
		}
	}
	if err := p.repo.DeleteGroups(ctx, owned); err != nil {
		return err
	}
	if err := p.repo.PurgeAuthor(ctx, userID, p.now().UTC()); err != nil {
		return err
	}
	return p.groups.ClearLastMessagesBy(ctx, userID)
}

type exportedMessage struct {
	ID        string    `json:"id"`
	GroupID   string    `json:"grupo_id"`
	Text      string    `json:"texto"`
	CreatedAt time.Time `json:"creado_en"`
}

type exportedReaction struct {
	MessageID string `json:"mensaje_id"`
	GroupID   string `json:"grupo_id"`
	Emoji     string `json:"emoji"`
}

type exported struct {
	Messages  []exportedMessage  `json:"mensajes"`
	Reactions []exportedReaction `json:"reacciones"`
}

// Export devuelve los mensajes propios no borrados y las reacciones propias; nunca mensajes
// ni reacciones de otras personas.
func (p *Privacy) Export(ctx context.Context, userID bson.ObjectID) (any, error) {
	mine, err := p.repo.ListByAuthor(ctx, userID)
	if err != nil {
		return nil, err
	}
	reacted, err := p.repo.ListReactedBy(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := exported{Messages: make([]exportedMessage, 0, len(mine)), Reactions: []exportedReaction{}}
	for _, m := range mine {
		out.Messages = append(out.Messages, exportedMessage{ID: m.ID.Hex(), GroupID: m.GroupID.Hex(), Text: m.Text, CreatedAt: m.CreatedAt})
	}
	for _, m := range reacted {
		for _, r := range m.Reactions {
			if r.UserID == userID {
				out.Reactions = append(out.Reactions, exportedReaction{MessageID: m.ID.Hex(), GroupID: m.GroupID.Hex(), Emoji: r.Emoji})
			}
		}
	}
	return out, nil
}

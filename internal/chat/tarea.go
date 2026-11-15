package chat

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Dissolved dice qué grupos están disueltos (nv-69).
type Dissolved interface {
	DissolvedIDs(ctx context.Context) ([]bson.ObjectID, error)
}

// DissolvedTask es la tarea nocturna que borra los mensajes de los grupos disueltos; cumple
// privacidad.Tarea. Mientras tanto nadie los lee: el grupo disuelto ya no se encuentra.
type DissolvedTask struct {
	repo   *Repository
	groups Dissolved
}

// NewDissolvedTask recibe sus dependencias por constructor.
func NewDissolvedTask(repo *Repository, groups Dissolved) *DissolvedTask {
	return &DissolvedTask{repo: repo, groups: groups}
}

// Name identifica la tarea en los logs.
func (t *DissolvedTask) Name() string { return "chat_grupos_disueltos" }

// Run borra los mensajes; devuelve de cuántos grupos.
func (t *DissolvedTask) Run(ctx context.Context) (int, error) {
	ids, err := t.groups.DissolvedIDs(ctx)
	if err != nil {
		return 0, err
	}
	return len(ids), t.repo.DeleteGroups(ctx, ids)
}

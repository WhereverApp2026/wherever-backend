package grafo

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Privacy es la parte del grafo en ARCO (nv-05): cumple privacidad.Purger. Se registra
// después de amistades y planes: al recalcular, el usuario ya no tiene relaciones y su nodo
// se borra. No exporta nada: el grafo solo guarda ids que ya salen de otros módulos.
type Privacy struct {
	queue *Queue
}

// NewPrivacy recibe la cola por constructor.
func NewPrivacy(queue *Queue) *Privacy { return &Privacy{queue: queue} }

// Name identifica al módulo en el registro.
func (p *Privacy) Name() string { return Collection }

// PurgeHistory recalcula al usuario por si otro módulo borró algo proyectado.
func (p *Privacy) PurgeHistory(ctx context.Context, userID bson.ObjectID) error {
	return p.queue.Touch(ctx, userID)
}

// PurgeAccount recalcula al usuario: sin amistades ni planes, su nodo sale del grafo.
func (p *Privacy) PurgeAccount(ctx context.Context, userID bson.ObjectID) error {
	return p.queue.Touch(ctx, userID)
}

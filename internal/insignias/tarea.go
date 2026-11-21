package insignias

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const loteNocturno = 200

// Tarea concilia cada noche a todas las personas activas, también a las que no abrieron la
// app (privacidad.Tarea; corre con el candado del trabajo nocturno de nv-05).
type Tarea struct {
	svc *Service
}

// NewTarea recibe el service por constructor.
func NewTarea(svc *Service) *Tarea { return &Tarea{svc: svc} }

// Name identifica la tarea en los logs.
func (t *Tarea) Name() string { return "conciliar_puntos" }

// Run recorre las cuentas activas por lotes y devuelve cuántas concilió. Una que falla se
// salta (la siguiente noche o su próxima visita la concilian) y el error se devuelve al final.
func (t *Tarea) Run(ctx context.Context) (int, error) {
	n := 0
	var primero error
	desde := bson.NilObjectID
	for {
		lote, err := t.svc.repo.Activos(ctx, desde, loteNocturno)
		if err != nil {
			return n, err
		}
		for _, id := range lote {
			if _, err := t.svc.Conciliar(ctx, id); err != nil {
				if primero == nil {
					primero = err
				}
				continue
			}
			n++
		}
		if len(lote) < loteNocturno {
			return n, primero
		}
		desde = lote[len(lote)-1]
	}
}

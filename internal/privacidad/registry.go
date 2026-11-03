// Package privacidad es la mecánica de los derechos ARCO (hoja A1, spec nv-05): cada módulo
// registra su Purger y su Exporter, y el trabajo nocturno procesa los borrados a 30 días y
// las exportaciones. No importa a ningún módulo: todo llega por interfaces.
package privacidad

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Purger borra los datos de un módulo para un usuario.
type Purger interface {
	Name() string
	// PurgeHistory borra el historial de uso ("Borrar mi historial") y conserva la cuenta.
	PurgeHistory(ctx context.Context, userID bson.ObjectID) error
	// PurgeAccount borra todo lo del usuario (borrado definitivo a los 30 días).
	PurgeAccount(ctx context.Context, userID bson.ObjectID) error
}

// Exporter devuelve los datos de un módulo para "Descargar mis datos".
type Exporter interface {
	Name() string
	Export(ctx context.Context, userID bson.ObjectID) (any, error)
}

// Registry guarda los Purger y Exporter en el orden en que se registran. Se arma al arrancar
// (main) y después solo se lee: no es estado global ni cambia por petición.
type Registry struct {
	purgers   []Purger
	exporters []Exporter
}

// NewRegistry crea un registro vacío.
func NewRegistry() *Registry { return &Registry{} }

// RegisterPurger agrega un Purger; corre después de los ya registrados.
func (r *Registry) RegisterPurger(p Purger) { r.purgers = append(r.purgers, p) }

// RegisterExporter agrega un Exporter.
func (r *Registry) RegisterExporter(e Exporter) { r.exporters = append(r.exporters, e) }

// PurgeHistory corre todos los Purger en orden. El error de uno no corta a los demás: se
// devuelven todos juntos con el nombre del módulo.
func (r *Registry) PurgeHistory(ctx context.Context, userID bson.ObjectID) error {
	return r.each(func(p Purger) error { return p.PurgeHistory(ctx, userID) })
}

// PurgeAccount es como PurgeHistory, pero borra la cuenta completa.
func (r *Registry) PurgeAccount(ctx context.Context, userID bson.ObjectID) error {
	return r.each(func(p Purger) error { return p.PurgeAccount(ctx, userID) })
}

func (r *Registry) each(run func(Purger) error) error {
	var errs []error
	for _, p := range r.purgers {
		if err := run(p); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", p.Name(), err))
		}
	}
	return errors.Join(errs...)
}

// Export arma `{modulo: datos}` con todos los Exporter; los que fallan se reportan y los
// demás se incluyen igual.
func (r *Registry) Export(ctx context.Context, userID bson.ObjectID) (map[string]any, error) {
	out := make(map[string]any, len(r.exporters))
	var errs []error
	for _, e := range r.exporters {
		data, err := e.Export(ctx, userID)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", e.Name(), err))
			continue
		}
		out[e.Name()] = data
	}
	return out, errors.Join(errs...)
}

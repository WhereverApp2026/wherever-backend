package mongo

import (
	"context"
	"fmt"
	"strings"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// IndexSet son los índices de una colección. Cada módulo declara los suyos y
// scripts/indexes los aplica (constitución §5: índices solo por script idempotente).
type IndexSet struct {
	Collection string
	Models     []mongo.IndexModel
}

// EnsureIndexes crea los índices que falten. Es idempotente: un índice con el mismo
// nombre y la misma definición no cambia; uno con el mismo nombre y otra definición falla.
func EnsureIndexes(ctx context.Context, db *mongo.Database, sets ...IndexSet) error {
	for _, set := range sets {
		if _, err := db.Collection(set.Collection).Indexes().CreateMany(ctx, set.Models); err != nil {
			return fmt.Errorf("índices de %s: %w", set.Collection, err)
		}
	}
	return nil
}

// IndexVerifier comprueba en /ready que existan los índices declarados, sin crearlos: si
// alguien levanta la API sin correr scripts/indexes, `ux_correo` no existiría y una carrera
// podría crear dos cuentas con el mismo correo. Cumple httpapi.Pinger.
type IndexVerifier struct {
	db   *mongo.Database
	sets []IndexSet
}

// NewIndexVerifier recibe la base y los índices obligatorios.
func NewIndexVerifier(db *mongo.Database, sets ...IndexSet) *IndexVerifier {
	return &IndexVerifier{db: db, sets: sets}
}

// Ping falla si falta algún índice declarado (o si no se pueden listar).
func (v *IndexVerifier) Ping(ctx context.Context) error {
	var missing []string
	for _, set := range v.sets {
		specs, err := v.db.Collection(set.Collection).Indexes().ListSpecifications(ctx)
		if err != nil {
			return fmt.Errorf("listar índices de %s: %w", set.Collection, err)
		}
		have := make(map[string]bool, len(specs))
		for _, spec := range specs {
			have[spec.Name] = true
		}
		for _, model := range set.Models {
			if name := indexName(model); !have[name] {
				missing = append(missing, set.Collection+"."+name)
			}
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("faltan índices (correr scripts/indexes): %s", strings.Join(missing, ", "))
	}
	return nil
}

func indexName(model mongo.IndexModel) string {
	opts := &options.IndexOptions{}
	if model.Options != nil {
		for _, set := range model.Options.List() {
			_ = set(opts)
		}
	}
	if opts.Name == nil {
		return ""
	}
	return *opts.Name
}

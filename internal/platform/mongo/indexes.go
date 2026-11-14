package mongo

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"golang.org/x/sync/errgroup"
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
//
// Lista las colecciones en paralelo (hasta verifyParallel a la vez) y recuerda un resultado
// correcto durante verifiedFor: los índices solo cambian cuando se corre scripts/indexes, y
// revisar una por una ya no cabía en el segundo de /ready con más de 20 colecciones y una
// red lenta. Un error nunca se recuerda: la siguiente llamada vuelve a revisar.
type IndexVerifier struct {
	db   *mongo.Database
	sets []IndexSet

	mu            sync.Mutex
	verifiedUntil time.Time
	now           func() time.Time
}

const (
	verifyParallel = 8
	verifiedFor    = 5 * time.Minute
)

// NewIndexVerifier recibe la base y los índices obligatorios.
func NewIndexVerifier(db *mongo.Database, sets ...IndexSet) *IndexVerifier {
	return &IndexVerifier{db: db, sets: sets, now: time.Now}
}

// Reset olvida el último resultado correcto (pruebas, o después de cambiar índices a mano).
func (v *IndexVerifier) Reset() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.verifiedUntil = time.Time{}
}

// Ping falla si falta algún índice declarado (o si no se pueden listar).
func (v *IndexVerifier) Ping(ctx context.Context) error {
	v.mu.Lock()
	fresh := v.now().Before(v.verifiedUntil)
	v.mu.Unlock()
	if fresh {
		return nil
	}
	if err := v.verify(ctx); err != nil {
		return err
	}
	v.mu.Lock()
	v.verifiedUntil = v.now().Add(verifiedFor)
	v.mu.Unlock()
	return nil
}

func (v *IndexVerifier) verify(ctx context.Context) error {
	var (
		mu      sync.Mutex
		missing []string
	)
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(verifyParallel)
	for _, set := range v.sets {
		g.Go(func() error {
			names, err := v.missingIn(gctx, set)
			if err != nil {
				return err
			}
			mu.Lock()
			missing = append(missing, names...)
			mu.Unlock()
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("faltan índices (correr scripts/indexes): %s", strings.Join(missing, ", "))
	}
	return nil
}

// missingIn lista los índices de una colección y devuelve los declarados que no están.
func (v *IndexVerifier) missingIn(ctx context.Context, set IndexSet) ([]string, error) {
	specs, err := v.db.Collection(set.Collection).Indexes().ListSpecifications(ctx)
	if err != nil {
		return nil, fmt.Errorf("listar índices de %s: %w", set.Collection, err)
	}
	have := make(map[string]bool, len(specs))
	for _, spec := range specs {
		have[spec.Name] = true
	}
	var missing []string
	for _, model := range set.Models {
		if name := indexName(model); !have[name] {
			missing = append(missing, set.Collection+"."+name)
		}
	}
	return missing, nil
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

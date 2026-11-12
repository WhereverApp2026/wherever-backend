package grafo

import (
	"context"
	"log/slog"
	"slices"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/neo4j"
)

// Límites de Aura Free (spec nv-52): se usa la cifra menor publicada y se avisa al 70 %.
const (
	MaxNodes         = 50_000
	MaxRelationships = 175_000
	WarnPercent      = 70
)

const (
	pollEvery   = 10 * time.Second
	itemTimeout = 10 * time.Second
	queueOpTime = 5 * time.Second
	sizeEvery   = 10 * time.Minute
	batch       = 100
)

// Friends es lo que el grafo lee de amistades.
type Friends interface {
	List(ctx context.Context, me bson.ObjectID, kind amistades.Kind) ([]amistades.Friendship, error)
}

// Plans es lo que el grafo lee de planes.
type Plans interface {
	ListByOwner(ctx context.Context, owner bson.ObjectID) ([]planes.Plan, error)
}

// Graph es el lado de Neo4j.
type Graph interface {
	ProjectUser(ctx context.Context, id string, friends, businesses []string) error
	ClearProjection(ctx context.Context) error
	GraphSize(ctx context.Context) (neo4j.Size, error)
}

// Worker aplica la cola a Neo4j. Corre en cada instancia de la API; la cola reparte el trabajo.
type Worker struct {
	queue   *Queue
	friends Friends
	plans   Plans
	graph   Graph
	log     *slog.Logger
	sizedAt time.Time
}

// NewWorker recibe sus dependencias por constructor.
func NewWorker(queue *Queue, friends Friends, plans Plans, graph Graph, log *slog.Logger) *Worker {
	return &Worker{queue: queue, friends: friends, plans: plans, graph: graph, log: log}
}

// Sync deja en Neo4j las amistades aceptadas y los negocios de los planes activos del usuario,
// leídos de Mongo en este momento.
func (w *Worker) Sync(ctx context.Context, userID bson.ObjectID) error {
	rows, err := w.friends.List(ctx, userID, amistades.KindFriends)
	if err != nil {
		return err
	}
	plans, err := w.plans.ListByOwner(ctx, userID)
	if err != nil {
		return err
	}
	friends := make([]string, 0, len(rows))
	for _, f := range rows {
		friends = append(friends, f.Other(userID).Hex())
	}
	return w.graph.ProjectUser(ctx, userID.Hex(), friends, businesses(plans))
}

// businesses son los negocios distintos de las paradas de los planes activos, ordenados.
func businesses(plans []planes.Plan) []string {
	out := []string{}
	for _, p := range plans {
		if p.Estado != planes.StatusActive {
			continue
		}
		for _, stop := range p.Paradas {
			out = append(out, stop.NegocioID.Hex())
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// Drain aplica la cola hasta vaciarla (o `batch` usuarios). Al primer fallo de Neo4j deja a
// ese usuario para más tarde y para: los demás fallarían igual.
func (w *Worker) Drain(ctx context.Context) (int, error) {
	applied := 0
	for range batch {
		p, ok, err := w.queue.claim(ctx)
		if err != nil || !ok {
			return applied, err
		}
		if err := w.apply(ctx, p); err != nil {
			return applied, err
		}
		applied++
		w.checkSize(ctx)
	}
	return applied, nil
}

// apply recalcula a un usuario tomado y lo saca de la cola o lo deja para reintentar. Las
// operaciones de la cola no usan el plazo de Neo4j: si este venció, igual hay que reagendar.
func (w *Worker) apply(ctx context.Context, p pending) error {
	itemCtx, cancel := context.WithTimeout(ctx, itemTimeout)
	syncErr := w.Sync(itemCtx, p.UserID)
	cancel()
	opCtx, cancelOp := context.WithTimeout(context.WithoutCancel(ctx), queueOpTime)
	defer cancelOp()
	if syncErr != nil {
		if err := w.queue.retry(opCtx, p); err != nil {
			return err
		}
		return syncErr
	}
	return w.queue.done(opCtx, p)
}

// Run aplica la cola cada `pollEvery` o en cuanto llega un cambio, hasta que ctx termina.
func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(pollEvery)
	defer ticker.Stop()
	for {
		if _, err := w.Drain(ctx); err != nil && ctx.Err() == nil {
			w.log.Warn("grafo: se reintentará", "evento", "grafo_reintento", "error", err.Error())
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-w.queue.kick:
		}
	}
}

// checkSize cuenta nodos y relaciones (a lo más cada `sizeEvery`) y avisa al 70 % del límite.
func (w *Worker) checkSize(ctx context.Context) {
	now := time.Now()
	if now.Sub(w.sizedAt) < sizeEvery {
		return
	}
	w.sizedAt = now
	size, err := w.graph.GraphSize(ctx)
	if err != nil {
		return
	}
	if NearLimit(size) {
		w.log.Warn("grafo: cerca del límite de Aura Free", "evento", "grafo_limite", "nodos", size.Nodes, "relaciones", size.Relationships)
	}
}

// NearLimit dice si el grafo pasó el 70 % de nodos o de relaciones.
func NearLimit(size neo4j.Size) bool {
	return size.Nodes*100 >= MaxNodes*WarnPercent || size.Relationships*100 >= MaxRelationships*WarnPercent
}

package grafo

import (
	"bytes"
	"context"
	"slices"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
)

// Rebuild borra la proyección y la arma de nuevo desde Mongo; devuelve cuántos usuarios
// proyectó. Lo usa `make reconstruir-grafo`. Es idempotente.
func (w *Worker) Rebuild(ctx context.Context, db *mongo.Database) (int, error) {
	users, err := projectedUsers(ctx, db)
	if err != nil {
		return 0, err
	}
	if err := w.graph.ClearProjection(ctx); err != nil {
		return 0, err
	}
	for _, id := range users {
		if err := w.Sync(ctx, id); err != nil {
			return 0, err
		}
	}
	return len(users), nil
}

// projectedUsers son quienes tienen una amistad aceptada o un plan activo.
func projectedUsers(ctx context.Context, db *mongo.Database) ([]bson.ObjectID, error) {
	accepted := bson.D{{Key: "estado", Value: amistades.StatusAccepted}}
	active := bson.D{{Key: "estado", Value: planes.StatusActive}}
	sources := []struct {
		col, field string
		filter     bson.D
	}{
		{amistades.Collection, "de", accepted},
		{amistades.Collection, "para", accepted},
		{planes.Collection, "dueno_id", active},
	}
	var all []bson.ObjectID
	for _, s := range sources {
		var ids []bson.ObjectID
		if err := db.Collection(s.col).Distinct(ctx, s.field, s.filter).Decode(&ids); err != nil {
			return nil, err
		}
		all = append(all, ids...)
	}
	slices.SortFunc(all, func(a, b bson.ObjectID) int { return bytes.Compare(a[:], b[:]) })
	return slices.Compact(all), nil
}

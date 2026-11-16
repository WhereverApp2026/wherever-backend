package main

import (
	"context"
	"log/slog"

	mongodriver "go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/WhereverApp2026/wherever-backend/internal/estadisticas"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
)

// startEstadisticas corre el trabajo de nv-74 cada hora en cada instancia (hoy y ayer de los
// negocios con actividad y el relleno de 90 días de 50 negocios); solo usa MongoDB.
func startEstadisticas(ctx context.Context, db *mongodriver.Database, log *slog.Logger) func() {
	svc := estadisticas.NewService(estadisticas.NewRepository(db), negocios.NewRepository(db), log)
	trabajo := estadisticas.NewTrabajo(svc, log)
	done := make(chan struct{})
	go func() {
		trabajo.Run(ctx)
		close(done)
	}()
	return func() { <-done }
}

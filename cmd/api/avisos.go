package main

import (
	"context"
	"log/slog"

	mongodriver "go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/WhereverApp2026/wherever-backend/internal/cupones"
	"github.com/WhereverApp2026/wherever-backend/internal/notificaciones"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/resenas"
)

// startAvisos corre cada 5 minutos los avisos programados de nv-86 (reseña pendiente, cupón
// por vencer y hora de salir) en cada instancia; cada aviso se reserva una vez en MongoDB, sin
// Redis.
func startAvisos(ctx context.Context, db *mongodriver.Database, notifier notificaciones.Notifier, log *slog.Logger) func() {
	unico := notificaciones.NewAgrupador(notificaciones.NewRepository(db), notifier, log)
	barrido := notificaciones.NewBarrido(log,
		resenas.NewAvisoPendiente(resenas.NewRepository(db), unico),
		cupones.NewAvisoPorVencer(cupones.NewRepository(db), unico),
		planes.NewAvisoSalida(planes.NewRepository(db), unico),
	)
	done := make(chan struct{})
	go func() {
		barrido.Run(ctx)
		close(done)
	}()
	return func() { <-done }
}

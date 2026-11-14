package main

import (
	"context"
	"log/slog"

	goredis "github.com/redis/go-redis/v9"
	mongodriver "go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/privacidad"
	"github.com/WhereverApp2026/wherever-backend/internal/reservas"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// reservasParts son las reservas (nv-59) armadas y los observadores de pausa y baja del negocio
// (nv-56): hoy solo reservas; cupones (nv-61) se agrega aquí.
type reservasParts struct {
	svc          *reservas.Service
	observadores negocios.Observadores
}

// buildReservas arma las reservas y registra su ARCO. Sin el puerto de anticipos (nv-60) una
// reserva con anticipo responde 422 `anticipo_no_disponible`.
func buildReservas(db *mongodriver.Database, places *negocios.Repository, users *usuarios.Repository, rdb *goredis.Client,
	cursor *pagination.Signer, registry *privacidad.Registry, log *slog.Logger) reservasParts {
	repo := reservas.NewRepository(db)
	svc := reservas.NewService(repo, places, users, reservas.NewRedis(rdb), cursor, log)
	privacy := reservas.NewPrivacy(repo)
	registry.RegisterPurger(privacy)
	registry.RegisterExporter(privacy)
	return reservasParts{svc: svc, observadores: negocios.Observadores{reservas.NewObservador(svc)}}
}

// startReservas corre el barrido cada 5 minutos (apartados vencidos y `no_asistio`) en cada
// instancia; solo usa MongoDB.
func startReservas(ctx context.Context, db *mongodriver.Database, log *slog.Logger) func() {
	svc := reservas.NewService(reservas.NewRepository(db), negocios.NewRepository(db), nil, nil, nil, log)
	barrido := reservas.NewBarrido(svc, log)
	done := make(chan struct{})
	go func() {
		barrido.Run(ctx)
		close(done)
	}()
	return func() { <-done }
}

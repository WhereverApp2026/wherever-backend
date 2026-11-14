package main

import (
	"context"
	"log/slog"

	goredis "github.com/redis/go-redis/v9"
	mongodriver "go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/pagos"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/privacidad"
	"github.com/WhereverApp2026/wherever-backend/internal/reservas"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// reservasParts son las reservas (nv-59) y los pagos (nv-60) armados y los observadores de pausa
// y baja del negocio (nv-56): hoy solo reservas; cupones (nv-61) se agrega aquí.
type reservasParts struct {
	svc          *reservas.Service
	pagos        *pagos.Service
	observadores negocios.Observadores
}

// buildReservas arma las reservas y los pagos y registra su ARCO. Sin STRIPE_SECRET_KEY la API
// arranca, las rutas de pago responden 503 y una reserva con anticipo responde 422
// `anticipo_no_disponible`.
func buildReservas(cfg config.Config, db *mongodriver.Database, places *negocios.Repository, users *usuarios.Repository, rdb *goredis.Client,
	cursor *pagination.Signer, registry *privacidad.Registry, log *slog.Logger) reservasParts {
	repo := reservas.NewRepository(db)
	svc := reservas.NewService(repo, places, users, reservas.NewRedis(rdb), cursor, log)
	privacy := reservas.NewPrivacy(repo)
	registry.RegisterPurger(privacy)
	registry.RegisterExporter(privacy)

	var api pagos.StripeAPI
	if cfg.Stripe.Enabled() {
		api = pagos.NewClient(cfg.Stripe.SecretKey, cfg.Stripe.APIURL)
	} else {
		log.Warn("sin STRIPE_SECRET_KEY: las rutas de pago responden 503", "evento", "pagos_apagados")
	}
	payRepo := pagos.NewRepository(db)
	pay := pagos.NewService(payRepo, api, pagos.NewRedis(rdb), pagos.ReservasDe(repo, svc), places, cursor,
		pagos.Config{WebhookSecret: cfg.Stripe.WebhookSecret, LinkBase: cfg.Mail.LinkBase}, log)
	svc.WithAnticipos(pay).WithRefunder(pay)
	registry.RegisterExporter(pagos.NewPrivacy(payRepo))
	registry.RegisterExporter(pagos.NewLiquidacionesPrivacy(payRepo))
	return reservasParts{svc: svc, pagos: pay, observadores: negocios.Observadores{reservas.NewObservador(svc)}}
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

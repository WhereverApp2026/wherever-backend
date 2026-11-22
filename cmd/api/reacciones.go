package main

import (
	"log/slog"

	mongodriver "go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/grupos"
	"github.com/WhereverApp2026/wherever-backend/internal/moderacion"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/postales"
	"github.com/WhereverApp2026/wherever-backend/internal/privacidad"
	"github.com/WhereverApp2026/wherever-backend/internal/reacciones"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// reaccionesDeps son los módulos que deciden quién ve cada objeto y quién va.
type reaccionesDeps struct {
	db      *mongodriver.Database
	cards   *postales.Service
	plans   *planes.Service
	groups  *grupos.Repository
	friends *amistades.Repository
	users   *usuarios.Repository
	cursor  *pagination.Signer
}

// buildReacciones arma me gusta y comentarios (nv-83) y registra su ARCO y su destino en la cola
// de nv-43. Hasta que buildAsistente conecte la guardia todo comentario queda en revisión
// (fail-closed). Los avisos van agrupados por objeto (`notificaciones_agrupadas`). Sin
// variables nuevas.
func buildReacciones(d reaccionesDeps, mods *moderacion.Service, avisos reacciones.Avisos, registry *privacidad.Registry, log *slog.Logger) *reacciones.Service {
	repo := reacciones.NewRepository(d.db)
	svc := reacciones.NewService(reacciones.Deps{Repo: repo, Momentos: d.cards, Planes: d.plans, Grupos: d.groups, Amigos: d.friends,
		Personas: d.users, Cursor: d.cursor, Log: log}).WithModeracion(moderacion.New(nil, nil, mods, log), mods).WithAvisos(avisos)
	mods.WithDestino(reacciones.TipoCola, svc.Destino())
	privacy := reacciones.NewPrivacy(repo)
	registry.RegisterPurger(privacy)
	registry.RegisterExporter(privacy)
	return svc
}

package main

import (
	"log/slog"

	mongodriver "go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/WhereverApp2026/wherever-backend/internal/eventos"
	"github.com/WhereverApp2026/wherever-backend/internal/moderacion"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/privacidad"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
)

// buildEventos arma eventos, ferias y tianguis (nv-65): registra su ARCO y su destino en la
// cola de nv-43. Como reseñas y postales, el texto sin modelo queda en revisión (fail-closed).
// `fotos` es IMAGENES_URL: solo se aceptan fotos subidas con nv-45. Sin variables nuevas.
func buildEventos(db *mongodriver.Database, places *negocios.Repository, cursor *pagination.Signer, fotos string, mods *moderacion.Service, registry *privacidad.Registry, log *slog.Logger) *eventos.Service {
	repo := eventos.NewRepository(db)
	svc := eventos.NewService(repo, places, cursor, fotos, log).WithModeracion(moderacion.New(nil, nil, mods, log), mods)
	mods.WithDestino(eventos.TipoCola, svc.Destino())
	privacy := eventos.NewPrivacy(repo)
	registry.RegisterPurger(privacy)
	registry.RegisterExporter(privacy)
	return svc
}

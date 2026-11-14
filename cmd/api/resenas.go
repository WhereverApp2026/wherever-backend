package main

import (
	"log/slog"

	mongodriver "go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/WhereverApp2026/wherever-backend/internal/cupones"
	"github.com/WhereverApp2026/wherever-backend/internal/ia"
	"github.com/WhereverApp2026/wherever-backend/internal/imagenes"
	"github.com/WhereverApp2026/wherever-backend/internal/moderacion"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/privacidad"
	"github.com/WhereverApp2026/wherever-backend/internal/resenas"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// buildResenas arma check-ins y reseñas (nv-62) y registra su ARCO y su destino en la cola de
// nv-43. Hasta que buildAsistente conecte la guardia no hay moderador de texto: todo comentario
// queda en revisión (fail-closed). Sin IMAGENES_* una reseña con fotos responde 503. Sin
// variables nuevas.
func buildResenas(db *mongodriver.Database, places *negocios.Repository, fichas *negocios.Service, users *usuarios.Repository,
	cursor *pagination.Signer, images *imagenes.Service, mods *moderacion.Service, registry *privacidad.Registry, log *slog.Logger) *resenas.Service {
	repo := resenas.NewRepository(db)
	svc := resenas.NewService(repo, places, fichas, users, cursor, log).WithModeracion(moderacion.New(nil, nil, mods, log), mods)
	if images != nil {
		svc.WithFotos(images)
	}
	mods.WithDestino(resenas.TipoColaResena, svc.Destino())
	mods.WithDestino(resenas.TipoColaRespuesta, svc.Destino())
	privacy := resenas.NewPrivacy(repo)
	registry.RegisterPurger(privacy)
	registry.RegisterExporter(privacy)
	registry.RegisterExporter(resenas.NewCheckinsPrivacy(repo))
	return svc
}

// iaClientes son los módulos que usan la guardia de IA además del asistente.
type iaClientes struct {
	cupones    *cupones.Service
	resenas    *resenas.Service
	moderacion *moderacion.Service
}

// conectar da la guardia a los títulos de cupones (nv-61) y a las reseñas (nv-62): respuesta
// sugerida y moderación del texto con los umbrales de `moderacion_reglas`.
func (c iaClientes) conectar(guardia *ia.Guardia, log *slog.Logger) {
	c.cupones.WithIA(guardia)
	c.resenas.WithIA(guardia).WithModeracion(moderacion.New(nil, moderacion.NewTextoIA(guardia), c.moderacion, log), c.moderacion)
}

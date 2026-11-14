package main

import (
	"log/slog"

	"github.com/WhereverApp2026/wherever-backend/internal/imagenes"
	"github.com/WhereverApp2026/wherever-backend/internal/moderacion"
	"github.com/WhereverApp2026/wherever-backend/internal/notificaciones"
	"github.com/WhereverApp2026/wherever-backend/internal/postales"
	"github.com/WhereverApp2026/wherever-backend/internal/privacidad"
)

// buildPostales arma postales, recuerdos y momentos (nv-63), registra su ARCO y sus destinos en
// la cola de nv-43: los textos dudosos y la foto de una postal que se aprueba o rechaza después.
// Hasta que buildAsistente conecte la guardia todo texto queda en revisión (fail-closed). Sin
// IMAGENES_* crear una postal responde 503. Sin variables nuevas.
func buildPostales(d postales.Deps, images *imagenes.Service, mods *moderacion.Service, notifier notificaciones.Notifier, registry *privacidad.Registry, log *slog.Logger) *postales.Service {
	svc := postales.NewService(d).WithModeracion(moderacion.New(nil, nil, mods, log), mods).WithNotifier(notifier)
	if images != nil {
		svc.WithImagenes(images)
		mods.WithDestino(imagenes.TipoCola, svc.DestinoImagen(images.Destino()))
	}
	mods.WithDestino(postales.TipoColaPostal, svc.Destino())
	mods.WithDestino(postales.TipoColaMomento, svc.Destino())
	privacy := postales.NewPrivacy(d.Repo)
	registry.RegisterPurger(privacy)
	registry.RegisterExporter(privacy)
	registry.RegisterExporter(postales.NewRecuerdosPrivacy(d.Repo))
	registry.RegisterExporter(postales.NewMomentosPrivacy(d.Repo))
	return svc
}

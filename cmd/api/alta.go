package main

import (
	"context"
	"errors"
	"log/slog"

	"go.mongodb.org/mongo-driver/v2/bson"
	mongodriver "go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/WhereverApp2026/wherever-backend/internal/imagenes"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios/alta"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios/expediente"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios/menu"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios/panel"
	"github.com/WhereverApp2026/wherever-backend/internal/notificaciones"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/privacidad"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// altaParts es el alta de negocios (nv-55) armada: el service, su parte de ARCO y la tarea de
// retención del trabajo nocturno.
type altaParts struct {
	svc       *alta.Service
	exp       *expediente.Service
	privacy   *expediente.Privacy
	retencion *expediente.Retencion
	fotos     alta.Imagenes
}

// observadores son los módulos que se enteran de la pausa y la baja de un negocio (nv-56):
// reservas (nv-59) y cupones (nv-61) se agregan aquí cuando existan.
var observadores = negocios.Observadores{}

// fotosPublicadas adapta las imágenes de nv-45 a la galería del alta y al menú.
type fotosPublicadas struct{ svc *imagenes.Service }

func (f fotosPublicadas) Publicada(ctx context.Context, owner bson.ObjectID, id string) (string, error) {
	url, err := f.svc.Publicada(ctx, owner, id)
	if errors.Is(err, imagenes.ErrNoPublicada) {
		return "", alta.ErrImagenNoValida
	}
	return url, err
}

// buildAlta arma el alta y registra su ARCO. Sin DATA_KEYS el expediente responde 503 (nunca
// guarda en claro). Las fotos usan las imágenes publicadas de nv-45 (sin IMAGENES_* responden
// 503); el almacén del bucket privado sigue pendiente: la subida de documentos responde 503.
func buildAlta(cfg config.Config, db *mongodriver.Database, repo *negocios.Repository, fichas *negocios.Service, users *usuarios.Repository,
	cursor *pagination.Signer, images *imagenes.Service, notifier notificaciones.Notifier, registry *privacidad.Registry, log *slog.Logger) altaParts {
	llavero := fieldCipher(cfg.DataKeys, log)
	if llavero == nil {
		log.Warn("sin DATA_KEYS: el expediente de negocios (INE, fiscales, CLABE) responde 503", "evento", "expediente_apagado")
	}
	exp := expediente.NewService(expediente.NewRepository(db), llavero, nil, log)
	privacy := expediente.NewPrivacy(exp)
	registry.RegisterPurger(alta.NewPrivacy(repo, fichas).WithObservadores(observadores))
	registry.RegisterPurger(privacy)
	registry.RegisterExporter(privacy)
	svc := alta.NewService(repo, exp, users, cursor, alta.Config{AvisoVersion: cfg.VersionApp.Aviso}, log).
		WithFichas(fichas).WithNotifier(notifier)
	var fotos alta.Imagenes
	if images != nil {
		fotos = fotosPublicadas{svc: images}
		svc.WithImagenes(fotos)
	}
	return altaParts{svc: svc, exp: exp, privacy: privacy, retencion: expediente.NewRetencion(exp), fotos: fotos}
}

// panelParts es el panel del negocio (nv-56) armado: el panel, el menú y su tarea nocturna.
type panelParts struct {
	panel *panel.Service
	menu  *menu.Service
	tarea *panel.Tarea
}

// buildPanel arma el panel y el menú y registra la exportación de los negocios del dueño. Sin
// fuentes registradas (reservas, cupones y reseñas llegan con nv-59, nv-61 y nv-62) las tarjetas
// del panel salen en null.
func buildPanel(db *mongodriver.Database, repo *negocios.Repository, fichas *negocios.Service, a altaParts, images *imagenes.Service,
	notifier notificaciones.Notifier, registry *privacidad.Registry, log *slog.Logger) panelParts {
	menuRepo := menu.NewRepository(db)
	menuSvc := menu.NewService(menuRepo, repo, log)
	if a.fotos != nil {
		menuSvc.WithImagenes(a.fotos)
	}
	svc := panel.NewService(repo, menuRepo, a.exp, fichas, log).WithObservadores(observadores).WithNotifier(notifier)
	if images != nil {
		svc.WithImagenes(images)
	}
	registry.RegisterExporter(panel.NewPrivacy(repo, menuRepo))
	return panelParts{panel: svc, menu: menuSvc, tarea: panel.NewTarea(svc)}
}

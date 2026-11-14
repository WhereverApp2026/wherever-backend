package main

import (
	"log/slog"

	mongodriver "go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios/alta"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios/expediente"
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
	privacy   *expediente.Privacy
	retencion *expediente.Retencion
}

// buildAlta arma el alta y registra su ARCO. Sin DATA_KEYS el expediente responde 503 (nunca
// guarda en claro). El almacén del bucket privado y las fotos llegan con nv-45: hasta entonces
// la subida de documentos y de fotos responde 503.
func buildAlta(cfg config.Config, db *mongodriver.Database, repo *negocios.Repository, fichas *negocios.Service, users *usuarios.Repository,
	cursor *pagination.Signer, notifier notificaciones.Notifier, registry *privacidad.Registry, log *slog.Logger) altaParts {
	llavero := fieldCipher(cfg.DataKeys, log)
	if llavero == nil {
		log.Warn("sin DATA_KEYS: el expediente de negocios (INE, fiscales, CLABE) responde 503", "evento", "expediente_apagado")
	}
	exp := expediente.NewService(expediente.NewRepository(db), llavero, nil, log)
	privacy := expediente.NewPrivacy(exp)
	registry.RegisterPurger(alta.NewPrivacy(repo, fichas))
	registry.RegisterPurger(privacy)
	registry.RegisterExporter(privacy)
	svc := alta.NewService(repo, exp, users, cursor, alta.Config{AvisoVersion: cfg.VersionApp.Aviso}, log).
		WithFichas(fichas).WithNotifier(notifier)
	return altaParts{svc: svc, privacy: privacy, retencion: expediente.NewRetencion(exp)}
}

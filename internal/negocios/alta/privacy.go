package alta

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
)

// Privacy da de baja los negocios de la persona al borrar la cuenta (ARCO, nv-55): dejan de
// verse y no vuelven a revisión. Cumple privacidad.Purger. El expediente lo atiende
// expediente.Privacy.
type Privacy struct {
	repo         *negocios.Repository
	fichas       Fichas
	observadores negocios.Observadores
}

// NewPrivacy recibe el repositorio y la caché de fichas (puede ser nil).
func NewPrivacy(repo *negocios.Repository, fichas Fichas) *Privacy {
	return &Privacy{repo: repo, fichas: fichas}
}

// WithObservadores avisa la baja a los módulos registrados (nv-56), como la baja del panel.
func (p *Privacy) WithObservadores(o negocios.Observadores) *Privacy {
	p.observadores = o
	return p
}

// Name identifica al módulo en el registro.
func (p *Privacy) Name() string { return "negocios_alta" }

// PurgeHistory no toca negocios: no son historial de uso.
func (p *Privacy) PurgeHistory(context.Context, bson.ObjectID) error { return nil }

// PurgeAccount da de baja los negocios creados por la persona.
func (p *Privacy) PurgeAccount(ctx context.Context, userID bson.ObjectID) error {
	ids, err := p.repo.SoftDeleteByOwner(ctx, userID, time.Now().UTC())
	if err != nil {
		return err
	}
	var errs []error
	for _, id := range ids {
		if p.fichas != nil {
			p.fichas.InvalidarFicha(ctx, id.Hex())
		}
		if err := p.observadores.Avisar(ctx, negocios.AvisoDadoDeBaja, id); err != nil {
			errs = append(errs, p.repo.MarcarAviso(ctx, id, negocios.AvisoDadoDeBaja))
		}
	}
	return errors.Join(errs...)
}

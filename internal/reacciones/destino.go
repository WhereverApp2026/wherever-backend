package reacciones

import (
	"context"

	"github.com/WhereverApp2026/wherever-backend/internal/moderacion"
	"github.com/WhereverApp2026/wherever-backend/internal/notificaciones"
)

// Destino aplica la decisión del administrador (nv-43) a los comentarios dudosos.
type Destino struct{ s *Service }

// Destino devuelve el que se registra con WithDestino(TipoCola, ...).
func (s *Service) Destino() Destino { return Destino{s: s} }

// AlDecidir cumple moderacion.Destino: aprobado = publicado (suma al contador y avisa al
// dueño); si no, rechazado. Lo ya decidido o eliminado no cambia.
func (d Destino) AlDecidir(ctx context.Context, it moderacion.Item) error {
	id, ok := parseID(it.Referencia)
	if !ok || it.Tipo != TipoCola {
		return nil
	}
	estado := EstadoRechazado
	if it.Estado == moderacion.EstadoAprobado {
		estado = EstadoPublicado
	}
	c, cambio, err := d.s.Repo.Moderar(ctx, id, estado)
	if err != nil || !cambio || estado != EstadoPublicado || c.AutorID == nil {
		return err
	}
	d.s.avisar(ctx, Ref{Tipo: c.ObjetoTipo, ID: c.ObjetoID, Dueno: c.DuenoID}, *c.AutorID, notificaciones.KindComment)
	return nil
}

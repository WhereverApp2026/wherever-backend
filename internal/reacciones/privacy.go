package reacciones

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Privacy es la parte de nv-83 en ARCO (nv-05). Al eliminar la cuenta quita sus me gusta,
// deja sus comentarios como «Comentario eliminado» sin autor y borra lo que recibieron sus
// objetos. `PurgeHistory` no toca nada: es contenido de la persona, no historial de uso.
type Privacy struct{ repo *Repository }

// NewPrivacy recibe el repositorio por constructor.
func NewPrivacy(repo *Repository) *Privacy { return &Privacy{repo: repo} }

// Name es la llave en la exportación.
func (p *Privacy) Name() string { return "reacciones" }

// PurgeHistory no borra nada (contenido, no historial).
func (p *Privacy) PurgeHistory(context.Context, bson.ObjectID) error { return nil }

// PurgeAccount borra todo lo de nv-83 de la persona.
func (p *Privacy) PurgeAccount(ctx context.Context, userID bson.ObjectID) error {
	return p.repo.BorrarDe(ctx, userID)
}

type meGustaExportado struct {
	ObjetoTipo string    `json:"objeto_tipo"`
	ObjetoID   string    `json:"objeto_id"`
	CreadoEn   time.Time `json:"creado_en"`
}

type comentarioExportado struct {
	ID         string    `json:"id"`
	ObjetoTipo string    `json:"objeto_tipo"`
	ObjetoID   string    `json:"objeto_id"`
	PadreID    *string   `json:"padre_id"`
	Texto      string    `json:"texto"`
	Estado     string    `json:"estado"`
	CreadoEn   time.Time `json:"creado_en"`
}

type exportado struct {
	MeGusta     []meGustaExportado    `json:"me_gusta"`
	Comentarios []comentarioExportado `json:"comentarios"`
}

// Export devuelve los me gusta y los comentarios propios (sin los eliminados).
func (p *Privacy) Export(ctx context.Context, userID bson.ObjectID) (any, error) {
	likes, err := p.repo.MeGustaDeUsuario(ctx, userID)
	if err != nil {
		return nil, err
	}
	comments, err := p.repo.ComentariosDe(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := exportado{MeGusta: make([]meGustaExportado, 0, len(likes)), Comentarios: make([]comentarioExportado, 0, len(comments))}
	for _, l := range likes {
		out.MeGusta = append(out.MeGusta, meGustaExportado{ObjetoTipo: l.ObjetoTipo, ObjetoID: l.ObjetoID.Hex(), CreadoEn: l.CreadoEn.UTC()})
	}
	for _, c := range comments {
		var padre *string
		if c.PadreID != nil {
			h := c.PadreID.Hex()
			padre = &h
		}
		out.Comentarios = append(out.Comentarios, comentarioExportado{ID: c.ID.Hex(), ObjetoTipo: c.ObjetoTipo, ObjetoID: c.ObjetoID.Hex(),
			PadreID: padre, Texto: c.Texto, Estado: c.Estado, CreadoEn: c.CreadoEn.UTC()})
	}
	return out, nil
}

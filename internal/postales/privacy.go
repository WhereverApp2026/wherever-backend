package postales

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Privacy es la parte de nv-63 en ARCO (nv-05). Al eliminar la cuenta borra postales, recuerdos
// y momentos de la persona y la quita de los etiquetados ajenos; sus imágenes las borra nv-45.
// `PurgeHistory` no toca nada: es contenido de la persona, no historial de uso.
type Privacy struct {
	repo *Repository
}

// NewPrivacy recibe el repositorio por constructor.
func NewPrivacy(repo *Repository) *Privacy { return &Privacy{repo: repo} }

// Name es la llave de las postales en la exportación.
func (p *Privacy) Name() string { return Collection }

// PurgeHistory no borra nada (contenido, no historial).
func (p *Privacy) PurgeHistory(context.Context, bson.ObjectID) error { return nil }

// PurgeAccount borra todo lo de nv-63 de la persona.
func (p *Privacy) PurgeAccount(ctx context.Context, userID bson.ObjectID) error {
	return p.repo.BorrarDe(ctx, userID)
}

type postalExportada struct {
	ID          string    `json:"id"`
	ImagenURL   string    `json:"imagen_url,omitempty"`
	Estilo      string    `json:"estilo"`
	Texto       string    `json:"texto"`
	Visibilidad string    `json:"visibilidad"`
	Estado      string    `json:"estado"`
	PlanID      *string   `json:"plan_id,omitempty"`
	NegocioID   *string   `json:"negocio_id,omitempty"`
	CreadoEn    time.Time `json:"creado_en"`
}

// Export devuelve las postales propias con la URL pública de su imagen (solo si se publicó).
func (p *Privacy) Export(ctx context.Context, userID bson.ObjectID) (any, error) {
	ps, err := p.repo.PostalesDe(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]postalExportada, 0, len(ps))
	for _, x := range ps {
		r := respuestaPostal(x, true, nil)
		out = append(out, postalExportada{ID: r.ID, ImagenURL: r.ImagenURL, Estilo: r.Estilo, Texto: r.Texto, Visibilidad: r.Visibilidad,
			Estado: r.Estado, PlanID: r.PlanID, NegocioID: r.NegocioID, CreadoEn: r.CreadoEn})
	}
	return out, nil
}

// RecuerdosPrivacy exporta los recuerdos (su borrado va en Privacy).
type RecuerdosPrivacy struct{ repo *Repository }

// NewRecuerdosPrivacy recibe el repositorio por constructor.
func NewRecuerdosPrivacy(repo *Repository) *RecuerdosPrivacy { return &RecuerdosPrivacy{repo: repo} }

// Name es la llave de los recuerdos en la exportación.
func (p *RecuerdosPrivacy) Name() string { return RecuerdosCollection }

// Export devuelve los recuerdos propios tal como los ve la persona.
func (p *RecuerdosPrivacy) Export(ctx context.Context, userID bson.ObjectID) (any, error) {
	rs, err := p.repo.RecuerdosDe(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]RecuerdoResponse, 0, len(rs))
	for _, r := range rs {
		out = append(out, respuestaRecuerdo(r))
	}
	return out, nil
}

// MomentosPrivacy exporta los momentos (su borrado va en Privacy).
type MomentosPrivacy struct{ repo *Repository }

// NewMomentosPrivacy recibe el repositorio por constructor.
func NewMomentosPrivacy(repo *Repository) *MomentosPrivacy { return &MomentosPrivacy{repo: repo} }

// Name es la llave de los momentos en la exportación.
func (p *MomentosPrivacy) Name() string { return MomentosCollection }

type momentoExportado struct {
	ID          string    `json:"id"`
	Rol         string    `json:"rol"`
	PostalID    string    `json:"postal_id,omitempty"`
	Texto       string    `json:"texto,omitempty"`
	Visibilidad string    `json:"visibilidad,omitempty"`
	Estado      string    `json:"estado,omitempty"`
	Etiquetados []string  `json:"etiquetados,omitempty"`
	CreadoEn    time.Time `json:"creado_en,omitzero"`
}

// Export devuelve los momentos propios (`rol: autor`) y solo el id de los ajenos donde la
// etiquetaron (`rol: etiquetado`): nunca contenido de otras personas.
func (p *MomentosPrivacy) Export(ctx context.Context, userID bson.ObjectID) (any, error) {
	ms, err := p.repo.MomentosDe(ctx, userID)
	if err != nil {
		return nil, err
	}
	ajenos, err := p.repo.EtiquetadoEn(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]momentoExportado, 0, len(ms)+len(ajenos))
	for _, m := range ms {
		out = append(out, momentoExportado{ID: m.ID.Hex(), Rol: "autor", PostalID: m.PostalID.Hex(), Texto: m.Texto, Visibilidad: m.Visibilidad,
			Estado: m.Estado, Etiquetados: idsHex(m.Etiquetados), CreadoEn: m.CreadoEn.UTC()})
	}
	for _, id := range ajenos {
		out = append(out, momentoExportado{ID: id.Hex(), Rol: "etiquetado"})
	}
	return out, nil
}

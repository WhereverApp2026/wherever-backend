package insignias

import (
	"context"
	"maps"
	"slices"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Privacy es la parte de nv-78 en ARCO (nv-05): exporta las insignias ganadas y las borra con
// la cuenta. No son historial de uso: «Borrar mi historial» no las toca.
type Privacy struct {
	repo *Repository
}

// NewPrivacy recibe el repositorio por constructor.
func NewPrivacy(repo *Repository) *Privacy { return &Privacy{repo: repo} }

// Name es la llave en la exportación.
func (p *Privacy) Name() string { return Collection }

// PurgeHistory no borra nada.
func (p *Privacy) PurgeHistory(context.Context, bson.ObjectID) error { return nil }

// PurgeAccount borra las insignias de la persona.
func (p *Privacy) PurgeAccount(ctx context.Context, userID bson.ObjectID) error {
	return p.repo.BorrarDe(ctx, userID)
}

type insigniaExportada struct {
	Insignia string    `json:"insignia"`
	GanadaEn time.Time `json:"ganada_en"`
}

// Export devuelve las insignias ganadas con su fecha, también las de catálogos anteriores.
func (p *Privacy) Export(ctx context.Context, userID bson.ObjectID) (any, error) {
	fechas, err := p.repo.Ganadas(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]insigniaExportada, 0, len(fechas))
	for _, d := range Catalogo {
		if f, ok := fechas[d.ID]; ok {
			out = append(out, insigniaExportada{Insignia: d.ID, GanadaEn: f.UTC()})
			delete(fechas, d.ID)
		}
	}
	for _, id := range slices.Sorted(maps.Keys(fechas)) {
		out = append(out, insigniaExportada{Insignia: id, GanadaEn: fechas[id].UTC()})
	}
	return out, nil
}

// PrivacyPuntos es la bitácora de puntos (nv-79) en ARCO: se exporta y se borra con la cuenta.
// Tampoco es historial de uso.
type PrivacyPuntos struct {
	repo *Repository
}

// NewPrivacyPuntos recibe el repositorio por constructor.
func NewPrivacyPuntos(repo *Repository) *PrivacyPuntos { return &PrivacyPuntos{repo: repo} }

// Name es la llave en la exportación.
func (p *PrivacyPuntos) Name() string { return MovimientosColl }

// PurgeHistory no borra nada.
func (p *PrivacyPuntos) PurgeHistory(context.Context, bson.ObjectID) error { return nil }

// PurgeAccount borra los movimientos de la persona.
func (p *PrivacyPuntos) PurgeAccount(ctx context.Context, userID bson.ObjectID) error {
	return p.repo.BorrarMovimientosDe(ctx, userID)
}

type movimientoExportado struct {
	Motivo   string    `json:"motivo"`
	RefID    string    `json:"ref_id"`
	Puntos   int       `json:"puntos"`
	CreadoEn time.Time `json:"creado_en"`
}

// Export devuelve los movimientos, del más viejo al más nuevo.
func (p *PrivacyPuntos) Export(ctx context.Context, userID bson.ObjectID) (any, error) {
	movs, err := p.repo.Movimientos(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]movimientoExportado, 0, len(movs))
	for _, m := range movs {
		out = append(out, movimientoExportado{Motivo: m.Motivo, RefID: m.RefID, Puntos: m.Puntos, CreadoEn: m.CreadoEn.UTC()})
	}
	return out, nil
}

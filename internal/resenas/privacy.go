package resenas

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Privacy es la parte de reseñas en ARCO (nv-05). Al eliminar la cuenta borra los check-ins y
// las marcas de «me fue útil», y anonimiza las reseñas (texto y calificación se quedan).
type Privacy struct {
	repo *Repository
}

// NewPrivacy recibe el repositorio por constructor.
func NewPrivacy(repo *Repository) *Privacy { return &Privacy{repo: repo} }

// Name identifica al módulo en el registro y en el JSON exportado.
func (p *Privacy) Name() string { return Collection }

// PurgeHistory («borrar historial») borra los check-ins que no dieron reseña.
func (p *Privacy) PurgeHistory(ctx context.Context, userID bson.ObjectID) error {
	return p.repo.BorrarCheckins(ctx, userID, true)
}

// PurgeAccount borra check-ins y marcas de útil, y anonimiza reseñas y reportes.
func (p *Privacy) PurgeAccount(ctx context.Context, userID bson.ObjectID) error {
	if err := p.repo.BorrarCheckins(ctx, userID, false); err != nil {
		return err
	}
	if err := p.repo.BorrarUtiles(ctx, userID); err != nil {
		return err
	}
	return p.repo.Anonimizar(ctx, userID)
}

type respuestaExportada struct {
	Texto string    `json:"texto"`
	Fecha time.Time `json:"fecha"`
}

type resenaExportada struct {
	ID               string              `json:"id"`
	NegocioID        string              `json:"negocio_id"`
	Calificacion     int                 `json:"calificacion"`
	Aspectos         Aspectos            `json:"aspectos"`
	Chips            []string            `json:"chips"`
	Comentario       string              `json:"comentario"`
	Fotos            []Foto              `json:"fotos"`
	Estado           string              `json:"estado"`
	Util             int                 `json:"util"`
	RespuestaNegocio *respuestaExportada `json:"respuesta_negocio,omitempty"`
	CreadoEn         time.Time           `json:"creado_en"`
	ActualizadoEn    time.Time           `json:"actualizado_en"`
}

// Export devuelve todo lo que la persona escribió y la respuesta del negocio.
func (p *Privacy) Export(ctx context.Context, userID bson.ObjectID) (any, error) {
	rs, err := p.repo.ResenasDe(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]resenaExportada, 0, len(rs))
	for _, r := range rs {
		e := resenaExportada{
			ID: r.ID.Hex(), NegocioID: r.NegocioID.Hex(), Calificacion: r.Calificacion, Aspectos: r.Aspectos, Chips: noNil(r.Chips),
			Comentario: r.Comentario, Fotos: fotosNoNil(r.Fotos), Estado: r.Estado, Util: r.Util,
			CreadoEn: r.CreadoEn.UTC(), ActualizadoEn: r.ActualizadoEn.UTC(),
		}
		if rn := r.RespuestaNegocio; rn != nil && rn.Estado == EstadoPublicada {
			e.RespuestaNegocio = &respuestaExportada{Texto: rn.Texto, Fecha: rn.Fecha.UTC()}
		}
		out = append(out, e)
	}
	return out, nil
}

// CheckinsPrivacy exporta los check-ins (su borrado va en Privacy).
type CheckinsPrivacy struct {
	repo *Repository
}

// NewCheckinsPrivacy recibe el repositorio por constructor.
func NewCheckinsPrivacy(repo *Repository) *CheckinsPrivacy { return &CheckinsPrivacy{repo: repo} }

// Name es la llave de los check-ins en la exportación.
func (p *CheckinsPrivacy) Name() string { return CheckinsCollection }

type checkinExportado struct {
	NegocioID  string    `json:"negocio_id"`
	Metodo     string    `json:"metodo"`
	Valido     bool      `json:"valido"`
	DistanciaM *int      `json:"distancia_m,omitempty"`
	CreadoEn   time.Time `json:"creado_en"`
}

// Export devuelve negocio, método, validez, distancia y fecha; nunca hubo coordenadas.
func (p *CheckinsPrivacy) Export(ctx context.Context, userID bson.ObjectID) (any, error) {
	cs, err := p.repo.CheckinsDe(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]checkinExportado, 0, len(cs))
	for _, c := range cs {
		out = append(out, checkinExportado{NegocioID: c.NegocioID.Hex(), Metodo: c.Metodo, Valido: c.Valido, DistanciaM: c.DistanciaM, CreadoEn: c.CreadoEn.UTC()})
	}
	return out, nil
}

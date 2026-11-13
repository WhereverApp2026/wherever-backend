package moderacion

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Privacy cumple privacidad.Purger y privacidad.Exporter para moderacion_cola: el contenido
// del usuario que espera o esperó revisión.
type Privacy struct {
	repo *Repository
}

// NewPrivacy recibe el repositorio por constructor.
func NewPrivacy(repo *Repository) *Privacy { return &Privacy{repo: repo} }

// Name identifica la parte en la exportación.
func (p *Privacy) Name() string { return "moderacion" }

// PurgeHistory no borra nada: la cola no es historial de uso, es contenido publicado o por
// publicar que el usuario borra desde su recurso de origen.
func (p *Privacy) PurgeHistory(context.Context, bson.ObjectID) error { return nil }

// PurgeAccount borra todo lo del usuario en la cola.
func (p *Privacy) PurgeAccount(ctx context.Context, userID bson.ObjectID) error {
	return p.repo.BorrarDeAutor(ctx, userID)
}

// exportado es lo que ve el usuario: su contenido y la decisión, sin puntajes internos ni quién
// la tomó.
type exportado struct {
	Tipo       string     `json:"tipo"`
	Referencia string     `json:"referencia"`
	Texto      string     `json:"texto,omitempty"`
	Estado     string     `json:"estado"`
	Motivo     string     `json:"motivo,omitempty"`
	CreadoEn   time.Time  `json:"creado_en"`
	DecididoEn *time.Time `json:"decidido_en,omitempty"`
}

// Export devuelve lo del usuario en la cola.
func (p *Privacy) Export(ctx context.Context, userID bson.ObjectID) (any, error) {
	items, err := p.repo.ListarDeAutor(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]exportado, 0, len(items))
	for _, it := range items {
		out = append(out, exportado{Tipo: it.Tipo, Referencia: it.Referencia, Texto: it.Texto, Estado: it.Estado, Motivo: it.Motivo, CreadoEn: it.CreadoEn, DecididoEn: it.DecididoEn})
	}
	return out, nil
}

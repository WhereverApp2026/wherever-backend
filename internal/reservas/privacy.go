package reservas

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Privacy es la parte de `reservas` en ARCO (nv-05). Borrar la cuenta cancela las reservas
// futuras (el negocio deja de verlas) y conserva las pasadas sin nota: los pagos que cuelgan de
// ellas se guardan 5 años (nv-60). "Borrar historial" no las toca.
type Privacy struct {
	repo *Repository
	now  func() time.Time
}

// NewPrivacy recibe el repositorio por constructor.
func NewPrivacy(repo *Repository) *Privacy { return &Privacy{repo: repo, now: time.Now} }

// Name identifica al módulo en el registro y en el JSON exportado.
func (p *Privacy) Name() string { return Collection }

// PurgeHistory no toca las reservas.
func (p *Privacy) PurgeHistory(context.Context, bson.ObjectID) error { return nil }

// PurgeAccount cancela las futuras y quita las notas.
func (p *Privacy) PurgeAccount(ctx context.Context, userID bson.ObjectID) error {
	now := p.now().UTC()
	if _, err := p.repo.CambiarVarias(ctx, bson.D{
		{Key: "usuario_id", Value: userID}, {Key: "fecha_hora", Value: bson.D{{Key: "$gte", Value: now}}},
		{Key: "estado", Value: bson.D{{Key: "$in", Value: activos}}},
	}, EstadoCancelada, PorSistema, now); err != nil {
		return err
	}
	return p.repo.QuitarNotas(ctx, userID)
}

type exportada struct {
	NegocioID string    `json:"negocio_id"`
	FechaHora time.Time `json:"fecha_hora"`
	Personas  int       `json:"personas"`
	Estado    string    `json:"estado"`
	Codigo    string    `json:"codigo"`
	Nota      string    `json:"nota,omitempty"`
}

// Export devuelve las reservas de la persona (negocio, fecha, personas, estado y código).
func (p *Privacy) Export(ctx context.Context, userID bson.ObjectID) (any, error) {
	rs, err := p.repo.DeUsuario(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]exportada, 0, len(rs))
	for _, r := range rs {
		out = append(out, exportada{NegocioID: r.NegocioID.Hex(), FechaHora: r.FechaHora.UTC(), Personas: r.Personas, Estado: r.Estado, Codigo: r.Codigo, Nota: r.Nota})
	}
	return out, nil
}

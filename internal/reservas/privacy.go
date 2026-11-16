package reservas

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
)

// Privacy es la parte de `reservas` en ARCO (nv-05). Borrar la cuenta cancela las reservas
// futuras (el negocio deja de verlas), conserva las pasadas sin nota (los pagos que cuelgan de
// ellas se guardan 5 años, nv-60) y borra los mensajes que recibió la persona (nv-73). "Borrar
// historial" no las toca.
type Privacy struct {
	repo     *Repository
	negocios NombresNegocio
	now      func() time.Time
}

// NombresNegocio da el nombre del negocio de cada mensaje exportado (nv-73).
type NombresNegocio interface {
	FindByID(ctx context.Context, id bson.ObjectID) (negocios.Business, error)
}

// NewPrivacy recibe el repositorio por constructor.
func NewPrivacy(repo *Repository) *Privacy { return &Privacy{repo: repo, now: time.Now} }

// WithNegocios agrega el nombre del negocio a los mensajes exportados.
func (p *Privacy) WithNegocios(n NombresNegocio) *Privacy {
	p.negocios = n
	return p
}

// Name identifica al módulo en el registro y en el JSON exportado.
func (p *Privacy) Name() string { return Collection }

// PurgeHistory no toca las reservas ni sus mensajes (no son historial de uso).
func (p *Privacy) PurgeHistory(context.Context, bson.ObjectID) error { return nil }

// PurgeAccount cancela las futuras, quita las notas y borra los mensajes recibidos.
func (p *Privacy) PurgeAccount(ctx context.Context, userID bson.ObjectID) error {
	now := p.now().UTC()
	if _, err := p.repo.CambiarVarias(ctx, bson.D{
		{Key: "usuario_id", Value: userID}, {Key: "fecha_hora", Value: bson.D{{Key: "$gte", Value: now}}},
		{Key: "estado", Value: bson.D{{Key: "$in", Value: activos}}},
	}, EstadoCancelada, PorSistema, now); err != nil {
		return err
	}
	if err := p.repo.QuitarNotas(ctx, userID); err != nil {
		return err
	}
	return p.repo.BorrarMensajesDeUsuario(ctx, userID)
}

type exportada struct {
	NegocioID string             `json:"negocio_id"`
	FechaHora time.Time          `json:"fecha_hora"`
	Personas  int                `json:"personas"`
	Estado    string             `json:"estado"`
	Codigo    string             `json:"codigo"`
	Nota      string             `json:"nota,omitempty"`
	Mensajes  []mensajeExportado `json:"mensajes"`
}

type mensajeExportado struct {
	Texto    string    `json:"texto"`
	Negocio  string    `json:"negocio"`
	CreadoEn time.Time `json:"creado_en"`
}

// Export devuelve las reservas de la persona (negocio, fecha, personas, estado y código) con
// los mensajes que recibió en cada una (texto, nombre del negocio y fecha).
func (p *Privacy) Export(ctx context.Context, userID bson.ObjectID) (any, error) {
	rs, err := p.repo.DeUsuario(ctx, userID)
	if err != nil {
		return nil, err
	}
	ms, err := p.repo.MensajesDeUsuario(ctx, userID)
	if err != nil {
		return nil, err
	}
	porReserva := map[bson.ObjectID][]mensajeExportado{}
	nombres := map[bson.ObjectID]string{}
	for _, m := range ms {
		porReserva[m.ReservaID] = append(porReserva[m.ReservaID], mensajeExportado{Texto: m.Texto, Negocio: p.nombre(ctx, nombres, m.NegocioID), CreadoEn: m.CreadoEn.UTC()})
	}
	out := make([]exportada, 0, len(rs))
	for _, r := range rs {
		mensajes := porReserva[r.ID]
		if mensajes == nil {
			mensajes = []mensajeExportado{}
		}
		out = append(out, exportada{NegocioID: r.NegocioID.Hex(), FechaHora: r.FechaHora.UTC(), Personas: r.Personas, Estado: r.Estado, Codigo: r.Codigo, Nota: r.Nota, Mensajes: mensajes})
	}
	return out, nil
}

// nombre resuelve el nombre del negocio una vez por negocio; sin fuente o sin negocio, vacío.
func (p *Privacy) nombre(ctx context.Context, cache map[bson.ObjectID]string, id bson.ObjectID) string {
	if n, ok := cache[id]; ok {
		return n
	}
	nombre := ""
	if p.negocios != nil {
		if b, err := p.negocios.FindByID(ctx, id); err == nil {
			nombre = b.Name
		}
	}
	cache[id] = nombre
	return nombre
}

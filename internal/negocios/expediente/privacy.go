package expediente

import (
	"context"
	"encoding/json"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Privacy es la parte del expediente en ARCO (nv-05): cumple privacidad.Purger y
// privacidad.Exporter. La bitácora de accesos no se borra con la cuenta.
type Privacy struct {
	svc *Service
}

// NewPrivacy recibe el service por constructor.
func NewPrivacy(svc *Service) *Privacy { return &Privacy{svc: svc} }

// Name identifica al módulo en el registro y en el JSON exportado.
func (p *Privacy) Name() string { return ExportName }

// PurgeHistory conserva el expediente: no es historial de uso.
func (p *Privacy) PurgeHistory(context.Context, bson.ObjectID) error { return nil }

// PurgeAccount es la cancelación: borra INE, comprobante y CLABE y bloquea los fiscales.
func (p *Privacy) PurgeAccount(ctx context.Context, userID bson.ObjectID) error {
	return p.svc.Cancelar(ctx, userID)
}

// RetirarDatosNegocio aplica la oposición cuando la persona retira el consentimiento
// `datos_negocio` (nv-19): igual que la cancelación para estos datos.
func (p *Privacy) RetirarDatosNegocio(ctx context.Context, userID bson.ObjectID) error {
	return p.svc.Cancelar(ctx, userID)
}

// ExportItem es un registro en "Descargar mis datos": metadatos, datos fiscales y CLABE
// enmascarada; nunca los archivos (se descargan aparte con GET /negocios/{id}/documentos/{tipo}).
type ExportItem struct {
	NegocioID string         `json:"negocio_id"`
	Tipo      string         `json:"tipo"`
	Estado    string         `json:"estado"`
	SubidoEn  time.Time      `json:"subido_en"`
	Datos     map[string]any `json:"datos,omitempty"`
}

// Export devuelve el expediente de la persona. Sin llavero, solo metadatos.
func (p *Privacy) Export(ctx context.Context, userID bson.ObjectID) (any, error) {
	rows, err := p.svc.repo.DePropietario(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]ExportItem, 0, len(rows))
	for _, d := range rows {
		item := ExportItem{NegocioID: d.NegocioID.Hex(), Tipo: d.Tipo, Estado: d.Estado, SubidoEn: d.SubidoEn.UTC()}
		if d.DatosCifrados != "" && p.svc.llavero != nil {
			item.Datos = p.datos(d)
		}
		out = append(out, item)
	}
	return out, nil
}

func (p *Privacy) datos(d Documento) map[string]any {
	plano, err := p.svc.llavero.Descifrar(lugar(d.NegocioID, d.Tipo), d.DatosCifrados)
	if err != nil {
		return nil
	}
	var m map[string]any
	if json.Unmarshal([]byte(plano), &m) != nil {
		return nil
	}
	if clabe, ok := m["clabe"].(string); ok {
		delete(m, "clabe")
		m["clabe_ultimos4"] = Enmascarar(clabe)
	}
	return m
}

// loteRetencion acota cuántos registros vencidos borra cada noche.
const loteRetencion = 500

// Retencion es la tarea nocturna de retención (cumple privacidad.Tarea).
type Retencion struct {
	svc *Service
}

// NewRetencion recibe el service por constructor.
func NewRetencion(svc *Service) *Retencion { return &Retencion{svc: svc} }

// Name identifica la tarea en los logs.
func (r *Retencion) Name() string { return "retencion_expediente" }

// Run borra lo vencido.
func (r *Retencion) Run(ctx context.Context) (int, error) { return r.svc.Retencion(ctx, loteRetencion) }

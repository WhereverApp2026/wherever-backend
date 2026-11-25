package pagos

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
)

// Cobrados son los estados de un pago que sí se cobró; solo ellos entran a los totales (nv-93).
var Cobrados = []string{EstadoAprobado, EstadoReembolsadoParcial, EstadoReembolsado}

// CobrosQuery es la consulta de GET /negocios/{id}/cobros (nv-93).
type CobrosQuery struct {
	Cursor string `query:"cursor" validate:"omitempty,max=512"`
	Limite int    `query:"limite" validate:"min=1,max=50"`
	Desde  string `query:"desde" validate:"omitempty,datetime=2006-01-02"`
	Hasta  string `query:"hasta" validate:"omitempty,datetime=2006-01-02"`
	Estado string `query:"estado" validate:"omitempty,oneof=requiere_pago procesando aprobado rechazado reembolsado_parcial reembolsado"`
}

// NewCobrosQuery da el límite por defecto.
func NewCobrosQuery() CobrosQuery {
	return CobrosQuery{Limite: pagination.DefaultLimit}
}

// CobroFila es un cobro de la lista: ids, fechas, desglose y tarjeta, sin datos de quien pagó
// ni ids de Stripe.
type CobroFila struct {
	PagoID            string    `json:"pago_id"`
	ReservaID         string    `json:"reserva_id"`
	FechaReserva      time.Time `json:"fecha_reserva"`
	FechaCobro        time.Time `json:"fecha_cobro"`
	Anticipo          int64     `json:"anticipo"`
	Reembolsos        int64     `json:"reembolsos"`
	Comision          int64     `json:"comision"`
	Neto              int64     `json:"neto"`
	Estado            string    `json:"estado"`
	Tarjeta           *Tarjeta  `json:"tarjeta,omitempty"`
	LiquidacionSemana string    `json:"liquidacion_semana,omitempty"`
}

// CobrosTotales suma los pagos cobrados del rango y los filtros, no solo los de la página.
type CobrosTotales struct {
	Pagos      int   `json:"pagos"`
	Anticipo   int64 `json:"anticipo"`
	Reembolsos int64 `json:"reembolsos"`
	Comision   int64 `json:"comision"`
	Neto       int64 `json:"neto"`
}

// CobrosPage es la página de cobros con los totales del rango.
type CobrosPage struct {
	pagination.PageResult[CobroFila]
	Totales CobrosTotales `json:"totales"`
}

// Totales suma el desglose de los pagos.
func Totales(ps []Pago) CobrosTotales {
	var d Desglose
	for _, p := range ps {
		d = d.Sumar(DesgloseDe(p))
	}
	return CobrosTotales{Pagos: len(ps), Anticipo: d.Anticipo, Reembolsos: d.Reembolsos, Comision: d.Comision, Neto: d.Neto}
}

func cobroFila(p Pago, loc *time.Location) CobroFila {
	d := DesgloseDe(p)
	out := CobroFila{
		PagoID: p.ID.Hex(), ReservaID: p.ReservaID.Hex(), FechaReserva: p.FechaReserva.UTC(), FechaCobro: p.CreadoEn.UTC(),
		Anticipo: d.Anticipo, Reembolsos: d.Reembolsos, Comision: d.Comision, Neto: d.Neto, Estado: p.Estado, Tarjeta: p.Tarjeta,
	}
	if p.LiquidacionSemana != nil {
		out.LiquidacionSemana = fecha(*p.LiquidacionSemana, loc)
	}
	return out
}

// RangoCobros convierte `desde` y `hasta` (AAAA-MM-DD, ambos incluidos, en `loc`) en el rango
// [desde 00:00, día siguiente a hasta 00:00) en UTC. `hasta` antes de `desde` = 422.
func RangoCobros(desde, hasta string, loc *time.Location) (*time.Time, *time.Time, error) {
	dia := func(campo, raw string, sumar int) (*time.Time, error) {
		if raw == "" {
			return nil, nil
		}
		t, err := time.ParseInLocation("2006-01-02", raw, loc)
		if err != nil {
			return nil, apperr.Validation("datos inválidos", apperr.FieldError{Field: campo, Message: "fecha inválida (AAAA-MM-DD)"})
		}
		t = t.AddDate(0, 0, sumar).UTC()
		return &t, nil
	}
	d, err := dia("desde", desde, 0)
	if err != nil {
		return nil, nil, err
	}
	h, err := dia("hasta", hasta, 1)
	if err != nil {
		return nil, nil, err
	}
	if d != nil && h != nil && !h.After(*d) {
		return nil, nil, apperr.Validation("datos inválidos", apperr.FieldError{Field: "hasta", Message: "hasta no puede ser anterior a desde"})
	}
	return d, h, nil
}

// Cobros lista los cobros del negocio por cursor, del más nuevo al más viejo, con los totales
// del rango (nv-93). Ajeno o inexistente = 404.
func (s *Service) Cobros(ctx context.Context, cl *middleware.Claims, raw string, q CobrosQuery) (CobrosPage, error) {
	b, err := s.negocioPropio(ctx, cl, raw)
	if err != nil {
		return CobrosPage{}, err
	}
	if s.cursor == nil {
		return CobrosPage{}, errStore()
	}
	desde, hasta, err := RangoCobros(q.Desde, q.Hasta, s.loc)
	if err != nil {
		return CobrosPage{}, err
	}
	f := FiltroCobros{Negocio: b.ID, Desde: desde, Hasta: hasta, Estado: q.Estado}
	scope := s.cursor.Scope("cobros", b.ID.Hex(), q.Desde, q.Hasta, q.Estado)
	despues, err := s.posicion(scope, q.Cursor)
	if err != nil {
		return CobrosPage{}, err
	}
	rows, err := s.repo.Cobros(ctx, f, despues, q.Limite)
	if err != nil {
		return CobrosPage{}, errStore()
	}
	next := ""
	if len(rows) > q.Limite {
		rows = rows[:q.Limite]
		last := rows[len(rows)-1]
		if next, err = s.cursor.Encode(pagination.Cursor{ID: last.ID.Hex(), Sort: last.CreadoEn.UTC().Format(time.RFC3339Nano), Scope: scope}); err != nil {
			return CobrosPage{}, errStore()
		}
	}
	cobrados, err := s.repo.CobradosDe(ctx, f)
	if err != nil {
		return CobrosPage{}, errStore()
	}
	items := make([]CobroFila, 0, len(rows))
	for _, p := range rows {
		items = append(items, cobroFila(p, s.loc))
	}
	return CobrosPage{PageResult: pagination.NewPageResult(items, next, q.Limite), Totales: Totales(cobrados)}, nil
}

// posicion lee el último cobro del cursor firmado; vacío = desde el más nuevo; ajeno o
// alterado = 422.
func (s *Service) posicion(scope, raw string) (*Posicion, error) {
	if raw == "" {
		return nil, nil
	}
	invalido := apperr.Validation("datos inválidos", apperr.FieldError{Field: "cursor", Message: "cursor inválido"})
	c, ok, err := s.cursor.Decode(scope, raw)
	if err != nil || !ok {
		return nil, invalido
	}
	t, err := time.Parse(time.RFC3339Nano, c.Sort)
	if err != nil {
		return nil, invalido
	}
	id, err := bson.ObjectIDFromHex(c.ID)
	if err != nil {
		return nil, invalido
	}
	return &Posicion{CreadoEn: t, ID: id}, nil
}

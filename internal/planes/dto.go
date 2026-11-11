package planes

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
)

// Formatos de fecha de los DTO: el plan es un día de México y la llegada, un instante.
const (
	DayLayout   = "2006-01-02"
	StampLayout = time.RFC3339
)

// Pestañas de GET /planes (A4). Grupales llega con nv-37.
const (
	TabUpcoming = "proximos"
	TabSaved    = "guardados"
	TabPast     = "pasados"
)

// StopRequest es una parada nueva: negocio, llegada, minutos, precio por persona y nota.
type StopRequest struct {
	NegocioID string `json:"negocio_id" validate:"required,objectid"`
	Llega     string `json:"llega" validate:"required,datetime=2006-01-02T15:04:05Z07:00"`
	Minutos   int    `json:"minutos" validate:"min=1,max=720"`
	Precio    int    `json:"precio" validate:"min=0,max=100000"`
	Nota      string `json:"nota" validate:"max=200"`
}

// CreateRequest es POST /planes.
type CreateRequest struct {
	Nombre      string        `json:"nombre" validate:"required,max=80"`
	Fecha       string        `json:"fecha" validate:"required,datetime=2006-01-02"`
	Personas    int           `json:"personas" validate:"min=1,max=20"`
	Presupuesto int           `json:"presupuesto" validate:"min=0,max=1000000"`
	Movilidad   string        `json:"movilidad" validate:"required,oneof=a_pie bicicleta auto transporte_publico"`
	Origen      string        `json:"origen" validate:"omitempty,oneof=manual ia reglas"`
	Paradas     []StopRequest `json:"paradas" validate:"required,min=1,max=6,dive"`
}

// PatchRequest es PATCH /planes/{id}: solo cambian los campos presentes.
type PatchRequest struct {
	Nombre      *string `json:"nombre" validate:"omitempty,min=1,max=80"`
	Fecha       *string `json:"fecha" validate:"omitempty,datetime=2006-01-02"`
	Personas    *int    `json:"personas" validate:"omitempty,min=1,max=20"`
	Presupuesto *int    `json:"presupuesto" validate:"omitempty,min=0,max=1000000"`
	Movilidad   *string `json:"movilidad" validate:"omitempty,oneof=a_pie bicicleta auto transporte_publico"`
}

// PatchStopRequest es PATCH /planes/{id}/paradas/{parada_id}: mover, cambiar duración,
// precio o nota.
type PatchStopRequest struct {
	Llega   *string `json:"llega" validate:"omitempty,datetime=2006-01-02T15:04:05Z07:00"`
	Minutos *int    `json:"minutos" validate:"omitempty,min=1,max=720"`
	Precio  *int    `json:"precio" validate:"omitempty,min=0,max=100000"`
	Nota    *string `json:"nota" validate:"omitempty,max=200"`
}

// OrderRequest es PUT /planes/{id}/paradas/orden: todos los ids de parada en el orden nuevo.
type OrderRequest struct {
	Orden []string `json:"orden" validate:"required,min=1,max=6,dive,objectid"`
}

// CloneRequest es POST /planes/{id}/clonar: fecha nueva y, si se quiere, otro nombre.
type CloneRequest struct {
	Fecha  string `json:"fecha" validate:"required,datetime=2006-01-02"`
	Nombre string `json:"nombre" validate:"omitempty,max=80"`
}

// ListQuery es GET /planes.
type ListQuery struct {
	Pestana string `query:"pestana" validate:"oneof=proximos guardados pasados"`
	Cursor  string `query:"cursor" validate:"omitempty,max=200,cursor"`
	Limit   int    `query:"limite" validate:"min=1,max=50"`
}

// NewListQuery trae los valores por defecto: próximos y 20 por página.
func NewListQuery() ListQuery {
	return ListQuery{Pestana: TabUpcoming, Limit: pagination.DefaultLimit}
}

func invalid(field, message string) error {
	return apperr.Validation(message, apperr.FieldError{Field: field, Message: message})
}

// parseDay lee un día de México (00:00 local).
func parseDay(field, raw string) (time.Time, error) {
	d, err := time.ParseInLocation(DayLayout, raw, negocios.MexicoCity())
	if err != nil {
		return time.Time{}, invalid(field, "fecha inválida")
	}
	return d, nil
}

func parseStamp(field, raw string) (time.Time, error) {
	t, err := time.Parse(StampLayout, raw)
	if err != nil {
		return time.Time{}, invalid(field, "hora inválida")
	}
	return t.UTC(), nil
}

func (r StopRequest) toStop(field string) (Parada, error) {
	business, err := bson.ObjectIDFromHex(r.NegocioID)
	if err != nil {
		return Parada{}, invalid(field+".negocio_id", "id inválido")
	}
	at, err := parseStamp(field+".llega", r.Llega)
	if err != nil {
		return Parada{}, err
	}
	return Parada{ID: bson.NewObjectID(), NegocioID: business, Llega: at, Minutos: r.Minutos, Precio: r.Precio, Nota: r.Nota}, nil
}

// toPlan arma el plan sin calcular; el service lo pasa por la calculadora.
func (r CreateRequest) toPlan(owner bson.ObjectID) (Plan, error) {
	day, err := parseDay("fecha", r.Fecha)
	if err != nil {
		return Plan{}, err
	}
	origin := r.Origen
	if origin == "" {
		origin = OriginManual
	}
	p := Plan{
		OwnerID: owner, Nombre: r.Nombre, Fecha: day, Personas: r.Personas, Presupuesto: r.Presupuesto,
		Movilidad: r.Movilidad, Origen: origin, Paradas: make([]Parada, 0, len(r.Paradas)),
	}
	for _, s := range r.Paradas {
		stop, err := s.toStop("paradas")
		if err != nil {
			return Plan{}, err
		}
		p.Paradas = append(p.Paradas, stop)
	}
	return p, nil
}

// apply cambia los campos presentes; una fecha nueva mueve las paradas los mismos días.
func (r PatchRequest) apply(p *Plan) error {
	if r.Fecha != nil {
		day, err := parseDay("fecha", *r.Fecha)
		if err != nil {
			return err
		}
		moveToDay(p, day)
	}
	setIf(&p.Nombre, r.Nombre)
	setIf(&p.Personas, r.Personas)
	setIf(&p.Presupuesto, r.Presupuesto)
	setIf(&p.Movilidad, r.Movilidad)
	return nil
}

func (r PatchStopRequest) apply(s *Parada) error {
	if r.Llega != nil {
		at, err := parseStamp("llega", *r.Llega)
		if err != nil {
			return err
		}
		s.Llega = at
	}
	setIf(&s.Minutos, r.Minutos)
	setIf(&s.Precio, r.Precio)
	setIf(&s.Nota, r.Nota)
	return nil
}

func setIf[T any](dst *T, v *T) {
	if v != nil {
		*dst = *v
	}
}

// moveToDay cambia la fecha y mueve cada llegada los mismos días de calendario en México.
func moveToDay(p *Plan, day time.Time) {
	loc := negocios.MexicoCity()
	from := p.Fecha.In(loc)
	days := int(time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC).
		Sub(time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)).Hours() / 24)
	for i := range p.Paradas {
		p.Paradas[i].Llega = p.Paradas[i].Llega.In(loc).AddDate(0, 0, days).UTC()
	}
	p.Fecha = day
}

// reorder reparte las mismas horas de llegada (ya en orden) en el orden pedido. `ids` debe
// nombrar cada parada una sola vez.
func reorder(stops []Parada, ids []string) error {
	if len(ids) != len(stops) {
		return invalid("orden", "el orden debe nombrar todas las paradas una vez")
	}
	byID := make(map[string]Parada, len(stops))
	times := make([]time.Time, len(stops))
	for i, s := range stops {
		byID[s.ID.Hex()] = s
		times[i] = s.Llega
	}
	out := make([]Parada, 0, len(stops))
	for i, id := range ids {
		s, ok := byID[id]
		if !ok {
			return invalid("orden", "el orden debe nombrar todas las paradas una vez")
		}
		delete(byID, id)
		s.Llega = times[i]
		out = append(out, s)
	}
	copy(stops, out)
	return nil
}

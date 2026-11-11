package asistente

import (
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/recomendador"
)

// Parámetros del armado por reglas (nv-31). nv-32 lo cambia por el itinerario de la IA
// verificado contra los mismos candidatos.
const (
	MaxCandidatos   = 15 // candidatos del motor que entran al armado (y, con nv-32, a la IA)
	ParadasObjetivo = 3
	minutosDefault  = 60
	redondeoLlegada = 5 * time.Minute
)

// minutosPorGiro es la estancia sugerida por giro de A2 §4.
var minutosPorGiro = map[string]int{
	"restaurante": 90, "cafeteria": 60, "bar": 90, "entretenimiento": 120, "servicios": 60, "comercio": 45,
}

// ErrSinCandidatos: ningún negocio abierto y compatible cabe en el plan (422).
var ErrSinCandidatos = errors.New("no hay lugares abiertos y compatibles cerca")

// Pedido es lo que fija el armado además de los candidatos.
type Pedido struct {
	Inicio         time.Time
	Personas       int
	Movilidad      string
	PresupuestoMax int // MXN por persona; 0 = sin tope
}

// armado guarda las paradas elegidas mientras se recorre la lista.
type armado struct {
	pedido  Pedido
	kmh     float64
	paradas []planes.Parada
	lugares map[bson.ObjectID]planes.Lugar
	usados  []recomendador.Recomendacion
	gasto   int
}

// Armar elige hasta 3 paradas de los candidatos en el orden del motor: primero giros
// distintos y luego repetidos, cada una abierta al llegar y al salir y dentro del
// presupuesto por persona. planes.Recalcular escribe traslados y costos.
func Armar(recs []recomendador.Recomendacion, p Pedido, v planes.Velocidades) (planes.Plan, []recomendador.Recomendacion, error) {
	kmh, ok := v[p.Movilidad]
	if !ok || kmh <= 0 {
		return planes.Plan{}, nil, planes.ErrMobility
	}
	a := &armado{pedido: p, kmh: kmh, lugares: map[bson.ObjectID]planes.Lugar{}}
	for _, distintos := range []bool{true, false} {
		for _, r := range recs {
			if len(a.paradas) == ParadasObjetivo {
				break
			}
			a.intentar(r, distintos)
		}
	}
	if len(a.paradas) == 0 {
		return planes.Plan{}, nil, ErrSinCandidatos
	}
	plan := planes.Plan{
		Nombre: nombrePlan(p.Inicio), Fecha: p.Inicio, Personas: p.Personas,
		Presupuesto: p.PresupuestoMax * p.Personas, Movilidad: p.Movilidad,
		Paradas: a.paradas, Origen: planes.OriginAI,
	}
	if err := planes.Recalcular(&plan, a.lugares, v); err != nil {
		return planes.Plan{}, nil, err
	}
	return plan, a.usados, nil
}

func (a *armado) intentar(r recomendador.Recomendacion, distintos bool) {
	b := r.Negocio
	lugar, ok := lugarDe(b)
	if !ok || a.usado(b, distintos) {
		return
	}
	if a.pedido.PresupuestoMax > 0 && a.gasto+b.AveragePrice > a.pedido.PresupuestoMax {
		return
	}
	llega, minutos := a.llegada(lugar), estancia(b.Category)
	salida := llega.Add(time.Duration(minutos-1) * time.Minute)
	if !negocios.OpenAt(b.ScheduleMin, llega) || !negocios.OpenAt(b.ScheduleMin, salida) {
		return
	}
	a.paradas = append(a.paradas, planes.Parada{
		ID: bson.NewObjectID(), NegocioID: b.ID, Llega: llega, Minutos: minutos, Precio: b.AveragePrice,
	})
	a.lugares[b.ID] = lugar
	a.usados = append(a.usados, r)
	a.gasto += b.AveragePrice
}

// usado: el negocio ya está en el plan o, en la primera vuelta, su giro.
func (a *armado) usado(b negocios.Business, distintos bool) bool {
	for _, u := range a.usados {
		if u.Negocio.ID == b.ID || (distintos && u.Negocio.Category == b.Category) {
			return true
		}
	}
	return false
}

// llegada: la primera parada empieza a la hora pedida; las demás al terminar la anterior más
// el traslado, redondeado hacia arriba a 5 minutos.
func (a *armado) llegada(to planes.Lugar) time.Time {
	if len(a.paradas) == 0 {
		return a.pedido.Inicio
	}
	prev := a.paradas[len(a.paradas)-1]
	_, traslado := planes.Tramo(a.lugares[prev.NegocioID], to, a.kmh)
	t := prev.Llega.Add(time.Duration(prev.Minutos+traslado) * time.Minute)
	if r := t.Truncate(redondeoLlegada); r.Before(t) {
		return r.Add(redondeoLlegada)
	}
	return t
}

func lugarDe(b negocios.Business) (planes.Lugar, bool) {
	if len(b.Location.Coordinates) != 2 {
		return planes.Lugar{}, false
	}
	return planes.Lugar{Lng: b.Location.Coordinates[0], Lat: b.Location.Coordinates[1], Horario: b.ScheduleMin}, true
}

func estancia(giro string) int {
	if m, ok := minutosPorGiro[giro]; ok {
		return m
	}
	return minutosDefault
}

// nombrePlan pone un nombre editable según la franja de inicio (hora de la Ciudad de México).
func nombrePlan(inicio time.Time) string {
	switch h := inicio.In(zona).Hour(); {
	case h >= 5 && h < 12:
		return "Plan de la mañana"
	case h >= 12 && h < 19:
		return "Plan de la tarde"
	default:
		return "Plan de la noche"
	}
}

package asistente

import (
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/recomendador"
)

// Parámetros del armado. Armar (por reglas, nv-31) y Verificar (itinerario de la IA, nv-32)
// comparten el mismo recorrido y las mismas reglas: abierta al llegar y al salir, dentro del
// presupuesto por persona y sin repetir negocio.
const (
	MaxCandidatos   = 15 // candidatos del motor que entran al armado y a la IA
	ParadasObjetivo = 3
	minutosDefault  = 60
	redondeoLlegada = 5 * time.Minute
	// maxEspera: una hora de la IA más de 60 min después de la llegada posible deja un hueco
	// muerto en el plan y cuenta como orden de horas roto.
	maxEspera = 60 * time.Minute
)

// Motivos de descarte de una parada propuesta por la IA (US-043).
const (
	MotivoFueraDeCandidatos = "fuera_de_candidatos"
	MotivoRepetido          = "repetido"
	MotivoCerrado           = "cerrado"
	MotivoPresupuesto       = "presupuesto"
	MotivoOrdenHoras        = "orden_horas"
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

// Descarte es una parada de la IA que la API no aceptó. NegocioID es cero si el id ni
// siquiera es un ObjectID.
type Descarte struct {
	Motivo    string
	NegocioID bson.ObjectID
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

func nuevoArmado(p Pedido, v planes.Velocidades) (*armado, error) {
	kmh, ok := v[p.Movilidad]
	if !ok || kmh <= 0 {
		return nil, planes.ErrMobility
	}
	return &armado{pedido: p, kmh: kmh, lugares: map[bson.ObjectID]planes.Lugar{}}, nil
}

// Armar elige hasta 3 paradas de los candidatos en el orden del motor: primero giros
// distintos y luego repetidos. planes.Recalcular escribe traslados y costos.
func Armar(recs []recomendador.Recomendacion, p Pedido, v planes.Velocidades) (planes.Plan, []recomendador.Recomendacion, error) {
	a, err := nuevoArmado(p, v)
	if err != nil {
		return planes.Plan{}, nil, err
	}
	a.completar(recs, ParadasObjetivo)
	plan, err := a.plan(v)
	if err != nil {
		return planes.Plan{}, nil, err
	}
	return plan, a.usados, nil
}

// Verificar revisa el itinerario de la IA contra los candidatos que la API le dio (US-043):
// descarta la parada con un id fuera de la lista, repetida, con la hora antes de poder llegar
// (o con más de 60 min de espera), cerrada al llegar o al salir, o que rompe el presupuesto.
// Cada descartada se reemplaza al final con el siguiente candidato del motor, con las reglas
// de Armar. Devuelve los descartes para registrarlos.
func Verificar(it Itinerario, recs []recomendador.Recomendacion, p Pedido, v planes.Velocidades) (planes.Plan, []recomendador.Recomendacion, []Descarte, error) {
	a, err := nuevoArmado(p, v)
	if err != nil {
		return planes.Plan{}, nil, nil, err
	}
	var descartes []Descarte
	for _, pia := range it.Paradas {
		if d, ok := a.proponer(pia, recs); !ok {
			descartes = append(descartes, d)
		}
	}
	a.completar(recs, len(it.Paradas))
	plan, err := a.plan(v)
	if err != nil {
		return planes.Plan{}, nil, descartes, err
	}
	return plan, a.usados, descartes, nil
}

// proponer intenta agregar una parada de la IA tal como la propuso (hora y estancia).
func (a *armado) proponer(pia ParadaIA, recs []recomendador.Recomendacion) (Descarte, bool) {
	id, err := bson.ObjectIDFromHex(pia.ID)
	if err != nil {
		return Descarte{Motivo: MotivoFueraDeCandidatos}, false
	}
	r, ok := buscar(recs, id)
	lugar, conUbicacion := lugarDe(r.Negocio)
	if !ok || !conUbicacion {
		return Descarte{Motivo: MotivoFueraDeCandidatos, NegocioID: id}, false
	}
	if a.usado(r.Negocio, false) {
		return Descarte{Motivo: MotivoRepetido, NegocioID: id}, false
	}
	llega, minima := a.horaIA(pia.Llega), a.minima(lugar)
	if llega.Before(minima) || llega.After(minima.Add(maxEspera)) {
		return Descarte{Motivo: MotivoOrdenHoras, NegocioID: id}, false
	}
	if motivo := a.cabe(r.Negocio, llega, pia.Minutos); motivo != "" {
		return Descarte{Motivo: motivo, NegocioID: id}, false
	}
	a.agregar(r, lugar, llega, pia.Minutos)
	return Descarte{}, true
}

// completar agrega candidatos del motor hasta tener `objetivo` paradas: primero giros
// distintos y luego repetidos.
func (a *armado) completar(recs []recomendador.Recomendacion, objetivo int) {
	for _, distintos := range []bool{true, false} {
		for _, r := range recs {
			if len(a.paradas) >= objetivo {
				return
			}
			a.intentar(r, distintos)
		}
	}
}

func (a *armado) intentar(r recomendador.Recomendacion, distintos bool) {
	lugar, ok := lugarDe(r.Negocio)
	if !ok || a.usado(r.Negocio, distintos) {
		return
	}
	llega, minutos := a.llegada(lugar), estancia(r.Negocio.Category)
	if a.cabe(r.Negocio, llega, minutos) != "" {
		return
	}
	a.agregar(r, lugar, llega, minutos)
}

// cabe devuelve el motivo por el que la parada no entra, o "" si entra.
func (a *armado) cabe(b negocios.Business, llega time.Time, minutos int) string {
	if a.pedido.PresupuestoMax > 0 && a.gasto+b.AveragePrice > a.pedido.PresupuestoMax {
		return MotivoPresupuesto
	}
	salida := llega.Add(time.Duration(minutos-1) * time.Minute)
	if !negocios.OpenAt(b.ScheduleMin, llega) || !negocios.OpenAt(b.ScheduleMin, salida) {
		return MotivoCerrado
	}
	return ""
}

func (a *armado) agregar(r recomendador.Recomendacion, lugar planes.Lugar, llega time.Time, minutos int) {
	b := r.Negocio
	a.paradas = append(a.paradas, planes.Parada{
		ID: bson.NewObjectID(), NegocioID: b.ID, Llega: llega, Minutos: minutos, Precio: b.AveragePrice,
	})
	a.lugares[b.ID] = lugar
	a.usados = append(a.usados, r)
	a.gasto += b.AveragePrice
}

func (a *armado) plan(v planes.Velocidades) (planes.Plan, error) {
	if len(a.paradas) == 0 {
		return planes.Plan{}, ErrSinCandidatos
	}
	p := a.pedido
	plan := planes.Plan{
		Nombre: nombrePlan(p.Inicio), Fecha: p.Inicio, Personas: p.Personas,
		Presupuesto: p.PresupuestoMax * p.Personas, Movilidad: p.Movilidad,
		Paradas: a.paradas, Origen: planes.OriginAI,
	}
	if err := planes.Recalcular(&plan, a.lugares, v); err != nil {
		return planes.Plan{}, err
	}
	return plan, nil
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

// minima: la primera parada puede empezar a la hora pedida; las demás al terminar la anterior
// más el traslado.
func (a *armado) minima(to planes.Lugar) time.Time {
	if len(a.paradas) == 0 {
		return a.pedido.Inicio
	}
	prev := a.paradas[len(a.paradas)-1]
	_, traslado := planes.Tramo(a.lugares[prev.NegocioID], to, a.kmh)
	return prev.Llega.Add(time.Duration(prev.Minutos+traslado) * time.Minute)
}

// llegada es la mínima redondeada hacia arriba a 5 minutos.
func (a *armado) llegada(to planes.Lugar) time.Time {
	t := a.minima(to)
	if r := t.Truncate(redondeoLlegada); r.Before(t) {
		return r.Add(redondeoLlegada)
	}
	return t
}

// horaIA pone la hora HH:MM de la IA en el día del inicio; si queda más de 12 h antes del
// inicio es del día siguiente (un plan de noche que cruza la medianoche). Validar ya revisó
// el formato.
func (a *armado) horaIA(hhmm string) time.Time {
	h, err := time.Parse(formatoHora, hhmm)
	if err != nil {
		return time.Time{} // queda antes de la mínima: orden de horas roto
	}
	ini := a.pedido.Inicio.In(zona)
	t := time.Date(ini.Year(), ini.Month(), ini.Day(), h.Hour(), h.Minute(), 0, 0, zona)
	if ini.Sub(t) > 12*time.Hour {
		t = t.AddDate(0, 0, 1)
	}
	return t
}

func buscar(recs []recomendador.Recomendacion, id bson.ObjectID) (recomendador.Recomendacion, bool) {
	for _, r := range recs {
		if r.Negocio.ID == id {
			return r, true
		}
	}
	return recomendador.Recomendacion{}, false
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

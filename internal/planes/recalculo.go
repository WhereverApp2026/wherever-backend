package planes

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
)

// earthRadiusKm es el radio medio de la Tierra.
const earthRadiusKm = 6371.0

// StreetFactor aproxima las calles sobre la línea recta.
const StreetFactor = 1.3

// Velocidades en km/h por movilidad.
type Velocidades map[string]float64

// DefaultVelocidades son las de la spec: a pie 4,5; bici 12; auto 25; transporte 18.
func DefaultVelocidades() Velocidades {
	return Velocidades{MobilityWalk: 4.5, MobilityBike: 12, MobilityCar: 25, MobilityTransit: 18}
}

// Lugar es lo que Recalcular necesita de un negocio aprobado.
type Lugar struct {
	Lat, Lng float64
	Horario  []negocios.VentanaMin
}

// Motivos de una parada inválida.
const (
	ReasonUnavailable = "negocio_no_disponible"
	ReasonClosed      = "cerrado"
	ReasonOverlap     = "no_cabe"
	ReasonDuration    = "duracion"
	ReasonPrice       = "precio"
)

// Errores del plan completo.
var (
	ErrStopCount = errors.New("el plan debe tener de 1 a 6 paradas")
	ErrMobility  = errors.New("movilidad desconocida")
	ErrPeople    = errors.New("personas debe ser al menos 1")
)

// StopError dice qué parada (ya en orden por hora) no es válida y por qué. nv-30 la
// devuelve como 422.
type StopError struct {
	Index  int
	StopID bson.ObjectID
	Reason string
}

func (e *StopError) Error() string {
	return fmt.Sprintf("parada %d: %s", e.Index, e.Reason)
}

// Haversine es la distancia en línea recta en kilómetros.
func Haversine(lat1, lng1, lat2, lng2 float64) float64 {
	rad := math.Pi / 180
	dLat := (lat2 - lat1) * rad
	dLng := (lng2 - lng1) * rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * earthRadiusKm * math.Asin(math.Sqrt(a))
}

// Tramo estima kilómetros (2 decimales) y minutos (hacia arriba) entre dos lugares.
func Tramo(from, to Lugar, kmh float64) (float64, int) {
	km := math.Round(Haversine(from.Lat, from.Lng, to.Lat, to.Lng)*StreetFactor*100) / 100
	return km, int(math.Ceil(km / kmh * 60))
}

// Recalcular ordena las paradas por hora, valida cada una contra su negocio (aprobado y
// abierto en toda su franja), calcula los traslados y comprueba que ninguna parada empiece
// antes de que acabe la anterior más el traslado. Al final fija los costos. Es la única
// función que escribe `traslados`, `costo_total` y `costo_por_persona`. Si falla, el plan
// queda ordenado pero sin cambios en lo calculado.
func Recalcular(p *Plan, lugares map[bson.ObjectID]Lugar, v Velocidades) error {
	kmh, err := checkHeader(p, v)
	if err != nil {
		return err
	}
	sort.SliceStable(p.Paradas, func(i, j int) bool { return p.Paradas[i].Llega.Before(p.Paradas[j].Llega) })
	traslados := make([]Traslado, 0, len(p.Paradas)-1)
	perPerson := 0
	for i, s := range p.Paradas {
		lugar, err := checkStop(i, s, lugares)
		if err != nil {
			return err
		}
		if i > 0 {
			t, err := link(i, p.Paradas[i-1], s, lugares[p.Paradas[i-1].NegocioID], lugar, kmh)
			if err != nil {
				return err
			}
			traslados = append(traslados, t)
		}
		perPerson += s.Precio
	}
	p.Traslados = traslados
	p.CostoPorPersona = perPerson
	p.CostoTotal = perPerson * p.Personas
	return nil
}

func checkHeader(p *Plan, v Velocidades) (float64, error) {
	if len(p.Paradas) < MinStops || len(p.Paradas) > MaxStops {
		return 0, ErrStopCount
	}
	if p.Personas < 1 {
		return 0, ErrPeople
	}
	kmh, ok := v[p.Movilidad]
	if !ok || kmh <= 0 {
		return 0, ErrMobility
	}
	return kmh, nil
}

func checkStop(i int, s Parada, lugares map[bson.ObjectID]Lugar) (Lugar, error) {
	fail := func(reason string) (Lugar, error) {
		return Lugar{}, &StopError{Index: i, StopID: s.ID, Reason: reason}
	}
	if s.Minutos <= 0 {
		return fail(ReasonDuration)
	}
	if s.Precio < 0 {
		return fail(ReasonPrice)
	}
	lugar, ok := lugares[s.NegocioID]
	if !ok {
		return fail(ReasonUnavailable)
	}
	last := s.Llega.Add(time.Duration(s.Minutos-1) * time.Minute)
	if !negocios.OpenAt(lugar.Horario, s.Llega) || !negocios.OpenAt(lugar.Horario, last) {
		return fail(ReasonClosed)
	}
	return lugar, nil
}

func link(i int, prev, cur Parada, from, to Lugar, kmh float64) (Traslado, error) {
	km, minutes := Tramo(from, to, kmh)
	ready := prev.Llega.Add(time.Duration(prev.Minutos+minutes) * time.Minute)
	if cur.Llega.Before(ready) {
		return Traslado{}, &StopError{Index: i, StopID: cur.ID, Reason: ReasonOverlap}
	}
	return Traslado{Desde: prev.ID, Hasta: cur.ID, Minutos: minutes, Km: km}, nil
}

package negocios

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// zonaHoraria es la zona del Excel; la constitución la usa por defecto mientras no haya zona
// por ciudad. cmd/api importa time/tzdata para que la imagen no dependa del sistema.
const zonaHoraria = "America/Mexico_City"

const minutosDia = 24 * 60

// mexicoCity se carga una vez. Si la base de zonas falta, México no tiene horario de verano
// desde 2022 y UTC−6 da la misma hora.
var mexicoCity = func() *time.Location {
	loc, err := time.LoadLocation(zonaHoraria)
	if err != nil {
		return time.FixedZone("CST", -6*60*60)
	}
	return loc
}()

// MexicoCity es la zona de los horarios; planes (nv-30) la usa para la fecha del plan.
func MexicoCity() *time.Location { return mexicoCity }

// localMinute devuelve el día (0 = domingo) y el minuto desde las 00:00 en la hora de México.
func localMinute(now time.Time) (int, int) {
	local := now.In(mexicoCity)
	return int(local.Weekday()), local.Hour()*60 + local.Minute()
}

// yesterday es el día anterior con la convención de Go (0 = domingo).
func yesterday(day int) int { return (day + 6) % 7 }

// OpenAt dice si alguna ventana de `horario_min` cubre `now`: la de hoy con
// `desde ≤ m < hasta`, o la de ayer que cruza la medianoche con `hasta > m + 1440`.
func OpenAt(windows []VentanaMin, now time.Time) bool {
	day, minute := localMinute(now)
	prev := yesterday(day)
	for _, w := range windows {
		if w.Dia == day && w.Desde <= minute && minute < w.Hasta {
			return true
		}
		if w.Dia == prev && w.Hasta > minute+minutosDia {
			return true
		}
	}
	return false
}

// openAtFilter es la misma regla de OpenAt como filtro de MongoDB, para filtrar antes de
// paginar.
func openAtFilter(now time.Time) bson.E {
	day, minute := localMinute(now)
	return bson.E{Key: "$or", Value: bson.A{
		bson.D{{Key: "horario_min", Value: bson.D{{Key: "$elemMatch", Value: bson.D{
			{Key: "dia", Value: day},
			{Key: "desde", Value: bson.D{{Key: "$lte", Value: minute}}},
			{Key: "hasta", Value: bson.D{{Key: "$gt", Value: minute}}},
		}}}}},
		bson.D{{Key: "horario_min", Value: bson.D{{Key: "$elemMatch", Value: bson.D{
			{Key: "dia", Value: yesterday(day)},
			{Key: "hasta", Value: bson.D{{Key: "$gt", Value: minute + minutosDia}}},
		}}}}},
	}}
}

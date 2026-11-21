package insignias

import (
	"fmt"
	"time"
)

// mexicoCity es la zona de la racha (constitución: America/Mexico_City por omisión). cmd/api
// importa time/tzdata; si la base de zonas falta, México no tiene horario de verano desde 2022
// y UTC−6 da la misma hora.
var mexicoCity = func() *time.Location {
	loc, err := time.LoadLocation("America/Mexico_City")
	if err != nil {
		return time.FixedZone("CST", -6*60*60)
	}
	return loc
}()

// semana es una semana ISO.
type semana struct{ anio, num int }

func semanaDe(t time.Time) semana {
	y, w := t.ISOWeek()
	return semana{y, w}
}

// Racha cuenta las semanas ISO seguidas, hasta la de `ahora`, con al menos un día de visita
// (`dias` en formato AAAA-MM-DD, el `dia` de los check-ins válidos). Si la semana actual aún
// no tiene visita, cuenta hasta la anterior. Los días mal formados se ignoran.
func Racha(dias []string, ahora time.Time) int {
	con := make(map[semana]bool, len(dias))
	for _, d := range dias {
		t, err := time.ParseInLocation(time.DateOnly, d, mexicoCity)
		if err != nil {
			continue
		}
		con[semanaDe(t)] = true
	}
	// Mediodía evita que un cambio de hora mueva el día al restar semanas.
	hoy := ahora.In(mexicoCity)
	cursor := time.Date(hoy.Year(), hoy.Month(), hoy.Day(), 12, 0, 0, 0, mexicoCity)
	if !con[semanaDe(cursor)] {
		cursor = cursor.AddDate(0, 0, -7)
	}
	n := 0
	for con[semanaDe(cursor)] {
		n++
		cursor = cursor.AddDate(0, 0, -7)
	}
	return n
}

// semanaISO es la semana de `t` en México como texto («2026-W47»); la usan los puntos por plan.
func semanaISO(t time.Time) string {
	s := semanaDe(t.In(mexicoCity))
	return fmt.Sprintf("%d-W%02d", s.anio, s.num)
}

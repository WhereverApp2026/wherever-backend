package reservas

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
)

// Horario es una fila de la disponibilidad: la hora (UTC) y cuántos lugares quedan.
type Horario struct {
	Hora          time.Time `json:"hora"`
	Disponible    bool      `json:"disponible"`
	LugaresLibres int       `json:"lugares_libres"`
}

// minutos lee "HH:MM" (00:00 a 24:00); ok=false si no tiene esa forma.
func minutos(hhmm string) (int, bool) {
	h, m, found := strings.Cut(hhmm, ":")
	if !found {
		return 0, false
	}
	hh, err1 := strconv.Atoi(h)
	mm, err2 := strconv.Atoi(m)
	if err1 != nil || err2 != nil || hh < 0 || hh > 24 || mm < 0 || mm > 59 || hh*60+mm > 24*60 {
		return 0, false
	}
	return hh*60 + mm, true
}

// Turnos son las horas de inicio del día local `fecha` (solo año, mes y día cuentan), cada
// `turnoMin`, dentro de los tramos de `horarios` de ese día de la semana. Un turno debe
// terminar antes del cierre; un tramo que cruza la medianoche (cierra ≤ abre) sigue contando
// como del día en que abre. Devuelve instantes en UTC, ordenados y sin repetir.
func Turnos(horarios []negocios.Horario, turnoMin int, fecha time.Time, loc *time.Location) []time.Time {
	if turnoMin <= 0 {
		return nil
	}
	y, mo, d := fecha.Date()
	inicio := time.Date(y, mo, d, 0, 0, 0, 0, loc)
	dia := int(inicio.Weekday())
	vistos := map[time.Time]bool{}
	out := []time.Time{}
	for _, h := range horarios {
		if h.Dia != dia || h.Cerrado {
			continue
		}
		abre, ok1 := minutos(h.Abre)
		cierra, ok2 := minutos(h.Cierra)
		if !ok1 || !ok2 {
			continue
		}
		if cierra <= abre {
			cierra += 24 * 60
		}
		for m := abre; m+turnoMin <= cierra; m += turnoMin {
			t := inicio.Add(time.Duration(m) * time.Minute).UTC()
			if !vistos[t] {
				vistos[t] = true
				out = append(out, t)
			}
		}
	}
	slices.SortFunc(out, func(a, b time.Time) int { return a.Compare(b) })
	return out
}

// Bloqueado dice si algún bloqueo se cruza con el turno [hora, hora+turno).
func Bloqueado(bloqueos []Bloqueo, hora time.Time, turnoMin int) bool {
	fin := hora.Add(time.Duration(turnoMin) * time.Minute)
	for _, b := range bloqueos {
		if hora.Before(b.Hasta) && fin.After(b.Desde) {
			return true
		}
	}
	return false
}

// Ocupadas suma las personas que ocupan cada horario en `now` (apartados vencidos no cuentan).
func Ocupadas(rs []Reserva, now time.Time) map[time.Time]int {
	out := map[time.Time]int{}
	for _, r := range rs {
		if r.Ocupa(now) {
			out[r.FechaHora.UTC()] += r.Personas
		}
	}
	return out
}

// Disponibilidad arma las filas del día: lugares libres = capacidad − ocupadas; 0 si el turno
// ya empezó o lo cubre un bloqueo. `disponible` = alcanza para `personas`.
func Disponibilidad(turnos []time.Time, capacidad, turnoMin, personas int, ocupadas map[time.Time]int, bloqueos []Bloqueo, now time.Time) []Horario {
	out := make([]Horario, 0, len(turnos))
	for _, t := range turnos {
		libres := capacidad - ocupadas[t]
		if !t.After(now) || Bloqueado(bloqueos, t, turnoMin) || libres < 0 {
			libres = 0
		}
		out = append(out, Horario{Hora: t, Disponible: libres >= personas && personas > 0, LugaresLibres: libres})
	}
	return out
}

// EsTurno dice si `hora` es exactamente uno de los turnos.
func EsTurno(turnos []time.Time, hora time.Time) bool {
	for _, t := range turnos {
		if t.Equal(hora) {
			return true
		}
	}
	return false
}

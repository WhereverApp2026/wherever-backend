package reservas

import (
	"slices"
	"time"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
)

// Horario es una fila de la disponibilidad: la hora (UTC), cuántos lugares quedan, la afluencia
// de esa hora (`poco`, `moderado`, `pico`; null sin curva) y si es la hora sugerida (nv-88).
type Horario struct {
	Hora          time.Time `json:"hora"`
	Disponible    bool      `json:"disponible"`
	LugaresLibres int       `json:"lugares_libres"`
	Afluencia     *string   `json:"afluencia"`
	Sugerida      bool      `json:"sugerida"`
	valor         int       // afluencia 0–100; solo cuenta con Afluencia != nil
}

// VentanaSugerencia es qué tan lejos de la hora pedida busca la sugerencia.
const VentanaSugerencia = 2 * time.Hour

// minutos lee "HH:MM" (00:00 a 24:00); ok=false si no tiene esa forma.
func minutos(hhmm string) (int, bool) { return negocios.MinutosHorario(hhmm) }

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

// CupoTurno son los lugares del turno [hora, hora+turno): el cupo de cada hora local que toca
// (nv-88); un turno que cae en dos horas usa el menor.
func CupoTurno(cfg negocios.ReservasConfig, hora time.Time, turnoMin int, loc *time.Location) int {
	ini := hora.In(loc)
	fin := ini.Add(time.Duration(max(turnoMin, 1)) * time.Minute)
	y, mo, d := ini.Date()
	cupo := -1
	for t := time.Date(y, mo, d, ini.Hour(), 0, 0, 0, loc); t.Before(fin); t = t.Add(time.Hour) {
		if c := cfg.CupoEn(int(t.Weekday()), t.Hour()); cupo < 0 || c < cupo {
			cupo = c
		}
	}
	return max(cupo, 0)
}

// Disponibilidad arma las filas del día: lugares libres = cupo del turno − ocupadas; 0 si el
// turno ya empezó o lo cubre un bloqueo. `disponible` = alcanza para `personas`. La afluencia
// es la de la hora local en que empieza el turno, de la curva `afluencia` del negocio (nv-66).
func Disponibilidad(turnos []time.Time, b negocios.Business, cfg negocios.ReservasConfig, personas int, ocupadas map[time.Time]int, bloqueos []Bloqueo, now time.Time, loc *time.Location) []Horario {
	out := make([]Horario, 0, len(turnos))
	for _, t := range turnos {
		libres := CupoTurno(cfg, t, cfg.DuracionTurnoMin, loc) - ocupadas[t]
		if !t.After(now) || Bloqueado(bloqueos, t, cfg.DuracionTurnoMin) || libres < 0 {
			libres = 0
		}
		local := t.In(loc)
		nivel, valor := negocios.CrowdAt(b.Traffic, int(local.Weekday()), local.Hour())
		out = append(out, Horario{Hora: t, Disponible: libres >= personas && personas > 0, LugaresLibres: libres, Afluencia: nivel, valor: valor})
	}
	return out
}

// Sugerir marca `sugerida` en la hora disponible con menor afluencia: dentro de ±2 h de
// `pedida` si la hay, o en todo el día. Empate: la más cercana a la pedida y luego la más
// temprana. Sin curva de afluencia no sugiere nada.
func Sugerir(hs []Horario, pedida *time.Time) {
	var elegido *Horario
	for i := range hs {
		h := &hs[i]
		if !h.Disponible || h.Afluencia == nil || (pedida != nil && distancia(h.Hora, *pedida) > VentanaSugerencia) {
			continue
		}
		if elegido == nil || mejorQue(*h, *elegido, pedida) {
			elegido = h
		}
	}
	if elegido != nil {
		elegido.Sugerida = true
	}
}

// mejorQue: menor afluencia; si empatan, más cerca de la pedida (las filas van en orden, así
// que sin pedida gana la más temprana).
func mejorQue(a, b Horario, pedida *time.Time) bool {
	if a.valor != b.valor {
		return a.valor < b.valor
	}
	return pedida != nil && distancia(a.Hora, *pedida) < distancia(b.Hora, *pedida)
}

func distancia(a, b time.Time) time.Duration {
	if d := a.Sub(b); d >= 0 {
		return d
	}
	return b.Sub(a)
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

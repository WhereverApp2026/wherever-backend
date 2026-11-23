package negocios

import (
	"strconv"
	"strings"
)

// Cupo por hora y día de la semana (nv-88): el dueño fija los lugares de cada hora en
// `reservas_config.cupo_por_hora`; donde no lo fija aplica `capacidad_por_horario`.
const (
	// MaxLugaresHora es el tope de lugares de una hora (el mismo de la capacidad general).
	MaxLugaresHora = 500
	// MaxCupos es una entrada por hora de la semana: 7 × 24.
	MaxCupos = diasSemana * horasDia
)

// CupoHora son los lugares de la hora `hora` ("HH:00", hora local) del día `dia` (0 = domingo).
// `lugares: 0` cierra esa hora para reservas.
type CupoHora struct {
	Dia     int    `bson:"dia" json:"dia"`
	Hora    string `bson:"hora" json:"hora"`
	Lugares int    `bson:"lugares" json:"lugares"`
}

// HoraEntera lee "HH:00" (00 a 23); ok=false con otra forma.
func HoraEntera(hora string) (int, bool) {
	h, m, found := strings.Cut(hora, ":")
	if !found || len(h) != 2 || m != "00" {
		return 0, false
	}
	n, err := strconv.Atoi(h)
	if err != nil || n < 0 || n >= horasDia {
		return 0, false
	}
	return n, true
}

// MinutosHorario lee "HH:MM" de un tramo de `horarios` (00:00 a 24:00) en minutos del día;
// ok=false si no tiene esa forma.
func MinutosHorario(hhmm string) (int, bool) {
	h, m, found := strings.Cut(hhmm, ":")
	if !found {
		return 0, false
	}
	hh, err1 := strconv.Atoi(h)
	mm, err2 := strconv.Atoi(m)
	if err1 != nil || err2 != nil || hh < 0 || hh > horasDia || mm < 0 || mm > 59 || hh*60+mm > horasDia*60 {
		return 0, false
	}
	return hh*60 + mm, true
}

// FormatoHora es la hora `h` como "HH:00".
func FormatoHora(h int) string {
	if h < 10 {
		return "0" + strconv.Itoa(h) + ":00"
	}
	return strconv.Itoa(h) + ":00"
}

// CupoEn son los lugares de la hora `hora` del día `dia`: la entrada del dueño o, sin ella,
// `capacidad_por_horario`.
func (c ReservasConfig) CupoEn(dia, hora int) int {
	for _, e := range c.CupoPorHora {
		if h, ok := HoraEntera(e.Hora); ok && e.Dia == dia && h == hora {
			return e.Lugares
		}
	}
	return c.CapacidadPorHorario
}

// HorasAbiertas marca las horas de la semana que se cruzan con algún tramo de `horarios`. Un
// tramo que cruza la medianoche marca las horas de la madrugada en el día siguiente (el día en
// que de verdad caen), igual que la disponibilidad.
func HorasAbiertas(horarios []Horario) [diasSemana][horasDia]bool {
	var out [diasSemana][horasDia]bool
	for _, t := range horarios {
		if t.Cerrado || t.Dia < 0 || t.Dia >= diasSemana {
			continue
		}
		abre, ok1 := MinutosHorario(t.Abre)
		cierra, ok2 := MinutosHorario(t.Cierra)
		if !ok1 || !ok2 {
			continue
		}
		if cierra <= abre {
			cierra += horasDia * 60
		}
		for m := abre / 60 * 60; m < cierra; m += 60 {
			k := m / 60
			out[(t.Dia+k/horasDia)%diasSemana][k%horasDia] = true
		}
	}
	return out
}

// CrowdAt es el nivel de afluencia (`poco`, `moderado`, `pico`) de la hora `hora` del día `dia`
// con su valor 0–100. Sin curva 7 × 24 o con el día en ceros = nil, como en Explorar.
func CrowdAt(matrix [][]int, dia, hora int) (*string, int) {
	if len(matrix) != diasSemana || dia < 0 || dia >= diasSemana || len(matrix[dia]) != horasDia || hora < 0 || hora >= horasDia {
		return nil, 0
	}
	if !hasTraffic(matrix[dia]) {
		return nil, 0
	}
	v := matrix[dia][hora]
	level := crowdOf(v)
	return &level, v
}

func hasTraffic(day []int) bool {
	for _, v := range day {
		if v > 0 {
			return true
		}
	}
	return false
}

// crowdOf: `poco` (0–33), `moderado` (34–66) o `pico` (67–100).
func crowdOf(v int) string {
	switch {
	case v <= crowdLowMax:
		return CrowdLow
	case v <= crowdMedMax:
		return CrowdMedium
	}
	return CrowdPeak
}

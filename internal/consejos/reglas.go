package consejos

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// Pesos de cada regla: gana el candidato de mayor peso. Las reglas que leen estadísticas (franja,
// aspecto, reservas) piden al menos DiasMinimos días con actividad; las de conteo no.
const (
	pesoResenasBase   = 0.6  // + 0.08 por reseña sin responder, hasta 5
	pesoResenasPaso   = 0.08 //
	pesoAspectoBase   = 0.5  // + la caída en puntos (escala 1–5), hasta 1
	pesoFranjaFactor  = 0.9  // × el déficit de la franja (0.4–1)
	pesoCupon         = 0.55
	pesoReservas      = 0.5
	pesoPerfil        = 0.2
	minDeficitFranja  = 0.4 // la franja tiene al menos 40 % menos llegadas que el promedio
	minLlegadasFranja = 20  // llegadas en 28 días para hablar de franjas
	minCaidaAspecto   = 0.3 // puntos de caída de un aspecto
	minResenasPeriodo = 3   // reseñas en cada periodo para comparar aspectos
	minVistasReservas = 60  // vistas a la ficha en 28 días = «visitas altas»
	horasFranja       = 2
	franjasDia        = 24 / horasFranja
)

// Aspectos son las sumas de los cinco aspectos de nv-62 (como en `estadisticas_diarias`).
type Aspectos struct {
	Sabor         float64 `bson:"sabor"`
	Atencion      float64 `bson:"atencion"`
	Ambiente      float64 `bson:"ambiente"`
	Espera        float64 `bson:"espera"`
	PrecioCalidad float64 `bson:"precio_calidad"`
}

func (a Aspectos) valores() [5]float64 {
	return [5]float64{a.Sabor, a.Atencion, a.Ambiente, a.Espera, a.PrecioCalidad}
}

var nombresAspecto = [5]string{"sabor", "atención", "ambiente", "espera", "precio-calidad"}

// Dia son las cifras de un día del negocio en `estadisticas_diarias` (nv-74), sin personas.
type Dia struct {
	Fecha            string   `bson:"fecha"`
	VistasFicha      int      `bson:"vistas_ficha"`
	Reservas         int      `bson:"reservas"`
	Llegadas         int      `bson:"llegadas"`
	CuponesCanjeados int      `bson:"cupones_canjeados"`
	FavoritosNuevos  int      `bson:"favoritos_nuevos"`
	PorHora          []int    `bson:"por_hora"`
	ResenasDia       int      `bson:"resenas_dia"`
	AspectosSuma     Aspectos `bson:"aspectos_suma"`
}

func (d Dia) activo() bool {
	return d.VistasFicha+d.Reservas+d.Llegadas+d.CuponesCanjeados+d.FavoritosNuevos+d.ResenasDia > 0
}

// Datos es todo lo que mira el motor: agregados del negocio, nunca personas ni textos.
type Datos struct {
	Hoy time.Time // en la zona del negocio
	// Dias son los de los últimos 56 días (28 actuales y 28 anteriores, para los aspectos).
	Dias                []Dia
	ResenasSinRespuesta int
	CuponesSinCanjes    int
	AceptaReservas      bool
	PerfilIncompleto    bool
}

// Hecho es el candidato elegido con su contexto agregado: lo único que puede ver la IA.
type Hecho struct {
	Regla      string
	Accion     string
	Peso       float64
	DiaSemana  string
	Desde      int // hora local de inicio de la franja
	Hasta      int
	Porcentaje int
	Conteo     int
	Aspecto    string
	Caida      float64
}

var diasSemana = [7]string{"domingos", "lunes", "martes", "miércoles", "jueves", "viernes", "sábados"}

// Elegir arma los candidatos y devuelve el de mayor peso. Sin ninguno: completar el perfil si
// le falta, o ninguna acción.
func Elegir(d Datos) Hecho {
	mejor := Hecho{Regla: ReglaNinguna, Accion: AccionNinguna}
	if d.PerfilIncompleto {
		mejor = Hecho{Regla: ReglaPerfil, Accion: AccionCompletarPerfil, Peso: pesoPerfil}
	}
	for _, c := range candidatos(d) {
		if c.Peso > mejor.Peso {
			mejor = c
		}
	}
	return mejor
}

// candidatos son los hechos que aplican: los de conteo siempre y los de estadísticas solo con
// al menos DiasMinimos días activos.
func candidatos(d Datos) []Hecho {
	out := []Hecho{}
	if d.ResenasSinRespuesta > 0 {
		out = append(out, Hecho{Regla: ReglaResenasSinResp, Accion: AccionResponderResenas, Conteo: d.ResenasSinRespuesta,
			Peso: pesoResenasBase + pesoResenasPaso*float64(min(d.ResenasSinRespuesta, 5))})
	}
	if d.CuponesSinCanjes > 0 {
		out = append(out, Hecho{Regla: ReglaCuponSinCanjes, Accion: AccionEditarCupon, Conteo: d.CuponesSinCanjes, Peso: pesoCupon})
	}
	actual, anterior := partir(d.Dias, d.Hoy)
	if diasActivos(actual) < DiasMinimos {
		return out
	}
	if h, ok := franjaFloja(actual, d.Hoy); ok {
		out = append(out, h)
	}
	if h, ok := aspectoEnBaja(actual, anterior); ok {
		out = append(out, h)
	}
	if vistas := sumar(actual, func(x Dia) int { return x.VistasFicha }); !d.AceptaReservas && vistas >= minVistasReservas {
		out = append(out, Hecho{Regla: ReglaReservasApagadas, Accion: AccionConfigurarReservas, Conteo: vistas, Peso: pesoReservas})
	}
	return out
}

// partir separa los días de los últimos 28 (sin hoy, que va a medias) y los 28 anteriores.
func partir(dias []Dia, hoy time.Time) (actual, anterior []Dia) {
	y, m, dd := hoy.Date()
	base := time.Date(y, m, dd, 0, 0, 0, 0, time.UTC)
	corte := base.AddDate(0, 0, -DiasVentana).Format(FormatoDia)
	inicio := base.AddDate(0, 0, -2*DiasVentana).Format(FormatoDia)
	fin := base.Format(FormatoDia)
	for _, x := range dias {
		switch {
		case x.Fecha >= corte && x.Fecha < fin:
			actual = append(actual, x)
		case x.Fecha >= inicio && x.Fecha < corte:
			anterior = append(anterior, x)
		}
	}
	return actual, anterior
}

func diasActivos(dias []Dia) int {
	n := 0
	for _, x := range dias {
		if x.activo() {
			n++
		}
	}
	return n
}

func sumar(dias []Dia, f func(Dia) int) int {
	n := 0
	for _, x := range dias {
		n += f(x)
	}
	return n
}

type celdas [7][franjasDia]int

// celdasDe suma las llegadas por día de la semana y bloque de 2 h.
func celdasDe(dias []Dia) (c celdas, total int) {
	for _, x := range dias {
		f, err := time.Parse(FormatoDia, x.Fecha)
		if err != nil {
			continue
		}
		w := int(f.Weekday())
		for h, n := range x.PorHora {
			if h < 24 {
				c[w][h/horasFranja] += n
				total += n
			}
		}
	}
	return c, total
}

type celda struct{ w, b, v int }

// comparables son las celdas de franjas en que el negocio suele recibir gente: el bloque tuvo
// llegadas otro día de la semana y ese día tuvo llegadas. Así no compara contra horas cerradas.
func comparables(c celdas) []celda {
	var porDia [7]int
	var porFranja [franjasDia]int
	for w := range 7 {
		for b := range franjasDia {
			porDia[w] += c[w][b]
			porFranja[b] += c[w][b]
		}
	}
	var out []celda
	for w := range 7 {
		for b := range franjasDia {
			if porDia[w] > 0 && porFranja[b]-c[w][b] > 0 {
				out = append(out, celda{w, b, c[w][b]})
			}
		}
	}
	return out
}

// franjaFloja busca el día de la semana y bloque de 2 h con menos llegadas frente al promedio de
// las franjas comparables. En empate gana la más próxima a hoy, para que el consejo cambie.
func franjaFloja(dias []Dia, hoy time.Time) (Hecho, bool) {
	c, total := celdasDe(dias)
	lista := comparables(c)
	if total < minLlegadasFranja || len(lista) == 0 {
		return Hecho{}, false
	}
	suma := 0
	hoyW := int(hoy.Weekday())
	mejor := lista[0]
	for _, x := range lista {
		suma += x.v
		if x.v < mejor.v || (x.v == mejor.v && (x.w-hoyW+7)%7 < (mejor.w-hoyW+7)%7) {
			mejor = x
		}
	}
	if suma == 0 {
		return Hecho{}, false
	}
	deficit := 1 - float64(mejor.v)/(float64(suma)/float64(len(lista)))
	if deficit < minDeficitFranja {
		return Hecho{}, false
	}
	return Hecho{Regla: ReglaFranjaFloja, Accion: AccionCrearCupon, DiaSemana: diasSemana[mejor.w], Desde: mejor.b * horasFranja,
		Hasta: mejor.b*horasFranja + horasFranja, Porcentaje: int(math.Round(deficit * 100)), Peso: pesoFranjaFactor * math.Min(deficit, 1)}, true
}

// aspectoEnBaja compara el promedio de cada aspecto de los últimos 28 días con los 28 anteriores
// y toma el que más bajó.
func aspectoEnBaja(actual, anterior []Dia) (Hecho, bool) {
	na := sumar(actual, func(x Dia) int { return x.ResenasDia })
	np := sumar(anterior, func(x Dia) int { return x.ResenasDia })
	if na < minResenasPeriodo || np < minResenasPeriodo {
		return Hecho{}, false
	}
	sa, sp := sumaAspectos(actual), sumaAspectos(anterior)
	mejor, caida, previo := "", 0.0, 0.0
	for i := range sa {
		antes := sp[i] / float64(np)
		if c := antes - sa[i]/float64(na); c > caida {
			mejor, caida, previo = nombresAspecto[i], c, antes
		}
	}
	if mejor == "" || caida < minCaidaAspecto {
		return Hecho{}, false
	}
	caida = math.Round(caida*10) / 10
	return Hecho{Regla: ReglaAspectoEnBaja, Accion: AccionVerResenas, Aspecto: mejor, Caida: caida,
		Porcentaje: int(math.Round(caida / previo * 100)), Peso: math.Min(pesoAspectoBase+caida, 1)}, true
}

func sumaAspectos(dias []Dia) [5]float64 {
	var s [5]float64
	for _, x := range dias {
		v := x.AspectosSuma.valores()
		for i := range s {
			s[i] += v[i]
		}
	}
	return s
}

// Plantilla es el texto sin IA de cada regla (también el respaldo si la IA falla).
func Plantilla(h Hecho) string {
	switch h.Regla {
	case ReglaFranjaFloja:
		return fmt.Sprintf("Los %s de %d a %d tienes %d %% menos llegadas que en tus otras franjas. ¿Lanzas un cupón para ese horario?",
			h.DiaSemana, h.Desde, h.Hasta, h.Porcentaje)
	case ReglaAspectoEnBaja:
		return fmt.Sprintf("Tu calificación de %s bajó %.1f puntos frente al mes anterior. Revisa qué dicen tus reseñas.", h.Aspecto, h.Caida)
	case ReglaResenasSinResp:
		return fmt.Sprintf("Tienes %d %s sin responder de las últimas 2 semanas. Contestar genera confianza.", h.Conteo, plural(h.Conteo, "reseña", "reseñas"))
	case ReglaCuponSinCanjes:
		return fmt.Sprintf("%s sin canjes en 2 semanas. Prueba otro horario o condición.", plural(h.Conteo, "Tienes un cupón activo", fmt.Sprintf("Tienes %d cupones activos", h.Conteo)))
	case ReglaReservasApagadas:
		return fmt.Sprintf("Tu ficha tuvo %d visitas en 4 semanas y aún no aceptas reservas. ¿Las activas?", h.Conteo)
	case ReglaPerfil:
		return "Completa tu perfil con descripción y fotos para que más gente te encuentre."
	default:
		return "Todo en orden por hoy. Vuelve mañana para un nuevo consejo."
	}
}

func plural(n int, uno, varios string) string {
	if n == 1 {
		return uno
	}
	return varios
}

// Contexto es lo que viaja a la IA: categoría y el hecho agregado. Nunca el nombre del negocio,
// de clientes, ids ni textos de reseñas.
func Contexto(h Hecho, categoria string) string {
	partes := []string{"Categoría del negocio: " + categoria + ".", "Hecho: " + h.Regla + ".", "Acción: " + h.Accion + "."}
	switch h.Regla {
	case ReglaFranjaFloja:
		partes = append(partes, fmt.Sprintf("Día: %s. Franja: de %d a %d h. Llegadas: %d %% menos que el promedio de sus franjas en 4 semanas.", h.DiaSemana, h.Desde, h.Hasta, h.Porcentaje))
	case ReglaAspectoEnBaja:
		partes = append(partes, fmt.Sprintf("Aspecto: %s. Bajó %.1f puntos (%d %%) frente a las 4 semanas anteriores.", h.Aspecto, h.Caida, h.Porcentaje))
	case ReglaResenasSinResp:
		partes = append(partes, fmt.Sprintf("Reseñas sin responder en 2 semanas: %d.", h.Conteo))
	case ReglaCuponSinCanjes:
		partes = append(partes, fmt.Sprintf("Cupones activos sin canjes en 2 semanas: %d.", h.Conteo))
	case ReglaReservasApagadas:
		partes = append(partes, fmt.Sprintf("Visitas a la ficha en 4 semanas: %d. No acepta reservas.", h.Conteo))
	}
	return strings.Join(partes, " ")
}

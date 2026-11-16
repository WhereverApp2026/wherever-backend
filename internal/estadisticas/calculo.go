package estadisticas

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
)

// acumulado suma días de `estadisticas_diarias`.
type acumulado struct {
	vistas, reservas, llegadas, canjes, favoritos int
	resenas                                       int
	sumaCalificacion                              float64
	aspectos                                      Aspectos
	porHora                                       [camposPorHora]int
	origen                                        Origen
	activos                                       int
}

func (a *acumulado) sumar(d Diaria) {
	a.vistas += d.VistasFicha
	a.reservas += d.Reservas
	a.llegadas += d.Llegadas
	a.canjes += d.CuponesCanjeados
	a.favoritos += d.FavoritosNuevos
	if d.CalificacionDia != nil && d.ResenasDia > 0 {
		a.resenas += d.ResenasDia
		a.sumaCalificacion += *d.CalificacionDia * float64(d.ResenasDia)
		a.aspectos.sumar(d.AspectosSuma)
	}
	for h, n := range d.PorHora {
		if h < camposPorHora {
			a.porHora[h] += n
		}
	}
	a.origen.sumar(d.Origen)
	if d.activa() {
		a.activos++
	}
}

func redondear(x float64) float64 { return math.Round(x*10) / 10 }

func (a acumulado) calificacion() *float64 {
	if a.resenas == 0 {
		return nil
	}
	v := redondear(a.sumaCalificacion / float64(a.resenas))
	return &v
}

func (a acumulado) cifras() Cifras {
	return Cifras{VistasFicha: a.vistas, Reservas: a.reservas, Llegadas: a.llegadas, CuponesCanjeados: a.canjes, FavoritosNuevos: a.favoritos, Calificacion: a.calificacion()}
}

// promedioAspectos es el promedio de cada aspecto; nil sin reseñas.
func (a acumulado) promedioAspectos() *Aspectos {
	if a.resenas == 0 {
		return nil
	}
	p := a.aspectos.entre(float64(a.resenas))
	return &p
}

// Rango resuelve el periodo (días locales, incluidos): por omisión los 30 días hasta ayer.
// Fechas futuras, desde > hasta o más de 90 días = 422.
func Rango(q EstadisticasQuery, hoy time.Time) (desde, hasta time.Time, agrupar string, err error) {
	agrupar = q.Agrupar
	if agrupar == "" {
		agrupar = AgruparDia
	}
	hasta = hoy.AddDate(0, 0, -1)
	if q.Hasta != "" {
		hasta, _ = time.Parse(FormatoDia, q.Hasta)
	}
	desde = hasta.AddDate(0, 0, -(DiasPorOmision - 1))
	if q.Desde != "" {
		desde, _ = time.Parse(FormatoDia, q.Desde)
	}
	switch {
	case hasta.After(hoy):
		err = invalido("hasta", "la fecha no puede ser futura")
	case desde.After(hoy):
		err = invalido("desde", "la fecha no puede ser futura")
	case desde.After(hasta):
		err = invalido("desde", "desde debe ser anterior o igual a hasta")
	case dias(desde, hasta) > DiasMaximos:
		err = invalido("desde", fmt.Sprintf("el periodo es de %d días como máximo", DiasMaximos))
	}
	return desde, hasta, agrupar, err
}

// lunes es el lunes de la semana de d.
func lunes(d time.Time) time.Time {
	return d.AddDate(0, 0, -((int(d.Weekday()) + 6) % 7))
}

// serie arma un punto por día o por semana (lunes a domingo; la primera empieza en desde).
// Los días sin documento salen en cero.
func serie(porDia map[string]Diaria, desde, hasta time.Time, agrupar string) []Punto {
	out := []Punto{}
	var actual *acumulado
	inicio := ""
	cerrar := func() {
		if actual != nil {
			out = append(out, Punto{Inicio: inicio, Cifras: actual.cifras()})
		}
	}
	for d := desde; !d.After(hasta); d = d.AddDate(0, 0, 1) {
		clave := dia(d)
		if agrupar == AgruparSemana {
			l := lunes(d)
			if l.Before(desde) {
				l = desde
			}
			clave = dia(l)
		}
		if clave != inicio {
			cerrar()
			actual, inicio = &acumulado{}, clave
		}
		actual.sumar(porDia[dia(d)])
	}
	cerrar()
	return out
}

func porcentaje(actual, anterior int) *float64 {
	if anterior == 0 {
		return nil
	}
	v := redondear(float64(actual-anterior) / float64(anterior) * 100)
	return &v
}

func cambio(t, a Cifras) Cambio {
	return Cambio{
		VistasFicha:      porcentaje(t.VistasFicha, a.VistasFicha),
		Reservas:         porcentaje(t.Reservas, a.Reservas),
		Llegadas:         porcentaje(t.Llegadas, a.Llegadas),
		CuponesCanjeados: porcentaje(t.CuponesCanjeados, a.CuponesCanjeados),
		FavoritosNuevos:  porcentaje(t.FavoritosNuevos, a.FavoritosNuevos),
	}
}

// periodoAnterior nombra el periodo con el que se compara.
func periodoAnterior(n int, agrupar string) string {
	if agrupar == AgruparSemana && n%7 == 0 {
		if n == 7 {
			return "la semana anterior"
		}
		return fmt.Sprintf("las %d semanas anteriores", n/7)
	}
	if n == 1 {
		return "el día anterior"
	}
	return fmt.Sprintf("los %d días anteriores", n)
}

// Frase arma la frase sin IA: el cambio más grande frente al periodo anterior, o que aún no
// hay datos con menos de 7 días con actividad.
func Frase(c Cambio, diasActivos int, periodo string) string {
	if diasActivos < MinDiasFrase {
		return "Aún no hay datos suficientes para comparar."
	}
	candidatos := []struct {
		nombre string
		v      *float64
	}{
		{"visitas", c.VistasFicha}, {"reservas", c.Reservas}, {"llegadas", c.Llegadas},
		{"cupones canjeados", c.CuponesCanjeados}, {"favoritos nuevos", c.FavoritosNuevos},
	}
	nombre, mayor := "", 0.0
	for _, k := range candidatos {
		if k.v != nil && math.Abs(*k.v) > math.Abs(mayor) {
			nombre, mayor = k.nombre, *k.v
		}
	}
	if nombre == "" || math.Round(math.Abs(mayor)) == 0 {
		return "Tus cifras se mantienen " + frente(periodo) + "."
	}
	verbo := "subieron"
	if mayor < 0 {
		verbo = "bajaron"
	}
	return fmt.Sprintf("Tus %s %s %d %% %s.", nombre, verbo, int(math.Round(math.Abs(mayor))), frente(periodo))
}

// frente contrae «a el» en «al».
func frente(periodo string) string {
	if resto, ok := strings.CutPrefix(periodo, "el "); ok {
		return "frente al " + resto
	}
	return "frente a " + periodo
}

var nombresAspectos = []string{"sabor", "atencion", "ambiente", "espera", "precio_calidad"}

func valoresAspectos(a Aspectos) []float64 {
	return []float64{a.Sabor, a.Atencion, a.Ambiente, a.Espera, a.PrecioCalidad}
}

// comparar arma los cinco aspectos del negocio contra la categoría.
func comparar(tuyo, categoria *Aspectos) []AspectoComparado {
	out := make([]AspectoComparado, 0, len(nombresAspectos))
	for i, nombre := range nombresAspectos {
		c := AspectoComparado{Aspecto: nombre}
		if tuyo != nil {
			v := redondear(valoresAspectos(*tuyo)[i])
			c.Tuyo = &v
		}
		if categoria != nil {
			v := redondear(valoresAspectos(*categoria)[i])
			c.Categoria = &v
		}
		out = append(out, c)
	}
	return out
}

// periodos separa los días leídos en el periodo actual y el anterior.
type periodos struct {
	porDia            map[string]Diaria
	actual, anterior  acumulado
	actualizado       *time.Time
	desdeAct, hastaAt string
}

func repartir(rows []Diaria, desde, hasta time.Time) periodos {
	p := periodos{porDia: make(map[string]Diaria, len(rows)), desdeAct: dia(desde), hastaAt: dia(hasta)}
	for _, d := range rows {
		if d.Fecha >= p.desdeAct {
			if d.Fecha > p.hastaAt {
				continue
			}
			p.porDia[d.Fecha] = d
			p.actual.sumar(d)
			if p.actualizado == nil || d.ActualizadoEn.After(*p.actualizado) {
				t := d.ActualizadoEn.UTC()
				p.actualizado = &t
			}
			continue
		}
		p.anterior.sumar(d)
	}
	return p
}

// avanzadas arma las cifras premium; la categoría solo con 5 negocios o más con reseñas en el
// mes de `hasta`.
func (s *Service) avanzadas(ctx context.Context, b negocios.Business, a acumulado, hasta time.Time) (*Avanzadas, error) {
	cat, ok, err := s.repo.Categoria(ctx, b.Category, hasta.Format(FormatoMes))
	if err != nil {
		return nil, err
	}
	var promedio *Aspectos
	if ok && cat.Negocios >= MinNegociosCategoria {
		promedio = &cat.Aspectos
	}
	return &Avanzadas{PorHora: a.porHora[:], Origen: a.origen, Aspectos: comparar(a.promedioAspectos(), promedio)}, nil
}

// Estadisticas son las cifras del rango contra el periodo anterior de la misma longitud
// (US-144). Las avanzadas solo con plan premium; sin él, `avanzadas: null` y 200.
func (s *Service) Estadisticas(ctx context.Context, claims *middleware.Claims, raw string, q EstadisticasQuery) (EstadisticasResponse, error) {
	b, err := s.propio(ctx, claims, raw)
	if err != nil {
		return EstadisticasResponse{}, err
	}
	desde, hasta, agrupar, err := Rango(q, s.hoy())
	if err != nil {
		return EstadisticasResponse{}, err
	}
	n := dias(desde, hasta)
	rows, err := s.repo.Rango(ctx, b.ID, dia(desde.AddDate(0, 0, -n)), dia(hasta))
	if err != nil {
		return EstadisticasResponse{}, errStore()
	}
	p := repartir(rows, desde, hasta)
	totales, anterior := p.actual.cifras(), p.anterior.cifras()
	c := cambio(totales, anterior)
	out := EstadisticasResponse{
		Desde: dia(desde), Hasta: dia(hasta), Agrupar: agrupar, Serie: serie(p.porDia, desde, hasta, agrupar),
		Totales: totales, Anterior: anterior, Cambio: c, Frase: Frase(c, p.actual.activos, periodoAnterior(n, agrupar)),
		PremiumRequerido: b.Plan != negocios.PlanPremium, ActualizadoEn: p.actualizado,
	}
	if !out.PremiumRequerido {
		if out.Avanzadas, err = s.avanzadas(ctx, b, p.actual, hasta); err != nil {
			return EstadisticasResponse{}, errStore()
		}
	}
	return out, nil
}

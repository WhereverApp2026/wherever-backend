package estadisticas

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/WhereverApp2026/wherever-backend/internal/estadisticas/pdf"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
)

var meses = []string{"enero", "febrero", "marzo", "abril", "mayo", "junio", "julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre"}

// NombreMes da «noviembre de 2026».
func NombreMes(m time.Time) string { return fmt.Sprintf("%s de %d", meses[m.Month()-1], m.Year()) }

func inicioMes(d time.Time) time.Time { return time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, time.UTC) }

// MesReporte valida `mes`: el actual o uno cerrado de los últimos 12, no anterior al alta.
func MesReporte(raw string, hoy, alta time.Time) (time.Time, error) {
	mes, _ := time.Parse(FormatoMes, raw)
	actual := inicioMes(hoy)
	switch {
	case mes.After(actual):
		return mes, invalido("mes", "el mes no puede ser futuro")
	case mes.Before(actual.AddDate(0, -(MesesReporte - 1), 0)):
		return mes, invalido("mes", fmt.Sprintf("solo los últimos %d meses", MesesReporte))
	case mes.Before(inicioMes(alta)):
		return mes, invalido("mes", "el negocio aún no estaba dado de alta")
	}
	return mes, nil
}

// datosReporte es lo que va al PDF; nunca lleva datos de clientes.
type datosReporte struct {
	negocio            string
	mes                string
	parcial            string
	totales, anterior  Cifras
	cambio             Cambio
	semanas            []Punto
	top                []TopCupon
	resenas            int
	calificacion       *float64
	aspectos           *Aspectos
	frase              string
	avanzadas          *Avanzadas
	premiumDisponibles bool
}

// Reporte arma el PDF del mes (US-144) y su nombre de archivo.
func (s *Service) Reporte(ctx context.Context, claims *middleware.Claims, raw string, q ReporteQuery) ([]byte, string, error) {
	b, err := s.propio(ctx, claims, raw)
	if err != nil {
		return nil, "", err
	}
	hoy := s.hoy()
	mes, err := MesReporte(q.Mes, hoy, fecha(b.CreatedAt, s.zona))
	if err != nil {
		return nil, "", err
	}
	hasta := mes.AddDate(0, 1, -1)
	parcial := ""
	if hasta.After(hoy) {
		hasta = hoy
		parcial = "hasta el " + strconv.Itoa(hoy.Day())
	}
	previo := mes.AddDate(0, -1, 0)
	rows, err := s.repo.Rango(ctx, b.ID, dia(previo), dia(hasta))
	if err != nil {
		return nil, "", errStore()
	}
	p := repartir(rows, mes, hasta)
	top, err := s.repo.TopCupones(ctx, b.ID, dia(mes), dia(hasta))
	if err != nil {
		return nil, "", errStore()
	}
	d := datosReporte{negocio: b.Name, mes: NombreMes(mes), parcial: parcial, totales: p.actual.cifras(), anterior: p.anterior.cifras(),
		semanas: serie(p.porDia, mes, hasta, AgruparSemana), top: top, resenas: p.actual.resenas, calificacion: p.actual.calificacion(),
		aspectos: p.actual.promedioAspectos(), premiumDisponibles: b.Plan == negocios.PlanPremium}
	d.cambio = cambio(d.totales, d.anterior)
	d.frase = Frase(d.cambio, p.actual.activos, "el mes anterior")
	if d.premiumDisponibles {
		if d.avanzadas, err = s.avanzadas(ctx, b, p.actual, hasta); err != nil {
			return nil, "", errStore()
		}
	}
	return armarPDF(d), "wherever-" + mes.Format(FormatoMes) + ".pdf", nil
}

func entero(n int) string { return strconv.Itoa(n) }

func decimal(v *float64) string {
	if v == nil {
		return "—"
	}
	return strconv.FormatFloat(*v, 'f', 1, 64)
}

func porciento(v *float64) string {
	if v == nil {
		return "—"
	}
	s := strconv.FormatFloat(*v, 'f', 1, 64) + " %"
	if *v > 0 {
		s = "+" + s
	}
	return s
}

var etiquetasAspectos = map[string]string{"sabor": "Sabor", "atencion": "Atención", "ambiente": "Ambiente", "espera": "Espera", "precio_calidad": "Precio-calidad"}

// armarPDF escribe el reporte en una a dos hojas.
func armarPDF(d datosReporte) []byte {
	doc := pdf.Nuevo()
	doc.Titulo(d.negocio)
	mes := "Reporte mensual de " + d.mes
	if d.parcial != "" {
		mes += " (" + d.parcial + ")"
	}
	doc.Parrafo(mes)
	doc.Parrafo(d.frase)

	doc.Subtitulo("Totales del mes")
	filas := [][]string{
		{"Visitas a la ficha", entero(d.totales.VistasFicha), entero(d.anterior.VistasFicha), porciento(d.cambio.VistasFicha)},
		{"Reservas", entero(d.totales.Reservas), entero(d.anterior.Reservas), porciento(d.cambio.Reservas)},
		{"Llegadas", entero(d.totales.Llegadas), entero(d.anterior.Llegadas), porciento(d.cambio.Llegadas)},
		{"Cupones canjeados", entero(d.totales.CuponesCanjeados), entero(d.anterior.CuponesCanjeados), porciento(d.cambio.CuponesCanjeados)},
		{"Favoritos nuevos", entero(d.totales.FavoritosNuevos), entero(d.anterior.FavoritosNuevos), porciento(d.cambio.FavoritosNuevos)},
	}
	doc.Tabla([]float64{180, 100, 110, 100}, []string{"Cifra", "Este mes", "Mes anterior", "Cambio"}, filas)

	doc.Subtitulo("Por semana")
	semanas := make([][]string, 0, len(d.semanas))
	for _, s := range d.semanas {
		semanas = append(semanas, []string{s.Inicio, entero(s.VistasFicha), entero(s.Reservas), entero(s.Llegadas), entero(s.CuponesCanjeados), entero(s.FavoritosNuevos)})
	}
	doc.Tabla([]float64{100, 75, 75, 75, 85, 85}, []string{"Desde", "Visitas", "Reservas", "Llegadas", "Cupones", "Favoritos"}, semanas)

	doc.Subtitulo("Cupones con más canjes")
	if len(d.top) == 0 {
		doc.Parrafo("Sin canjes este mes.")
	} else {
		top := make([][]string, 0, len(d.top))
		for _, c := range d.top {
			top = append(top, []string{c.Titulo, entero(c.Canjes)})
		}
		doc.Tabla([]float64{380, 100}, []string{"Cupón", "Canjes"}, top)
	}

	doc.Subtitulo("Calificación del mes")
	if d.calificacion == nil {
		doc.Parrafo("Sin reseñas publicadas este mes.")
	} else {
		doc.Parrafo(fmt.Sprintf("Promedio %s de 5 con %d reseñas.", decimal(d.calificacion), d.resenas))
		aspectos := make([][]string, 0, len(nombresAspectos))
		for i, nombre := range nombresAspectos {
			v := valoresAspectos(*d.aspectos)[i]
			aspectos = append(aspectos, []string{etiquetasAspectos[nombre], decimal(&v)})
		}
		doc.Tabla([]float64{180, 100}, []string{"Aspecto", "Promedio"}, aspectos)
	}

	if d.avanzadas != nil {
		escribirAvanzadas(doc, d.avanzadas)
	}
	doc.Subtitulo("Nota")
	doc.Parrafo("Cifras anónimas: el reporte no incluye nombres, correos ni comentarios de clientes. Los días se cuentan en hora del centro de México.")
	return doc.Bytes()
}

// escribirAvanzadas agrega las cifras premium: horas con más llegadas, origen de las visitas y
// aspectos contra la categoría.
func escribirAvanzadas(doc *pdf.Documento, a *Avanzadas) {
	doc.Subtitulo("Horas con más llegadas")
	horas := make([]int, 0, len(a.PorHora))
	for h, n := range a.PorHora {
		if n > 0 {
			horas = append(horas, h)
		}
	}
	slices.SortStableFunc(horas, func(x, y int) int { return a.PorHora[y] - a.PorHora[x] })
	if len(horas) == 0 {
		doc.Parrafo("Sin llegadas registradas este mes.")
	} else {
		filas := [][]string{}
		for _, h := range horas[:min(3, len(horas))] {
			filas = append(filas, []string{fmt.Sprintf("%02d:00 a %02d:59", h, h), entero(a.PorHora[h])})
		}
		doc.Tabla([]float64{180, 100}, []string{"Hora", "Llegadas"}, filas)
	}
	doc.Subtitulo("Origen de las visitas")
	doc.Tabla([]float64{180, 100}, []string{"Origen", "Visitas"}, [][]string{
		{"Búsqueda", entero(a.Origen.Busqueda)}, {"Mapa", entero(a.Origen.Mapa)}, {"Inicio", entero(a.Origen.Inicio)},
		{"Plan con IA", entero(a.Origen.PlanIA)}, {"Cupón", entero(a.Origen.Cupon)},
	})
	doc.Subtitulo("Aspectos contra tu categoría")
	filas := make([][]string, 0, len(a.Aspectos))
	for _, c := range a.Aspectos {
		filas = append(filas, []string{etiquetasAspectos[c.Aspecto], decimal(c.Tuyo), decimal(c.Categoria)})
	}
	doc.Tabla([]float64{180, 100, 120}, []string{"Aspecto", "Tuyo", "Categoría"}, filas)
}

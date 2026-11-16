package estadisticas

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

func d(s string) time.Time {
	t, _ := time.Parse(FormatoDia, s)
	return t
}

func campo(t *testing.T, err error) string {
	t.Helper()
	var e *apperr.Error
	require.ErrorAs(t, err, &e)
	require.Equal(t, apperr.KindValidation, e.Kind)
	require.Len(t, e.Fields, 1)
	return e.Fields[0].Field
}

func TestRango_PorOmisionYLimites(t *testing.T) {
	hoy := d("2026-11-16")
	desde, hasta, agrupar, err := Rango(EstadisticasQuery{}, hoy)
	require.NoError(t, err)
	assert.Equal(t, "2026-10-17", dia(desde))
	assert.Equal(t, "2026-11-15", dia(hasta), "termina ayer")
	assert.Equal(t, AgruparDia, agrupar)
	assert.Equal(t, 30, dias(desde, hasta))

	_, hasta, _, err = Rango(EstadisticasQuery{Desde: "2026-11-01", Hasta: "2026-11-16", Agrupar: AgruparSemana}, hoy)
	require.NoError(t, err, "hoy se permite")
	assert.Equal(t, "2026-11-16", dia(hasta))

	_, _, _, err = Rango(EstadisticasQuery{Desde: "2026-08-19", Hasta: "2026-11-16"}, hoy)
	require.NoError(t, err, "90 días exactos")
	_, _, _, err = Rango(EstadisticasQuery{Desde: "2026-08-18", Hasta: "2026-11-16"}, hoy)
	assert.Equal(t, "desde", campo(t, err), "91 días")
	_, _, _, err = Rango(EstadisticasQuery{Hasta: "2026-11-17"}, hoy)
	assert.Equal(t, "hasta", campo(t, err), "futura")
	_, _, _, err = Rango(EstadisticasQuery{Desde: "2026-11-17", Hasta: "2026-11-16"}, hoy)
	assert.Equal(t, "desde", campo(t, err))
	_, _, _, err = Rango(EstadisticasQuery{Desde: "2026-11-10", Hasta: "2026-11-09"}, hoy)
	assert.Equal(t, "desde", campo(t, err), "desde > hasta")
}

func TestSerie_PorDiaYPorSemana(t *testing.T) {
	cal := 4.0
	porDia := map[string]Diaria{
		"2026-11-01": {Fecha: "2026-11-01", VistasFicha: 3, Reservas: 1}, // domingo
		"2026-11-02": {Fecha: "2026-11-02", VistasFicha: 2, ResenasDia: 2, CalificacionDia: &cal},
		"2026-11-09": {Fecha: "2026-11-09", Llegadas: 5},
	}
	dias := serie(porDia, d("2026-11-01"), d("2026-11-10"), AgruparDia)
	require.Len(t, dias, 10)
	assert.Equal(t, "2026-11-01", dias[0].Inicio)
	assert.Equal(t, 3, dias[0].VistasFicha)
	assert.Zero(t, dias[2].VistasFicha, "día sin documento en cero")
	assert.Nil(t, dias[0].Calificacion)
	require.NotNil(t, dias[1].Calificacion)
	assert.InDelta(t, 4.0, *dias[1].Calificacion, 0.001)

	semanas := serie(porDia, d("2026-11-01"), d("2026-11-10"), AgruparSemana)
	require.Len(t, semanas, 3, "domingo suelto, semana completa del 2 al 8 y lunes 9-10")
	assert.Equal(t, []string{"2026-11-01", "2026-11-02", "2026-11-09"}, []string{semanas[0].Inicio, semanas[1].Inicio, semanas[2].Inicio})
	assert.Equal(t, 3, semanas[0].VistasFicha)
	assert.Equal(t, 2, semanas[1].VistasFicha)
	assert.Equal(t, 5, semanas[2].Llegadas)
}

func TestCambio_AnteriorEnCeroEsNull(t *testing.T) {
	c := cambio(Cifras{VistasFicha: 118, Reservas: 3, Llegadas: 0}, Cifras{VistasFicha: 100, Reservas: 0, Llegadas: 4})
	require.NotNil(t, c.VistasFicha)
	assert.InDelta(t, 18.0, *c.VistasFicha, 0.001)
	assert.Nil(t, c.Reservas)
	require.NotNil(t, c.Llegadas)
	assert.InDelta(t, -100.0, *c.Llegadas, 0.001)
}

func TestFrase_PorReglas(t *testing.T) {
	sube, baja := 18.2, -40.0
	assert.Equal(t, "Aún no hay datos suficientes para comparar.", Frase(Cambio{VistasFicha: &sube}, 6, "los 30 días anteriores"))
	assert.Equal(t, "Tus visitas subieron 18 % frente a las 4 semanas anteriores.",
		Frase(Cambio{VistasFicha: &sube}, 7, periodoAnterior(28, AgruparSemana)))
	assert.Equal(t, "Tus reservas bajaron 40 % frente a los 30 días anteriores.",
		Frase(Cambio{VistasFicha: &sube, Reservas: &baja}, 20, periodoAnterior(30, AgruparDia)))
	assert.Equal(t, "Tus cifras se mantienen frente al mes anterior.", Frase(Cambio{}, 10, "el mes anterior"))
	assert.Equal(t, "la semana anterior", periodoAnterior(7, AgruparSemana))
	assert.Equal(t, "los 10 días anteriores", periodoAnterior(10, AgruparSemana))
	assert.Equal(t, "el día anterior", periodoAnterior(1, AgruparDia))
}

func TestRepartir_PeriodoAnteriorYActualizado(t *testing.T) {
	t1 := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Hour)
	p := repartir([]Diaria{
		{Fecha: "2026-11-05", VistasFicha: 4, ActualizadoEn: t2},
		{Fecha: "2026-11-11", VistasFicha: 2, ActualizadoEn: t1},
		{Fecha: "2026-11-12", VistasFicha: 1, ActualizadoEn: t2},
	}, d("2026-11-10"), d("2026-11-12"))
	assert.Equal(t, 3, p.actual.vistas)
	assert.Equal(t, 4, p.anterior.vistas)
	assert.Equal(t, 2, p.actual.activos)
	require.NotNil(t, p.actualizado)
	assert.Equal(t, t2, *p.actualizado)
}

func TestComparar_CategoriaNull(t *testing.T) {
	a := acumulado{resenas: 2, aspectos: Aspectos{Sabor: 9, Atencion: 8, Ambiente: 7, Espera: 6, PrecioCalidad: 10}}
	out := comparar(a.promedioAspectos(), nil)
	require.Len(t, out, 5)
	assert.Equal(t, "sabor", out[0].Aspecto)
	require.NotNil(t, out[0].Tuyo)
	assert.InDelta(t, 4.5, *out[0].Tuyo, 0.001)
	assert.Nil(t, out[0].Categoria)
	assert.Nil(t, comparar(acumulado{}.promedioAspectos(), nil)[0].Tuyo, "sin reseñas")
}

func TestMesReporte(t *testing.T) {
	hoy, alta := d("2026-11-16"), d("2026-01-20")
	mes, err := MesReporte("2026-11", hoy, alta)
	require.NoError(t, err, "el mes en curso")
	assert.Equal(t, "noviembre de 2026", NombreMes(mes))
	_, err = MesReporte("2026-01", hoy, alta)
	require.NoError(t, err, "el del alta")
	_, err = MesReporte("2026-12", hoy, alta)
	assert.Equal(t, "mes", campo(t, err), "futuro")
	_, err = MesReporte("2025-11", hoy, d("2025-01-01"))
	assert.Equal(t, "mes", campo(t, err), "hace 12 meses")
	_, err = MesReporte("2025-12", hoy, d("2025-01-01"))
	require.NoError(t, err, "hace 11 meses")
	_, err = MesReporte("2025-12", hoy, alta)
	assert.Equal(t, "mes", campo(t, err), "antes del alta")
}

func TestArmarPDF_SinDatosDeClientes(t *testing.T) {
	cal := 4.5
	b := armarPDF(datosReporte{negocio: "Fonda Doña Lupe", mes: "octubre de 2026", totales: Cifras{VistasFicha: 120, Reservas: 8},
		semanas: []Punto{{Inicio: "2026-10-01"}}, top: []TopCupon{{Titulo: "2x1 en café", Canjes: 7}}, resenas: 2, calificacion: &cal,
		aspectos: &Aspectos{Sabor: 5, Atencion: 4, Ambiente: 4, Espera: 3, PrecioCalidad: 5}, frase: "Tus visitas subieron 18 % frente al mes anterior."})
	assert.Contains(t, string(b), "%PDF-1.4")
	assert.Contains(t, string(b), "Fonda Do\xf1a Lupe")
	assert.Contains(t, string(b), "2x1 en caf\xe9")
}

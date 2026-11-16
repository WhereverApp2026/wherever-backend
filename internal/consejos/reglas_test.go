package consejos

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// hoy es un jueves (2026-11-19) en la zona del negocio.
var hoy = time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)

// dias arma n días hacia atrás desde ayer con la función dada.
func dias(n int, f func(fecha time.Time) Dia) []Dia {
	out := make([]Dia, 0, n)
	for i := 1; i <= n; i++ {
		fecha := hoy.AddDate(0, 0, -i)
		d := f(fecha)
		d.Fecha = fecha.Format(FormatoDia)
		out = append(out, d)
	}
	return out
}

// conLlegadas: 2 llegadas en cada bloque de 10 a 20 h, salvo los martes de 16 a 18.
func conLlegadas(fecha time.Time) Dia {
	h := make([]int, 24)
	for x := 10; x < 20; x++ {
		h[x] = 2
	}
	if fecha.Weekday() == time.Tuesday {
		h[16], h[17] = 0, 0
	}
	return Dia{VistasFicha: 5, Llegadas: 20, PorHora: h}
}

func TestElegir_FranjaFloja(t *testing.T) {
	h := Elegir(Datos{Hoy: hoy, Dias: dias(28, conLlegadas), AceptaReservas: true})

	assert.Equal(t, ReglaFranjaFloja, h.Regla)
	assert.Equal(t, AccionCrearCupon, h.Accion)
	assert.Equal(t, "martes", h.DiaSemana)
	assert.Equal(t, 16, h.Desde)
	assert.Equal(t, 18, h.Hasta)
	assert.Equal(t, 100, h.Porcentaje)
	assert.InDelta(t, pesoFranjaFactor, h.Peso, 1e-9)
	assert.Equal(t, "Los martes de 16 a 18 tienes 100 % menos llegadas que en tus otras franjas. ¿Lanzas un cupón para ese horario?", Plantilla(h))
}

func TestElegir_FranjaParejaNoEsCandidata(t *testing.T) {
	parejo := func(time.Time) Dia {
		h := make([]int, 24)
		for x := 10; x < 20; x++ {
			h[x] = 2
		}
		return Dia{Llegadas: 20, PorHora: h}
	}
	h := Elegir(Datos{Hoy: hoy, Dias: dias(28, parejo), AceptaReservas: true})

	assert.Equal(t, ReglaNinguna, h.Regla, "sin franja floja ni otros hechos")
	assert.Equal(t, AccionNinguna, h.Accion)
}

func TestElegir_AspectoEnBaja(t *testing.T) {
	// 4 semanas anteriores: atención 5; últimas 4: atención 4. Una reseña por día.
	todo := dias(56, func(fecha time.Time) Dia {
		atencion := 4.0
		if fecha.Before(hoy.AddDate(0, 0, -DiasVentana)) {
			atencion = 5
		}
		return Dia{ResenasDia: 1, VistasFicha: 1, AspectosSuma: Aspectos{Sabor: 4, Atencion: atencion, Ambiente: 4, Espera: 4, PrecioCalidad: 4}}
	})
	h := Elegir(Datos{Hoy: hoy, Dias: todo, AceptaReservas: true})

	assert.Equal(t, ReglaAspectoEnBaja, h.Regla)
	assert.Equal(t, AccionVerResenas, h.Accion)
	assert.Equal(t, "atención", h.Aspecto)
	assert.InDelta(t, 1.0, h.Caida, 1e-9)
	assert.Equal(t, 20, h.Porcentaje)
	assert.InDelta(t, 1.0, h.Peso, 1e-9, "0.5 + 1 punto, tope 1")
	assert.Contains(t, Plantilla(h), "atención bajó 1.0 puntos")
}

func TestElegir_AspectoSinReseñasSuficientesNoCuenta(t *testing.T) {
	todo := dias(56, func(fecha time.Time) Dia {
		if fecha.Before(hoy.AddDate(0, 0, -3)) {
			return Dia{VistasFicha: 1}
		}
		return Dia{VistasFicha: 1, ResenasDia: 1, AspectosSuma: Aspectos{Atencion: 1}}
	})
	h := Elegir(Datos{Hoy: hoy, Dias: todo, AceptaReservas: true})

	assert.Equal(t, ReglaNinguna, h.Regla)
}

func TestElegir_ResenasSinRespuestaYPeso(t *testing.T) {
	h := Elegir(Datos{Hoy: hoy, ResenasSinRespuesta: 2})
	assert.Equal(t, ReglaResenasSinResp, h.Regla)
	assert.Equal(t, AccionResponderResenas, h.Accion)
	assert.InDelta(t, 0.76, h.Peso, 1e-9)
	assert.Equal(t, "Tienes 2 reseñas sin responder de las últimas 2 semanas. Contestar genera confianza.", Plantilla(h))

	assert.InDelta(t, 1.0, Elegir(Datos{Hoy: hoy, ResenasSinRespuesta: 9}).Peso, 1e-9, "tope en 5 reseñas")
	assert.Contains(t, Plantilla(Elegir(Datos{Hoy: hoy, ResenasSinRespuesta: 1})), "1 reseña sin")
}

func TestElegir_CuponSinCanjes(t *testing.T) {
	h := Elegir(Datos{Hoy: hoy, CuponesSinCanjes: 1})
	assert.Equal(t, ReglaCuponSinCanjes, h.Regla)
	assert.Equal(t, AccionEditarCupon, h.Accion)
	assert.Equal(t, "Tienes un cupón activo sin canjes en 2 semanas. Prueba otro horario o condición.", Plantilla(h))
	assert.Contains(t, Plantilla(Elegir(Datos{Hoy: hoy, CuponesSinCanjes: 3})), "3 cupones activos")
}

func TestElegir_ReservasApagadasConVisitasAltas(t *testing.T) {
	visitas := dias(28, func(time.Time) Dia { return Dia{VistasFicha: 3} })
	h := Elegir(Datos{Hoy: hoy, Dias: visitas})
	assert.Equal(t, ReglaReservasApagadas, h.Regla)
	assert.Equal(t, AccionConfigurarReservas, h.Accion)
	assert.Equal(t, 84, h.Conteo)

	assert.Equal(t, ReglaNinguna, Elegir(Datos{Hoy: hoy, Dias: visitas, AceptaReservas: true}).Regla, "con reservas no aplica")
	pocas := dias(28, func(time.Time) Dia { return Dia{VistasFicha: 1} })
	assert.Equal(t, ReglaNinguna, Elegir(Datos{Hoy: hoy, Dias: pocas}).Regla, "28 visitas no son altas")
}

func TestElegir_SinDatosSuficientes(t *testing.T) {
	seis := dias(6, func(time.Time) Dia { return Dia{VistasFicha: 50} })
	assert.Equal(t, ReglaPerfil, Elegir(Datos{Hoy: hoy, Dias: seis, PerfilIncompleto: true}).Regla, "< 7 días: sin reglas de estadísticas")
	assert.Equal(t, AccionCompletarPerfil, Elegir(Datos{Hoy: hoy, PerfilIncompleto: true}).Accion)
	assert.Equal(t, AccionNinguna, Elegir(Datos{Hoy: hoy}).Accion)
	assert.Equal(t, ReglaResenasSinResp, Elegir(Datos{Hoy: hoy, ResenasSinRespuesta: 1, PerfilIncompleto: true}).Regla, "los conteos ganan al perfil")
}

func TestElegir_GanaElDeMayorPeso(t *testing.T) {
	h := Elegir(Datos{Hoy: hoy, Dias: dias(28, conLlegadas), ResenasSinRespuesta: 5, CuponesSinCanjes: 2, AceptaReservas: true})
	assert.Equal(t, ReglaResenasSinResp, h.Regla, "5 reseñas (1.0) > franja (0.9) > cupón (0.55)")

	h = Elegir(Datos{Hoy: hoy, Dias: dias(28, conLlegadas), ResenasSinRespuesta: 1, CuponesSinCanjes: 2, AceptaReservas: true})
	assert.Equal(t, ReglaFranjaFloja, h.Regla, "franja (0.9) > 1 reseña (0.68)")
}

func TestElegir_HoyNoCuenta(t *testing.T) {
	hoyMismo := []Dia{{Fecha: hoy.Format(FormatoDia), VistasFicha: 500}}
	assert.Equal(t, ReglaNinguna, Elegir(Datos{Hoy: hoy, Dias: hoyMismo}).Regla)
}

func TestContexto_SoloAgregados(t *testing.T) {
	h := Elegir(Datos{Hoy: hoy, Dias: dias(28, conLlegadas), AceptaReservas: true})
	c := Contexto(h, "restaurante")

	assert.Equal(t, "Categoría del negocio: restaurante. Hecho: franja_floja. Acción: crear_cupon. Día: martes. Franja: de 16 a 18 h. Llegadas: 100 % menos que el promedio de sus franjas en 4 semanas.", c)
	for _, regla := range []Hecho{{Regla: ReglaAspectoEnBaja, Aspecto: "sabor", Caida: 0.5, Porcentaje: 10}, {Regla: ReglaResenasSinResp, Conteo: 2},
		{Regla: ReglaCuponSinCanjes, Conteo: 1}, {Regla: ReglaReservasApagadas, Conteo: 80}, {Regla: ReglaPerfil}, {Regla: ReglaNinguna}} {
		assert.NotEmpty(t, Contexto(regla, "café"))
		assert.NotEmpty(t, Plantilla(regla))
	}
}

func TestSalidaConsejo_Validar(t *testing.T) {
	ok := salidaConsejo{Texto: "  Los martes   de 4 a 6 tienes poca gente. ¿Lanzas un cupón?  ", Accion: AccionCrearCupon}
	require.NoError(t, ok.Validar())
	assert.Equal(t, "Los martes de 4 a 6 tienes poca gente. ¿Lanzas un cupón?", ok.Texto)

	for nombre, s := range map[string]salidaConsejo{
		"corto":         {Texto: "Hola", Accion: AccionNinguna},
		"largo":         {Texto: strings.Repeat("a", MaxTextoIA+1), Accion: AccionNinguna},
		"correo":        {Texto: "Escribe a dueno@example.com para lanzar tu cupón hoy", Accion: AccionCrearCupon},
		"acción ajena":  {Texto: "Los martes de 4 a 6 tienes poca gente. ¿Lanzas un cupón?", Accion: "borrar_negocio"},
		"acción vacía":  {Texto: "Los martes de 4 a 6 tienes poca gente. ¿Lanzas un cupón?"},
		"solo espacios": {Texto: strings.Repeat(" ", 30), Accion: AccionNinguna},
	} {
		assert.Error(t, s.Validar(), nombre)
	}
}

func TestPlantillas_CabenEnLaRespuesta(t *testing.T) {
	for _, h := range []Hecho{
		{Regla: ReglaFranjaFloja, DiaSemana: "miércoles", Desde: 22, Hasta: 24, Porcentaje: 100},
		{Regla: ReglaAspectoEnBaja, Aspecto: "precio-calidad", Caida: 4},
		{Regla: ReglaResenasSinResp, Conteo: 999}, {Regla: ReglaCuponSinCanjes, Conteo: 999}, {Regla: ReglaReservasApagadas, Conteo: 99999},
		{Regla: ReglaPerfil}, {Regla: ReglaNinguna},
	} {
		n := utf8.RuneCountInString(Plantilla(h))
		assert.True(t, n >= MinTextoIA && n <= MaxTextoIA, "%s: %d", h.Regla, n)
	}
}

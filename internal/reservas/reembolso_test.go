package reservas

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
)

func TestPorcentaje_PoliticasEnSusBordes(t *testing.T) {
	cita := time.Date(2026, 12, 1, 20, 0, 0, 0, time.UTC)
	casos := []struct {
		politica string
		antes    time.Duration
		want     int
	}{
		{PoliticaFlexible, 2 * time.Hour, 100},
		{PoliticaFlexible, 2*time.Hour - time.Second, 0},
		{PoliticaModerada, 24 * time.Hour, 100},
		{PoliticaModerada, 24*time.Hour - time.Second, 50},
		{PoliticaEstricta, 48 * time.Hour, 50},
		{PoliticaEstricta, 48*time.Hour - time.Second, 0},
		{"desconocida", 3 * time.Hour, 100},
	}
	for _, c := range casos {
		got := Porcentaje(PoliticaDe(c.politica), cita, cita.Add(-c.antes), false)
		assert.Equal(t, c.want, got, "%s a %s", c.politica, c.antes)
	}
	assert.Equal(t, 100, Porcentaje(PoliticaDe(PoliticaEstricta), cita, cita, true), "si cancela el negocio, 100 %")
}

func TestReembolso_SinPagoEsCero(t *testing.T) {
	cita := time.Date(2026, 12, 1, 20, 0, 0, 0, time.UTC)
	r := Reserva{FechaHora: cita, Anticipo: Anticipo{MontoCentavos: 20000}, Politica: PoliticaDe(PoliticaModerada)}
	assert.Zero(t, Reembolso(r, cita.Add(-time.Hour), false), "sin pago_id no hay qué devolver")
	r.PagoID = "pi_prueba"
	assert.Equal(t, int64(10000), Reembolso(r, cita.Add(-time.Hour), false))
	assert.Equal(t, int64(20000), Reembolso(r, cita.Add(-time.Hour), true))
}

func TestCalcularAnticipo(t *testing.T) {
	assert.Equal(t, int64(10000), CalcularAnticipo(negocios.Anticipo{Tipo: AnticipoFijo, Valor: 100}, 250, 4))
	assert.Equal(t, int64(10000), CalcularAnticipo(negocios.Anticipo{Tipo: AnticipoPorcentaje, Valor: 10}, 250, 4), "10 % de 250 × 4 = $100")
	assert.Equal(t, int64(20000), CalcularAnticipo(negocios.Anticipo{Tipo: AnticipoPorPersona, Valor: 50}, 250, 4))
	assert.Zero(t, CalcularAnticipo(negocios.Anticipo{Tipo: AnticipoFijo}, 250, 4))
	assert.Zero(t, CalcularAnticipo(negocios.Anticipo{Tipo: "raro", Valor: 5}, 250, 4))
}

func TestNuevoCodigo_AlfabetoYLongitud(t *testing.T) {
	vistos := map[string]bool{}
	for range 500 {
		c, err := NuevoCodigo()
		require.NoError(t, err)
		require.Len(t, c, CodigoLongitud)
		for _, r := range c {
			require.True(t, strings.ContainsRune(alfabeto, r), c)
		}
		assert.NotContains(t, c, "O")
		assert.NotContains(t, c, "I")
		vistos[c] = true
	}
	assert.Greater(t, len(vistos), 490, "32^6 combinaciones: casi nunca se repite")
	assert.Len(t, alfabeto, 32)
}

func TestReserva_OcupaYCancelable(t *testing.T) {
	now := time.Now()
	futuro, pasado := now.Add(time.Minute), now.Add(-time.Minute)
	assert.True(t, Reserva{Estado: EstadoBloqueada, BloqueoExpiraEn: &futuro}.Ocupa(now))
	assert.False(t, Reserva{Estado: EstadoBloqueada, BloqueoExpiraEn: &pasado}.Ocupa(now))
	assert.False(t, Reserva{Estado: EstadoBloqueada}.Ocupa(now))
	assert.True(t, Reserva{Estado: EstadoPendiente}.Ocupa(now))
	assert.False(t, Reserva{Estado: EstadoAsistio}.Ocupa(now))
	assert.True(t, Reserva{Estado: EstadoConfirmada}.Cancelable())
	assert.False(t, Reserva{Estado: EstadoExpirada}.Cancelable())
}

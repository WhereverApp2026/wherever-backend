package clima_test

import (
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/clima"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/climatest"
)

func nuevo(t *testing.T) (*clima.Servicio, *climatest.Servidor, *time.Time) {
	t.Helper()
	srv := climatest.Nuevo(t)
	now := time.Date(2026, 11, 12, 18, 0, 0, 0, time.UTC)
	svc := clima.NewServicio(clima.NewOpenMeteo(srv.URL), slog.New(slog.NewTextHandler(io.Discard, nil))).
		WithClock(func() time.Time { return now })
	return svc, srv, &now
}

func TestClima_RespuestaNormal(t *testing.T) {
	svc, srv, _ := nuevo(t)
	srv.Responder(climatest.Actual{TemperaturaC: 21.5, Codigo: 61, EsDeDia: true, PrecipitacionMM: 1.2})
	c := svc.Actual(t.Context(), 19.4326, -99.1332)
	require.True(t, c.Disponible)
	assert.InDelta(t, 21.5, *c.TemperaturaC, 1e-9)
	assert.Equal(t, clima.EstadoLluvia, c.Estado)
	assert.True(t, *c.Adverso)
	assert.True(t, *c.EsDeDia)
}

func TestClima_ServicioLentoNoDisponible(t *testing.T) {
	svc, srv, _ := nuevo(t)
	svc.WithPlazo(50 * time.Millisecond)
	srv.Tardar(time.Second)
	start := time.Now()
	c := svc.Actual(t.Context(), 19.43, -99.13)
	assert.False(t, c.Disponible)
	assert.Nil(t, c.Adverso, "sin clima el factor queda neutro")
	assert.Less(t, time.Since(start), 900*time.Millisecond, "respeta el plazo")
}

func TestClima_ServicioCaidoSeRecuerdaUnMinuto(t *testing.T) {
	svc, srv, now := nuevo(t)
	srv.Fallar(http.StatusInternalServerError)
	assert.False(t, svc.Actual(t.Context(), 19.43, -99.13).Disponible)
	srv.Responder(climatest.Actual{TemperaturaC: 20})
	assert.False(t, svc.Actual(t.Context(), 19.43, -99.13).Disponible, "el fallo se recuerda")
	assert.Equal(t, 1, srv.Llamadas())
	*now = now.Add(clima.VigenciaFallo)
	assert.True(t, svc.Actual(t.Context(), 19.43, -99.13).Disponible)
}

func TestClima_CacheTreintaMinutosPorCelda(t *testing.T) {
	svc, srv, now := nuevo(t)
	svc.Actual(t.Context(), 19.4326, -99.1332)
	svc.Actual(t.Context(), 19.4349, -99.1301) // misma celda de 0,01°
	assert.Equal(t, 1, srv.Llamadas())
	*now = now.Add(clima.Vigencia - time.Second)
	svc.Actual(t.Context(), 19.4326, -99.1332)
	assert.Equal(t, 1, srv.Llamadas(), "vigente hasta los 30 min")
	*now = now.Add(time.Second)
	svc.Actual(t.Context(), 19.4326, -99.1332)
	assert.Equal(t, 2, srv.Llamadas(), "a los 30 min se vuelve a pedir")
	svc.Reset()
	svc.Actual(t.Context(), 19.4326, -99.1332)
	assert.Equal(t, 3, srv.Llamadas())
}

func TestClima_EstadoYAdverso(t *testing.T) {
	cases := map[int]string{0: clima.EstadoDespejado, 2: clima.EstadoNublado, 45: clima.EstadoNiebla,
		53: clima.EstadoLlovizna, 63: clima.EstadoLluvia, 81: clima.EstadoLluvia, 73: clima.EstadoNieve, 95: clima.EstadoTormenta}
	for code, want := range cases {
		assert.Equal(t, want, clima.EstadoDe(code), code)
	}
	assert.False(t, clima.Adverso(clima.EstadoDespejado, 22, 0))
	assert.True(t, clima.Adverso(clima.EstadoDespejado, 33, 0), "calor")
	assert.True(t, clima.Adverso(clima.EstadoNublado, 6, 0), "frío")
	assert.True(t, clima.Adverso(clima.EstadoNublado, 18, 0.5), "llueve aunque el código no lo diga")
}

func TestClima_RespuestaInvalidaNoDisponible(t *testing.T) {
	svc := clima.NewServicio(clima.NewOpenMeteo("http://127.0.0.1:1"), slog.New(slog.NewTextHandler(io.Discard, nil)))
	assert.False(t, svc.Actual(t.Context(), 0, 0).Disponible)
	assert.InDelta(t, 19.43, clima.Redondear(19.4349), 1e-9)
}

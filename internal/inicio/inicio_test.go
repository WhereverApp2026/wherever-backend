package inicio

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/clima"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/recomendaciones"
)

func TestFranjaDe(t *testing.T) {
	at := func(h, m int) time.Time { return time.Date(2026, 11, 12, h, m, 0, 0, negocios.MexicoCity()) }
	assert.Equal(t, FranjaNoche, FranjaDe(at(4, 59)))
	assert.Equal(t, FranjaManana, FranjaDe(at(5, 0)))
	assert.Equal(t, FranjaManana, FranjaDe(at(11, 59)))
	assert.Equal(t, FranjaTarde, FranjaDe(at(12, 0)))
	assert.Equal(t, FranjaTarde, FranjaDe(at(18, 59)))
	assert.Equal(t, FranjaNoche, FranjaDe(at(19, 0)))
	assert.Equal(t, FranjaNoche, FranjaDe(time.Date(2026, 11, 12, 2, 0, 0, 0, time.UTC)), "20:00 en CDMX")
}

type recoFalso struct {
	q       recomendaciones.Query
	adverso *bool
	moment  time.Time
	err     error
}

func (r *recoFalso) ForHome(_ context.Context, _ string, q recomendaciones.Query, adverso *bool) (recomendaciones.Response, error) {
	r.q, r.adverso = q, adverso
	return recomendaciones.Response{Items: []recomendaciones.Card{}, Moment: r.moment}, r.err
}

type climaFijo clima.Clima

func (c climaFijo) Actual(context.Context, float64, float64) clima.Clima { return clima.Clima(c) }

func TestHome_ArmaSaludoChipsYPasaElClima(t *testing.T) {
	lat, lng := 19.43, -99.13
	reco := &recoFalso{moment: time.Date(2026, 11, 12, 20, 30, 0, 0, negocios.MexicoCity())}
	cl := clima.De(clima.Lectura{TemperaturaC: 15, Codigo: 63})
	out, err := NewService(reco, climaFijo(cl)).Home(t.Context(), "", Query{Lat: &lat, Lng: &lng, Hour: "20:30"})
	require.NoError(t, err)
	assert.Equal(t, "Buenas noches", out.Saludo)
	assert.Equal(t, FranjaNoche, out.Franja)
	assert.Equal(t, chips[FranjaNoche], out.Chips)
	assert.Equal(t, Recomendaciones, reco.q.Limit)
	require.NotNil(t, reco.adverso)
	assert.True(t, *reco.adverso)
}

func TestHome_SinClimaFactorNeutroYErrorDeRecomendaciones(t *testing.T) {
	lat, lng := 19.43, -99.13
	reco := &recoFalso{moment: time.Now()}
	out, err := NewService(reco, climaFijo(clima.NoDisponible())).Home(t.Context(), "", Query{Lat: &lat, Lng: &lng})
	require.NoError(t, err)
	assert.False(t, out.Clima.Disponible)
	assert.Nil(t, reco.adverso)

	reco.err = errors.New("caída")
	_, err = NewService(reco, climaFijo(clima.NoDisponible())).Home(t.Context(), "", Query{Lat: &lat, Lng: &lng})
	assert.Error(t, err)
}

package negocios

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// at es una hora local de Ciudad de México. 2026-11-09 es lunes (dia 1).
func at(t *testing.T, day, hour, minute int) time.Time {
	t.Helper()
	loc, err := time.LoadLocation(zonaHoraria)
	require.NoError(t, err)
	return time.Date(2026, 11, 8+day, hour, minute, 0, 0, loc)
}

func TestHorario_OpenAt(t *testing.T) {
	simple := deriveScheduleMin([]Horario{{Dia: 1, Abre: "09:00", Cierra: "18:00"}})
	split := deriveScheduleMin([]Horario{
		{Dia: 1, Abre: "08:00", Cierra: "12:00"},
		{Dia: 1, Abre: "16:00", Cierra: "22:00"},
	})
	overnight := deriveScheduleMin([]Horario{{Dia: 5, Abre: "20:00", Cierra: "03:00"}}) // viernes
	cases := []struct {
		name    string
		windows []VentanaMin
		now     time.Time
		want    bool
	}{
		{"tramo simple abierto", simple, at(t, 1, 12, 0), true},
		{"dos tramos, entre ambos cerrado", split, at(t, 1, 14, 0), false},
		{"dos tramos, segundo abierto", split, at(t, 1, 21, 59), true},
		{"cruce de medianoche, sábado de madrugada", overnight, at(t, 6, 2, 30), true},
		{"cruce de medianoche, viernes noche", overnight, at(t, 5, 23, 0), true},
		{"día sin servicio", simple, at(t, 2, 12, 0), false},
		{"borde exacto de apertura incluido", simple, at(t, 1, 9, 0), true},
		{"borde exacto de cierre excluido", simple, at(t, 1, 18, 0), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, OpenAt(tc.windows, tc.now))
		})
	}
}

func TestHorario_UsesMexicoCityNotUTC(t *testing.T) {
	windows := deriveScheduleMin([]Horario{{Dia: 1, Abre: "09:00", Cierra: "10:00"}})
	// 15:30 UTC del lunes son 09:30 en Ciudad de México.
	assert.True(t, OpenAt(windows, time.Date(2026, 11, 9, 15, 30, 0, 0, time.UTC)))
	assert.False(t, OpenAt(windows, time.Date(2026, 11, 9, 9, 30, 0, 0, time.UTC)))
}

func TestHorario_FilterMatchesRule(t *testing.T) {
	elem := func(cond bson.D) bson.D {
		return bson.D{{Key: "horario_min", Value: bson.D{{Key: "$elemMatch", Value: cond}}}}
	}
	want := bson.E{Key: "$or", Value: bson.A{
		elem(bson.D{
			{Key: "dia", Value: 0},
			{Key: "desde", Value: bson.D{{Key: "$lte", Value: 75}}},
			{Key: "hasta", Value: bson.D{{Key: "$gt", Value: 75}}},
		}),
		// Ayer de domingo es sábado; su ventana debe pasar de 1440 + 75.
		elem(bson.D{
			{Key: "dia", Value: 6},
			{Key: "hasta", Value: bson.D{{Key: "$gt", Value: 75 + minutosDia}}},
		}),
	}}
	assert.Equal(t, want, openAtFilter(at(t, 0, 1, 15))) // domingo 01:15
}

func TestLugaresCursor_RoundTripAndOrderMismatch(t *testing.T) {
	id := bson.NewObjectID()
	dist := 1234.5678901234
	row := placeRow{Business: Business{ID: id, AveragePrice: 150, Rating: Calificacion{Promedio: 4.7}}, DistanceM: &dist}

	got, err := decodeCursor(encodeCursor(OrderDistance, row), OrderDistance)
	require.NoError(t, err)
	assert.Equal(t, id, got.ID)
	assert.Equal(t, dist, got.Value)

	got, err = decodeCursor(encodeCursor(OrderPrice, row), OrderPrice)
	require.NoError(t, err)
	assert.Equal(t, 150, got.Value)

	_, err = decodeCursor(encodeCursor(OrderRating, row), OrderPrice)
	require.Error(t, err)
	_, err = decodeCursor("%%%", OrderRating)
	require.Error(t, err)
	none, err := decodeCursor("", OrderRating)
	require.NoError(t, err)
	assert.Nil(t, none)
}

func TestLugares_CellDegreesShrinkWithZoom(t *testing.T) {
	assert.InDelta(t, 360.0/1024/4, CellDegrees(10), 1e-12)
	assert.Greater(t, CellDegrees(10), CellDegrees(16))
}

func TestLugares_CoverAndDetailHideControlFields(t *testing.T) {
	b := Business{
		ID: bson.NewObjectID(), QRCheckin: "secreto",
		Location: Location{Type: "Point", Coordinates: []float64{-99.1, 19.4}},
		Photos:   []Foto{{URL: "b", Orden: 2}, {URL: "a", Orden: 1}},
	}
	assert.Equal(t, "a", cover(b.Photos))
	assert.Empty(t, cover(nil))
	d := toDetail(b, time.Now())
	assert.Equal(t, Point{Lat: 19.4, Lng: -99.1}, d.Location)
	assert.NotNil(t, d.Tags)
	assert.Equal(t, Point{}, pointOf(Location{}))
}

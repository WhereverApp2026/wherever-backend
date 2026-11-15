package grupos

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

func TestRoundLocation_TresDecimales(t *testing.T) {
	c := roundLocation(19.432612, -99.133209)
	assert.InDelta(t, 19.433, c.Lat, 1e-9)
	assert.InDelta(t, -99.133, c.Lng, 1e-9)
}

func TestCenter_PromedioEnCuadricula(t *testing.T) {
	c := center([]Coords{{Lat: 19.4012, Lng: -99.1712}, {Lat: 19.4231, Lng: -99.1488}})
	assert.InDelta(t, 19.41, c.Lat, 1e-9)
	assert.InDelta(t, -99.16, c.Lng, 1e-9)
	// Mover a una persona unos cientos de metros no cambia el centro: no se puede rastrear.
	moved := center([]Coords{{Lat: 19.4031, Lng: -99.1712}, {Lat: 19.4231, Lng: -99.1488}})
	assert.Equal(t, c, moved)
}

func TestRangos_Cortes(t *testing.T) {
	for m, want := range map[int]string{0: "hasta_15", 15: "hasta_15", 16: "15_30", 30: "15_30", 45: "30_60", 61: "mas_60"} {
		assert.Equal(t, want, minutesRange(m), m)
	}
	for p, want := range map[int]string{0: "hasta_50", 50: "hasta_50", 80: "50_100", 150: "100_200", 201: "mas_200"} {
		assert.Equal(t, want, costRange(p), p)
	}
}

func TestTripCost_Tabla(t *testing.T) {
	assert.Equal(t, 0, tripCost(planes.MobilityWalk, 10))
	assert.Equal(t, 0, tripCost(planes.MobilityBike, 10))
	assert.Equal(t, 40, tripCost(planes.MobilityCar, 10))
	assert.Equal(t, transitFare, tripCost(planes.MobilityTransit, 10))
}

func TestTrip_MovilidadDesconocidaEsAPie(t *testing.T) {
	s := (&Service{}).WithExtras(nil, nil, nil)
	from, to := Coords{Lat: 19.40, Lng: -99.17}, Coords{Lat: 19.41, Lng: -99.17}
	walk, cost := s.trip(from, to, "")
	car, _ := s.trip(from, to, planes.MobilityCar)
	assert.Equal(t, 0, cost)
	assert.Greater(t, walk, car)
}

func TestSurprise_SeCalculaAlLeer(t *testing.T) {
	target, other := bson.NewObjectID(), bson.NewObjectID()
	now := time.Date(2026, 11, 20, 18, 0, 0, 0, time.UTC)
	sp := &Surprise{Active: true, For: target, RevealAt: now.Add(time.Hour)}
	assert.True(t, sp.HidesFrom(target, now))
	assert.False(t, sp.HidesFrom(other, now))
	assert.False(t, sp.HidesFrom(target, now.Add(time.Hour)), "a la hora de revelar ya se ve")
	assert.False(t, (&Surprise{For: target, RevealAt: now.Add(time.Hour)}).HidesFrom(target, now), "apagada")
	var none *Surprise
	assert.False(t, none.HidesFrom(target, now))

	mine := surpriseView(sp, target, now)
	require.NotNil(t, mine)
	assert.Empty(t, mine.For, "la persona sorprendida no ve que es para ella")
	theirs := surpriseView(sp, other, now)
	require.NotNil(t, theirs)
	assert.Equal(t, target.Hex(), theirs.For)
	assert.Nil(t, surpriseView(sp, other, now.Add(2*time.Hour)))
}

func TestMemberVisible_Vence(t *testing.T) {
	now := time.Now()
	exp := now.Add(time.Minute)
	m := Member{SharedLocation: &Coords{Lat: 1, Lng: 1}, LocationExpires: &exp}
	assert.True(t, m.Visible(now))
	assert.False(t, m.Visible(now.Add(2*time.Minute)))
	assert.False(t, Member{}.Visible(now))
}

func TestPlanDay_HoraDeMexico(t *testing.T) {
	// 2026-11-21 02:00 UTC es el 20 en la Ciudad de México.
	start, end := planDay(time.Date(2026, 11, 21, 2, 0, 0, 0, time.UTC))
	assert.Equal(t, 20, start.Day())
	assert.Equal(t, 24*time.Hour, end.Sub(start))
}

func TestExtrasDTO_Validaciones(t *testing.T) {
	v, err := validation.New()
	require.NoError(t, err)
	lat, lng := 19.4, -99.1
	assert.NoError(t, v.Struct(MeetingPointRequest{Name: "Kiosco", Lat: &lat, Lng: &lng, At: "2026-11-20T18:00:00Z"}))
	assert.NoError(t, v.Struct(MeetingPointRequest{BusinessID: bson.NewObjectID().Hex(), At: "2026-11-20T18:00:00Z"}))
	assert.Error(t, v.Struct(MeetingPointRequest{At: "2026-11-20T18:00:00Z"}), "sin nombre ni negocio")
	bad := 91.0
	assert.Error(t, v.Struct(MeetingPointRequest{Name: "x", Lat: &bad, Lng: &lng, At: "2026-11-20T18:00:00Z"}))
	assert.Error(t, v.Struct(SurpriseRequest{}), "activo es obligatorio")
	assert.Error(t, v.Struct(RenameRequest{}))
	assert.Error(t, v.Struct(RespondRequest{Status: "quizas"}))
	assert.NoError(t, v.Struct(RespondRequest{Location: &LocationInput{Lat: &lat, Lng: &lng}}))
	assert.Error(t, v.Struct(RespondRequest{Location: &LocationInput{Lat: &lat}}))
}

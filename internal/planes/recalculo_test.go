package planes

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
)

// Puntos de Tula de Allende, Hidalgo.
var (
	zocaloTula   = Lugar{Lat: 20.0536, Lng: -99.3406}
	atlantes     = Lugar{Lat: 20.0642, Lng: -99.3420}
	catedralTula = Lugar{Lat: 20.0540, Lng: -99.3426}
)

// sabado10 es sábado 2026-11-14 a las 10:00 en Ciudad de México (UTC−6).
var sabado10 = time.Date(2026, 11, 14, 16, 0, 0, 0, time.UTC)

func allWeek() []negocios.VentanaMin {
	out := make([]negocios.VentanaMin, 0, 7)
	for d := 0; d < 7; d++ {
		out = append(out, negocios.VentanaMin{Dia: d, Desde: 8 * 60, Hasta: 22 * 60})
	}
	return out
}

func at(min int) time.Time { return sabado10.Add(time.Duration(min) * time.Minute) }

// fixture arma un plan con paradas en lugares separados ~1 km entre sí.
func fixture(mobility string, starts ...int) (*Plan, map[bson.ObjectID]Lugar) {
	p := &Plan{Personas: 2, Movilidad: mobility}
	lugares := map[bson.ObjectID]Lugar{}
	for i, s := range starts {
		id := bson.NewObjectID()
		l := Lugar{Lat: zocaloTula.Lat + float64(i)*0.009, Lng: zocaloTula.Lng, Horario: allWeek()}
		lugares[id] = l
		p.Paradas = append(p.Paradas, Parada{ID: bson.NewObjectID(), NegocioID: id, Llega: at(s), Minutos: 60, Precio: 100 + 10*i})
	}
	return p, lugares
}

func TestHaversine_DistanciasConocidasEnTula(t *testing.T) {
	assert.InDelta(t, 1.18, Haversine(zocaloTula.Lat, zocaloTula.Lng, atlantes.Lat, atlantes.Lng), 0.02)
	assert.InDelta(t, 0.21, Haversine(zocaloTula.Lat, zocaloTula.Lng, catedralTula.Lat, catedralTula.Lng), 0.02)
	assert.InDelta(t, 0, Haversine(atlantes.Lat, atlantes.Lng, atlantes.Lat, atlantes.Lng), 1e-9)
	assert.InDelta(t, Haversine(zocaloTula.Lat, zocaloTula.Lng, atlantes.Lat, atlantes.Lng),
		Haversine(atlantes.Lat, atlantes.Lng, zocaloTula.Lat, zocaloTula.Lng), 1e-9)
}

func TestRecalcular_OrdenaPorHora(t *testing.T) {
	p, l := fixture(MobilityCar, 200, 0, 100)
	require.NoError(t, Recalcular(p, l, DefaultVelocidades()))
	for i := 1; i < len(p.Paradas); i++ {
		assert.True(t, p.Paradas[i-1].Llega.Before(p.Paradas[i].Llega))
	}
	require.Len(t, p.Traslados, 2)
	assert.Equal(t, p.Paradas[0].ID, p.Traslados[0].Desde)
	assert.Equal(t, p.Paradas[1].ID, p.Traslados[0].Hasta)
}

func TestRecalcular_TrasladosPorMovilidad(t *testing.T) {
	// ~1 km en línea recta × 1,3 = 1,3 km.
	cases := map[string]int{MobilityWalk: 18, MobilityBike: 7, MobilityCar: 4, MobilityTransit: 5}
	for mobility, want := range cases {
		p, l := fixture(mobility, 0, 120)
		require.NoError(t, Recalcular(p, l, DefaultVelocidades()), mobility)
		assert.InDelta(t, 1.3, p.Traslados[0].Km, 0.01, mobility)
		assert.Equal(t, want, p.Traslados[0].Minutos, mobility)
	}
}

func TestRecalcular_Costos(t *testing.T) {
	p, l := fixture(MobilityWalk, 0, 90, 180)
	require.NoError(t, Recalcular(p, l, DefaultVelocidades()))
	assert.Equal(t, 330, p.CostoPorPersona)
	assert.Equal(t, 660, p.CostoTotal)
}

func TestRecalcular_ParadaQueNoCabe(t *testing.T) {
	// La segunda empieza al acabar la primera, sin tiempo para 18 min a pie.
	p, l := fixture(MobilityWalk, 0, 60)
	err := Recalcular(p, l, DefaultVelocidades())
	var se *StopError
	require.ErrorAs(t, err, &se)
	assert.Equal(t, 1, se.Index)
	assert.Equal(t, ReasonOverlap, se.Reason)
	assert.Empty(t, p.Traslados, "si falla no deja cálculos a medias")
}

func TestRecalcular_TrasladoJustoCabe(t *testing.T) {
	p, l := fixture(MobilityWalk, 0, 78)
	require.NoError(t, Recalcular(p, l, DefaultVelocidades()))
}

func TestRecalcular_UnaParada(t *testing.T) {
	p, l := fixture(MobilityCar, 0)
	require.NoError(t, Recalcular(p, l, DefaultVelocidades()))
	assert.Empty(t, p.Traslados)
	assert.Equal(t, 200, p.CostoTotal)
}

func TestRecalcular_SeisParadas(t *testing.T) {
	p, l := fixture(MobilityCar, 0, 70, 140, 210, 280, 350)
	require.NoError(t, Recalcular(p, l, DefaultVelocidades()))
	assert.Len(t, p.Traslados, 5)
	assert.Equal(t, 2*(100+110+120+130+140+150), p.CostoTotal)
}

func TestRecalcular_CantidadDeParadas(t *testing.T) {
	empty, l := fixture(MobilityCar)
	assert.ErrorIs(t, Recalcular(empty, l, DefaultVelocidades()), ErrStopCount)
	seven, l7 := fixture(MobilityCar, 0, 70, 140, 210, 280, 350, 420)
	assert.ErrorIs(t, Recalcular(seven, l7, DefaultVelocidades()), ErrStopCount)
}

func TestRecalcular_NegocioNoDisponible(t *testing.T) {
	p, l := fixture(MobilityCar, 0, 120)
	delete(l, p.Paradas[1].NegocioID)
	var se *StopError
	require.ErrorAs(t, Recalcular(p, l, DefaultVelocidades()), &se)
	assert.Equal(t, ReasonUnavailable, se.Reason)
	assert.Equal(t, p.Paradas[1].ID, se.StopID)
}

func TestRecalcular_CerradoEnLaFranja(t *testing.T) {
	// Abre de 08:00 a 22:00: llegar a las 21:30 por 60 min se pasa del cierre.
	p, l := fixture(MobilityCar, 11*60+30)
	var se *StopError
	require.ErrorAs(t, Recalcular(p, l, DefaultVelocidades()), &se)
	assert.Equal(t, ReasonClosed, se.Reason)
	// Llega a las 21:00 y sale a las 22:00 justo: sí cabe.
	p2, l2 := fixture(MobilityCar, 11*60)
	require.NoError(t, Recalcular(p2, l2, DefaultVelocidades()))
}

func TestRecalcular_EncabezadoInvalido(t *testing.T) {
	p, l := fixture("cohete", 0)
	assert.ErrorIs(t, Recalcular(p, l, DefaultVelocidades()), ErrMobility)
	p2, l2 := fixture(MobilityCar, 0)
	p2.Personas = 0
	assert.ErrorIs(t, Recalcular(p2, l2, DefaultVelocidades()), ErrPeople)
}

func TestRecalcular_DuracionYPrecio(t *testing.T) {
	p, l := fixture(MobilityCar, 0)
	p.Paradas[0].Minutos = 0
	var se *StopError
	require.ErrorAs(t, Recalcular(p, l, DefaultVelocidades()), &se)
	assert.Equal(t, ReasonDuration, se.Reason)
	p2, l2 := fixture(MobilityCar, 0)
	p2.Paradas[0].Precio = -1
	require.ErrorAs(t, Recalcular(p2, l2, DefaultVelocidades()), &se)
	assert.Equal(t, ReasonPrice, se.Reason)
	assert.Contains(t, se.Error(), "parada 0")
}

func TestPlan_FormaJSON(t *testing.T) {
	p, l := fixture(MobilityCar, 0, 120)
	p.OwnerID, p.Estado, p.Origen = bson.NewObjectID(), StatusActive, OriginManual
	require.NoError(t, Recalcular(p, l, DefaultVelocidades()))
	raw, err := json.Marshal(p)
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal(raw, &got))
	for _, key := range []string{"id", "nombre", "fecha", "personas", "presupuesto", "movilidad", "paradas", "traslados", "costo_total", "costo_por_persona", "origen", "version"} {
		assert.Contains(t, got, key)
	}
	assert.NotContains(t, got, "dueno_id", "el dueño no sale en la respuesta")
	assert.NotContains(t, got, "estado")
	var nested struct {
		Paradas   []map[string]any `json:"paradas"`
		Traslados []map[string]any `json:"traslados"`
	}
	require.NoError(t, json.Unmarshal(raw, &nested))
	for _, key := range []string{"id", "negocio_id", "llega", "minutos", "precio"} {
		assert.Contains(t, nested.Paradas[0], key)
	}
	for _, key := range []string{"desde", "hasta", "minutos", "km"} {
		assert.Contains(t, nested.Traslados[0], key)
	}
}

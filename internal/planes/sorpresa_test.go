package planes

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestCut_SoloInicioYPunto(t *testing.T) {
	first, second := bson.NewObjectID(), bson.NewObjectID()
	at := time.Date(2026, 11, 20, 18, 0, 0, 0, time.UTC)
	p := Plan{
		Nombre: "Sorpresa", Fecha: at, Presupuesto: 900, CostoTotal: 700, CostoPorPersona: 350,
		Paradas:   []Parada{{NegocioID: second, Llega: at.Add(2 * time.Hour)}, {NegocioID: first, Llega: at}},
		Traslados: []Traslado{{Minutos: 10}},
	}
	cut(&p, &Sorpresa{RevelarEn: at.Add(-time.Hour)})

	assert.Empty(t, p.Paradas)
	assert.Empty(t, p.Traslados)
	assert.Zero(t, p.CostoTotal+p.CostoPorPersona+p.Presupuesto)
	require.NotNil(t, p.Sorpresa)
	require.NotNil(t, p.Sorpresa.Inicio)
	assert.Equal(t, at, *p.Sorpresa.Inicio, "la primera parada por hora")
	require.NotNil(t, p.Sorpresa.Punto)
	assert.Equal(t, first.Hex(), p.Sorpresa.Punto.NegocioID, "sin punto de encuentro, el lugar de la primera parada")

	raw, err := json.Marshal(p)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), second.Hex(), "ninguna otra parada se filtra")
	assert.Contains(t, string(raw), `"paradas":[]`)
}

func TestCut_ConservaElPuntoDelGrupo(t *testing.T) {
	lat, lng := 19.4, -99.1
	p := Plan{Paradas: []Parada{{NegocioID: bson.NewObjectID(), Llega: time.Now()}}}
	cut(&p, &Sorpresa{Punto: &PuntoSorpresa{Nombre: "Kiosco", Lat: &lat, Lng: &lng}})
	assert.Equal(t, "Kiosco", p.Sorpresa.Punto.Nombre)
	assert.Empty(t, p.Sorpresa.Punto.NegocioID)
}

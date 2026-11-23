package planes

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestSalidasPorAvisar_TrasladoMasCincoMinutos(t *testing.T) {
	base := time.Date(2026, 11, 23, 18, 0, 0, 0, time.UTC)
	a := Parada{ID: bson.NewObjectID(), NegocioID: bson.NewObjectID(), Llega: base}
	b := Parada{ID: bson.NewObjectID(), NegocioID: bson.NewObjectID(), Llega: base.Add(2 * time.Hour)}
	p := Plan{ID: bson.NewObjectID(), OwnerID: bson.NewObjectID(), Paradas: []Parada{a, b},
		Traslados: []Traslado{{Desde: a.ID, Hasta: b.ID, Minutos: 20}}}
	sale := b.Llega.Add(-25 * time.Minute)

	assert.Empty(t, SalidasPorAvisar(p, sale.Add(-time.Second)), "antes de la hora de salir")
	got := SalidasPorAvisar(p, sale)
	if assert.Len(t, got, 1) {
		assert.Equal(t, b.ID, got[0].Parada.ID)
		assert.Equal(t, sale, got[0].SaleEn)
		assert.Equal(t, p.OwnerID, got[0].DuenoID)
	}
	assert.Len(t, SalidasPorAvisar(p, b.Llega.Add(-time.Minute)), 1, "aún sirve antes de llegar")
	assert.Empty(t, SalidasPorAvisar(p, b.Llega), "ya debía estar ahí")
}

func TestSalidasPorAvisar_SinTrasladoValido(t *testing.T) {
	a := Parada{ID: bson.NewObjectID(), Llega: time.Now()}
	p := Plan{Paradas: []Parada{a}, Traslados: []Traslado{{Desde: a.ID, Hasta: bson.NewObjectID(), Minutos: 10}, {Desde: a.ID, Hasta: a.ID}}}
	assert.Empty(t, SalidasPorAvisar(p, time.Now()))
}

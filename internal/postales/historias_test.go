package postales

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestAgruparHistorias_PersonaMasRecientePrimeroYCronologico(t *testing.T) {
	ana, jorge, ajena := bson.NewObjectID(), bson.NewObjectID(), bson.NewObjectID()
	now := time.Now().UTC()
	m := func(autor bson.ObjectID, hace time.Duration) Momento {
		return Momento{ID: bson.NewObjectID(), AutorID: autor, CreadoEn: now.Add(-hace)}
	}
	// Del más nuevo al más viejo, como los entrega el repositorio.
	rows := []Momento{m(ana, time.Minute), m(jorge, time.Hour), m(ajena, 2*time.Hour), m(ana, 3*time.Hour), m(jorge, 4*time.Hour)}
	autores := map[bson.ObjectID]AutorView{ana: {ID: ana.Hex()}, jorge: {ID: jorge.Hex()}}
	out := AgruparHistorias(rows, autores, func(x Momento) HistoriaMomento { return HistoriaMomento{ID: x.ID.Hex()} })

	require.Len(t, out, 2, "quien no está en autores no sale")
	assert.Equal(t, ana.Hex(), out[0].Autor.ID)
	assert.Equal(t, []string{rows[3].ID.Hex(), rows[0].ID.Hex()}, ids(out[0].Momentos), "de la más vieja a la más nueva")
	assert.Equal(t, []string{rows[4].ID.Hex(), rows[1].ID.Hex()}, ids(out[1].Momentos))
}

func TestAgruparHistorias_Hasta30Personas(t *testing.T) {
	rows := []Momento{}
	autores := map[bson.ObjectID]AutorView{}
	for range MaxPersonasHist + 5 {
		id := bson.NewObjectID()
		autores[id] = AutorView{ID: id.Hex()}
		rows = append(rows, Momento{ID: bson.NewObjectID(), AutorID: id})
	}
	out := AgruparHistorias(rows, autores, func(x Momento) HistoriaMomento { return HistoriaMomento{ID: x.ID.Hex()} })
	assert.Len(t, out, MaxPersonasHist)
}

func ids(ms []HistoriaMomento) []string {
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		out = append(out, m.ID)
	}
	return out
}

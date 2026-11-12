package grafo

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/neo4j"
)

func TestBackoff_CreceHastaElTope(t *testing.T) {
	assert.Equal(t, 5*time.Second, backoff(0))
	assert.Equal(t, 5*time.Second, backoff(1))
	assert.Equal(t, 10*time.Second, backoff(2))
	assert.Equal(t, 40*time.Second, backoff(4))
	assert.Equal(t, maxDelay, backoff(8))
	assert.Equal(t, maxDelay, backoff(1000))
}

func TestBusinesses_SoloPlanesActivosSinRepetir(t *testing.T) {
	a, b, c := bson.NewObjectID(), bson.NewObjectID(), bson.NewObjectID()
	stops := func(ids ...bson.ObjectID) []planes.Parada {
		out := make([]planes.Parada, len(ids))
		for i, id := range ids {
			out[i] = planes.Parada{NegocioID: id}
		}
		return out
	}
	got := businesses([]planes.Plan{
		{Estado: planes.StatusActive, Paradas: stops(b, a)},
		{Estado: planes.StatusActive, Paradas: stops(a)},
		{Estado: planes.StatusDeleted, Paradas: stops(c)},
	})

	want := []string{a.Hex(), b.Hex()}
	if want[0] > want[1] {
		want[0], want[1] = want[1], want[0]
	}
	assert.Equal(t, want, got)
	assert.Empty(t, businesses(nil))
	assert.NotNil(t, businesses(nil), "una lista vacía, no nil: Cypher recibe []")
}

func TestNearLimit_AvisaAlSetentaPorCiento(t *testing.T) {
	assert.False(t, NearLimit(neo4j.Size{Nodes: 34_999, Relationships: 122_499}))
	assert.True(t, NearLimit(neo4j.Size{Nodes: 35_000}))
	assert.True(t, NearLimit(neo4j.Size{Relationships: 122_500}))
}

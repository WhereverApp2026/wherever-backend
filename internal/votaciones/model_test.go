package votaciones

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

func voteWith(options []bson.ObjectID, choices ...bson.ObjectID) Vote {
	v := Vote{Options: options, Ballots: map[string]bson.ObjectID{}}
	for _, c := range choices {
		v.Ballots[bson.NewObjectID().Hex()] = c
	}
	return v
}

func TestWinnerOf(t *testing.T) {
	a, b, c := bson.NewObjectID(), bson.NewObjectID(), bson.NewObjectID()
	opts := []bson.ObjectID{a, b, c}
	assert.Equal(t, b, WinnerOf(voteWith(opts, b, b, a)), "más votos")
	assert.Equal(t, a, WinnerOf(voteWith(opts, b, a)), "empate: la propuesta primero")
	assert.Equal(t, b, WinnerOf(voteWith(opts, c, b)), "empate entre la segunda y la tercera")
	assert.Equal(t, a, WinnerOf(voteWith(opts)), "sin votos gana la primera")
	assert.Equal(t, c, WinnerOf(voteWith(opts, c, bson.NewObjectID(), bson.NewObjectID())), "un voto por otra cosa no cuenta")
	assert.Equal(t, []int{0, 0, 1}, voteWith(opts, c, bson.NewObjectID()).Counts())
}

func TestDTO_Validacion(t *testing.T) {
	v, err := validation.New()
	require.NoError(t, err)
	id := func() string { return bson.NewObjectID().Hex() }
	same := id()
	soon := time.Now().Add(time.Hour)
	cases := []struct {
		name string
		dto  any
		ok   bool
	}{
		{"dos opciones", OpenRequest{StopID: id(), Options: []string{id(), id()}, ClosesAt: soon}, true},
		{"tres opciones", OpenRequest{StopID: id(), Options: []string{id(), id(), id()}, ClosesAt: soon}, true},
		{"una opción", OpenRequest{StopID: id(), Options: []string{id()}, ClosesAt: soon}, false},
		{"cuatro opciones", OpenRequest{StopID: id(), Options: []string{id(), id(), id(), id()}, ClosesAt: soon}, false},
		{"repetidas", OpenRequest{StopID: id(), Options: []string{same, same}, ClosesAt: soon}, false},
		{"sin hora límite", OpenRequest{StopID: id(), Options: []string{id(), id()}}, false},
		{"sin parada", OpenRequest{Options: []string{id(), id()}, ClosesAt: soon}, false},
		{"voto", CastRequest{BusinessID: id()}, true},
		{"voto vacío", CastRequest{}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := v.Struct(c.dto)
			if c.ok {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestNewView_SoloConteosYMiVoto(t *testing.T) {
	a, b := bson.NewObjectID(), bson.NewObjectID()
	me, other := bson.NewObjectID(), bson.NewObjectID()
	v := Vote{
		ID: bson.NewObjectID(), Options: []bson.ObjectID{a, b}, Status: StatusOpen,
		Ballots: map[string]bson.ObjectID{me.Hex(): a, other.Hex(): b},
	}
	out := newView(v, me, nil)
	assert.Equal(t, a.Hex(), out.MyVote)
	assert.Equal(t, 2, out.TotalVotes)
	assert.Equal(t, 1, out.Options[0].Votes)
	raw, err := json.Marshal(out)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), other.Hex(), "no dice quién votó qué")
	assert.NotContains(t, string(raw), me.Hex())
}

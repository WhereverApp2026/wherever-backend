package sugerencias

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/neo4j"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// Dobles de las interfaces del service: la lógica de armado se prueba sin bases; las
// consultas reales van en tests/integration contra Mongo y Neo4j.
type relations []amistades.Friendship

func (r relations) ListByUser(context.Context, bson.ObjectID) ([]amistades.Friendship, error) {
	return r, nil
}

type users struct{ hidden map[bson.ObjectID]bool }

func (u users) find(ids []bson.ObjectID, searchable bool) []usuarios.User {
	out := []usuarios.User{}
	for _, id := range ids {
		if searchable && u.hidden[id] {
			continue
		}
		out = append(out, usuarios.User{ID: id, Name: "n" + id.Hex()[20:]})
	}
	return out
}

func (u users) FindPublicByIDs(_ context.Context, ids []bson.ObjectID) ([]usuarios.User, error) {
	return u.find(ids, false), nil
}

func (u users) FindSuggestableByIDs(_ context.Context, ids []bson.ObjectID) ([]usuarios.User, error) {
	return u.find(ids, true), nil
}

type places struct{ visible bool }

func (p places) FindVisible(context.Context, bson.ObjectID) (negocios.Business, bool, error) {
	return negocios.Business{}, p.visible, nil
}

type graph struct {
	mutual  []neo4j.Mutual
	went    []string
	err     error
	exclude []string
	friends []string
}

func (g *graph) MutualFriends(_ context.Context, _ string, exclude []string, _ int) ([]neo4j.Mutual, error) {
	g.exclude = exclude
	return g.mutual, g.err
}

func (g *graph) FriendsAt(_ context.Context, _, _ string, friends []string) ([]string, error) {
	g.friends = friends
	return g.went, g.err
}

func newService(t *testing.T, rel relations, u users, p places, g Graph) *Service {
	t.Helper()
	s, err := NewService(rel, u, p, nil)
	require.NoError(t, err)
	s.graph = g
	return s
}

func ids(n int) []bson.ObjectID {
	out := make([]bson.ObjectID, n)
	for i := range out {
		out[i] = bson.NewObjectID()
	}
	return out
}

func TestSuggestions_KeepsGraphOrderHidesUnsearchableAndCaps(t *testing.T) {
	me := bson.NewObjectID()
	people := ids(MaxSuggestions + 5)
	found := make([]neo4j.Mutual, 0, len(people))
	for i, id := range people {
		found = append(found, neo4j.Mutual{ID: id.Hex(), Common: int64(len(people) - i)})
	}
	pending := bson.NewObjectID()
	rel := relations{{From: me, To: pending, Status: amistades.StatusPending}}
	g := &graph{mutual: found}
	s := newService(t, rel, users{hidden: map[bson.ObjectID]bool{people[0]: true}}, places{}, g)

	out, err := s.Suggestions(context.Background(), me.Hex())
	require.NoError(t, err)
	assert.False(t, out.Degraded)
	require.Len(t, out.Results, MaxSuggestions)
	assert.Equal(t, people[1].Hex(), out.Results[0].Person.ID, "quien apagó buscable no aparece")
	assert.Equal(t, int64(len(people)-1), out.Results[0].Common)
	assert.Equal(t, []string{pending.Hex()}, g.exclude, "toda relación en Mongo se excluye")
}

func TestSuggestions_GraphDownIsDegradedNotError(t *testing.T) {
	me := bson.NewObjectID().Hex()
	s := newService(t, nil, users{}, places{}, &graph{err: errors.New("aura en pausa")})
	out, err := s.Suggestions(context.Background(), me)
	require.NoError(t, err)
	assert.True(t, out.Degraded)
	assert.Empty(t, out.Results)

	s = newService(t, nil, users{}, places{}, nil)
	out, err = s.Suggestions(context.Background(), me)
	require.NoError(t, err)
	assert.True(t, out.Degraded)
}

func TestFriendsAt_CountsAndShowsThree(t *testing.T) {
	me := bson.NewObjectID()
	friends := ids(5)
	rel := relations{}
	for _, f := range friends {
		rel = append(rel, amistades.Friendship{From: me, To: f, Status: amistades.StatusAccepted})
	}
	blocked := bson.NewObjectID()
	rel = append(rel, amistades.Friendship{From: blocked, To: me, Status: amistades.StatusBlocked})
	went := []string{friends[0].Hex(), friends[1].Hex(), friends[2].Hex(), friends[3].Hex()}
	g := &graph{went: went}
	s := newService(t, rel, users{}, places{visible: true}, g)

	out, err := s.FriendsAt(context.Background(), me.Hex(), bson.NewObjectID().Hex())
	require.NoError(t, err)
	assert.Equal(t, 4, out.Total)
	assert.Len(t, out.Friends, MaxAvatars)
	assert.Len(t, g.friends, len(friends), "solo amistades aceptadas van al grafo")
	assert.NotContains(t, g.friends, blocked.Hex())
}

func TestFriendsAt_NotFoundAndDegraded(t *testing.T) {
	me := bson.NewObjectID()
	s := newService(t, nil, users{}, places{visible: false}, &graph{})
	_, err := s.FriendsAt(context.Background(), me.Hex(), bson.NewObjectID().Hex())
	assertStatus(t, err, 404)
	_, err = s.FriendsAt(context.Background(), me.Hex(), "x' OR 1=1")
	assertStatus(t, err, 404)

	rel := relations{{From: me, To: bson.NewObjectID(), Status: amistades.StatusAccepted}}
	s = newService(t, rel, users{}, places{visible: true}, &graph{err: errors.New("caído")})
	out, err := s.FriendsAt(context.Background(), me.Hex(), bson.NewObjectID().Hex())
	require.NoError(t, err)
	assert.True(t, out.Degraded)
	assert.Zero(t, out.Total)
	assert.NotNil(t, out.Friends)
}

func TestService_BadSubjectIsUnauthorized(t *testing.T) {
	s := newService(t, nil, users{}, places{}, nil)
	_, err := s.Suggestions(context.Background(), "nadie")
	assertStatus(t, err, 401)
	_, err = s.FriendsAt(context.Background(), "nadie", bson.NewObjectID().Hex())
	assertStatus(t, err, 401)
}

func assertStatus(t *testing.T, err error, status int) {
	t.Helper()
	var e *apperr.Error
	require.ErrorAs(t, err, &e)
	assert.Equal(t, status, e.Kind.Status())
}

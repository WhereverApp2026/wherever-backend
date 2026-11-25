package negocios

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestMarks_WithSessionEveryCardSaysTrueOrFalse(t *testing.T) {
	saved, other := bson.NewObjectID(), bson.NewObjectID()
	m := NewMarks([]bson.ObjectID{saved})

	require.NotNil(t, m.Of(saved))
	assert.True(t, *m.Of(saved))
	require.NotNil(t, m.Of(other))
	assert.False(t, *m.Of(other))
}

func TestMarks_WithoutSessionTheFieldIsOmitted(t *testing.T) {
	var m Marks
	assert.Nil(t, m.Of(bson.NewObjectID()))

	user := bson.NewObjectID()
	got, err := LoadMarks(context.Background(), nil, &user, []bson.ObjectID{bson.NewObjectID()})
	require.NoError(t, err)
	assert.Nil(t, got, "sin la dependencia no hay marcas")

	got, err = LoadMarks(context.Background(), nil, nil, []bson.ObjectID{bson.NewObjectID()})
	require.NoError(t, err)
	assert.Nil(t, got, "sin sesión no hay marcas")

	raw, err := json.Marshal(ExploreItem{IsFavorite: got.Of(bson.NewObjectID())})
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "es_favorito")
}

func TestExploreItem_WithSessionSerializesFalse(t *testing.T) {
	raw, err := json.Marshal(ExploreItem{IsFavorite: Marks{}.Of(bson.NewObjectID())})
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"es_favorito":false`)
}

func TestMarkFavorites_WithoutViewerLeavesCardsUntouched(t *testing.T) {
	s := &Service{}
	items := []ExploreItem{{}}
	require.NoError(t, s.markFavorites(context.Background(), []placeRow{{}}, items, nil))
	assert.Nil(t, items[0].IsFavorite)
}

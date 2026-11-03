package database

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	mongodrv "go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/WhereverApp2026/wherever-backend/internal/auth"
	"github.com/WhereverApp2026/wherever-backend/internal/indexes"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// expected son los índices de las hojas A1 (los de nv-04) y J1, más ix_refresh_previo.
var expected = map[string][]string{
	usuarios.Collection:     {"_id_", "ux_correo", "ux_google_id"},
	auth.SessionsCollection: {"_id_", "ix_familia", "ix_refresh_previo", "ix_usuario_sesiones", "ttl_sesion", "ux_refresh_hash"},
	auth.TokensCollection:   {"_id_", "ix_usuario_tipo", "ttl_token", "ux_token_hash"},
}

func freshDB(t *testing.T) *mongodrv.Database {
	t.Helper()
	db := mongoClient.DB()
	require.NoError(t, db.Drop(context.Background()))
	return db
}

func specs(t *testing.T, db *mongodrv.Database, collection string) map[string]mongodrv.IndexSpecification {
	t.Helper()
	list, err := db.Collection(collection).Indexes().ListSpecifications(context.Background())
	require.NoError(t, err)
	out := map[string]mongodrv.IndexSpecification{}
	for _, s := range list {
		out[s.Name] = s
	}
	return out
}

func TestIndexes_CreatesExactlyTheDeclaredOnes(t *testing.T) {
	db := freshDB(t)

	require.NoError(t, mongo.EnsureIndexes(context.Background(), db, indexes.All()...))

	for collection, names := range expected {
		got := specs(t, db, collection)
		gotNames := make([]string, 0, len(got))
		for name := range got {
			gotNames = append(gotNames, name)
		}
		assert.ElementsMatch(t, names, gotNames, collection)
	}
}

func TestIndexes_AreIdempotent(t *testing.T) {
	db := freshDB(t)
	ctx := context.Background()

	require.NoError(t, mongo.EnsureIndexes(ctx, db, indexes.All()...))
	require.NoError(t, mongo.EnsureIndexes(ctx, db, indexes.All()...), "correr el script dos veces no falla")

	assert.Len(t, specs(t, db, auth.SessionsCollection), len(expected[auth.SessionsCollection]))
}

func TestIndexes_UniqueTTLAndPartial(t *testing.T) {
	db := freshDB(t)
	require.NoError(t, mongo.EnsureIndexes(context.Background(), db, indexes.All()...))

	for collection, names := range map[string][]string{
		usuarios.Collection:     {"ux_correo", "ux_google_id"},
		auth.SessionsCollection: {"ux_refresh_hash"},
		auth.TokensCollection:   {"ux_token_hash"},
	} {
		got := specs(t, db, collection)
		for _, name := range names {
			require.NotNil(t, got[name].Unique, name)
			assert.True(t, *got[name].Unique, name)
		}
	}
	for collection, name := range map[string]string{auth.SessionsCollection: "ttl_sesion", auth.TokensCollection: "ttl_token"} {
		ttl := specs(t, db, collection)[name]
		require.NotNil(t, ttl.ExpireAfterSeconds, name)
		assert.Equal(t, int32(0), *ttl.ExpireAfterSeconds, name)
	}
}

// ux_google_id es parcial: muchas cuentas sin google_id no chocan, dos con el mismo sí.
func TestIndexes_PartialGoogleIDAllowsManyWithout(t *testing.T) {
	db := freshDB(t)
	ctx := context.Background()
	require.NoError(t, mongo.EnsureIndexes(ctx, db, indexes.All()...))
	col := db.Collection(usuarios.Collection)

	_, err := col.InsertMany(ctx, []any{bson.D{{Key: "correo", Value: "a@x.test"}}, bson.D{{Key: "correo", Value: "b@x.test"}}})
	require.NoError(t, err)
	_, err = col.InsertOne(ctx, bson.D{{Key: "correo", Value: "c@x.test"}, {Key: "google_id", Value: "g-1"}})
	require.NoError(t, err)
	_, err = col.InsertOne(ctx, bson.D{{Key: "correo", Value: "d@x.test"}, {Key: "google_id", Value: "g-1"}})
	assert.True(t, mongodrv.IsDuplicateKeyError(err))
	_, err = col.InsertOne(ctx, bson.D{{Key: "correo", Value: "a@x.test"}})
	assert.True(t, mongodrv.IsDuplicateKeyError(err), "ux_correo")
}

package database

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	mongodrv "go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/asistente"
	"github.com/WhereverApp2026/wherever-backend/internal/auth"
	"github.com/WhereverApp2026/wherever-backend/internal/busquedas"
	"github.com/WhereverApp2026/wherever-backend/internal/ciudades"
	"github.com/WhereverApp2026/wherever-backend/internal/grafo"
	"github.com/WhereverApp2026/wherever-backend/internal/grupos"
	"github.com/WhereverApp2026/wherever-backend/internal/indexes"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/notificaciones"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
	"github.com/WhereverApp2026/wherever-backend/internal/privacidad"
	"github.com/WhereverApp2026/wherever-backend/internal/recomendaciones"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
	"github.com/WhereverApp2026/wherever-backend/internal/votaciones"
)

// expected son los índices de las hojas A1 y J1 (incluido ix_refresh_previo) más los de
// `exportaciones` (colección nueva de nv-05), el único de `descartes_usuario` (nv-24) los de `amistades` (nv-36), la cola `grafo_pendiente` (nv-52), `busquedas_recientes` (nv-22), `consentimientos` (nv-19) y `grupos` (nv-37).
var expected = map[string][]string{
	usuarios.Collection:              {"_id_", "ux_correo", "ux_google_id", "ux_codigo_amigo", "ix_nombre_busqueda", "ix_ubicacion", "ix_eliminacion", "ux_telefono_hash", "ux_nombre_usuario"},
	ciudades.Collection:              {"_id_", "ux_ciudad", "ix_centro"},
	privacidad.ExportsCollection:     {"_id_", "ix_exportacion_estado", "ix_usuario_exportaciones", "ttl_exportacion"},
	auth.SessionsCollection:          {"_id_", "ix_familia", "ix_refresh_previo", "ix_usuario_sesiones", "ttl_sesion", "ux_refresh_hash"},
	auth.TokensCollection:            {"_id_", "ix_usuario_tipo", "ttl_token", "ux_token_hash"},
	negocios.Collection:              {"_id_", "ix_geo_filtros", "ix_texto", "ix_autocompletar", "ix_categoria_calif", "ux_slug", "ix_propietario", "ix_destacados", "ux_qr_checkin", "ix_revision"},
	planes.Collection:                {"_id_", "ix_dueno_estado_fecha", "ix_estado_fecha"},
	asistente.DescartesCollection:    {"_id_", "ttl_descarte", "ix_motivo_fecha"},
	recomendaciones.Collection:       {"_id_", "ux_usuario_negocio"},
	amistades.Collection:             {"_id_", "ux_par", "ix_de_estado", "ix_para_estado"},
	grafo.Collection:                 {"_id_", "ix_disponible_en"},
	busquedas.Collection:             {"_id_", "ux_usuario_texto", "ix_usuario_fecha", "ttl_buscado_en"},
	usuarios.ConsentsCollection:      {"_id_", "ix_usuario_fecha"},
	grupos.Collection:                {"_id_", "ux_plan", "ix_integrante_actualizado", "ux_codigo_hash"},
	votaciones.Collection:            {"_id_", "ux_parada_abierta", "ix_grupo_creada", "ix_estado_cierra"},
	notificaciones.Collection:        {"_id_", "ix_usuario_creada", "ttl_notificacion"},
	notificaciones.DevicesCollection: {"_id_", "ux_token_hash", "ix_usuario_uso"},
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
		usuarios.Collection:              {"ux_correo", "ux_google_id", "ux_codigo_amigo", "ux_telefono_hash", "ux_nombre_usuario"},
		ciudades.Collection:              {"ux_ciudad"},
		auth.SessionsCollection:          {"ux_refresh_hash"},
		auth.TokensCollection:            {"ux_token_hash"},
		negocios.Collection:              {"ux_slug", "ux_qr_checkin"},
		recomendaciones.Collection:       {"ux_usuario_negocio"},
		grupos.Collection:                {"ux_plan", "ux_codigo_hash"},
		votaciones.Collection:            {"ux_parada_abierta"},
		notificaciones.DevicesCollection: {"ux_token_hash"},
	} {
		got := specs(t, db, collection)
		for _, name := range names {
			require.NotNil(t, got[name].Unique, name)
			assert.True(t, *got[name].Unique, name)
		}
	}
	for collection, name := range map[string]string{auth.SessionsCollection: "ttl_sesion", auth.TokensCollection: "ttl_token", privacidad.ExportsCollection: "ttl_exportacion", asistente.DescartesCollection: "ttl_descarte"} {
		ttl := specs(t, db, collection)[name]
		require.NotNil(t, ttl.ExpireAfterSeconds, name)
		assert.Equal(t, int32(0), *ttl.ExpireAfterSeconds, name)
	}
}

func TestIndexes_NegociosTextAndPartial(t *testing.T) {
	db := freshDB(t)
	ctx := context.Background()
	require.NoError(t, mongo.EnsureIndexes(ctx, db, indexes.All()...))

	cur, err := db.Collection(negocios.Collection).Indexes().List(ctx)
	require.NoError(t, err)
	var docs []bson.M
	require.NoError(t, cur.All(ctx, &docs))
	byName := make(map[string]bson.M, len(docs))
	for _, doc := range docs {
		name, ok := doc["name"].(string)
		require.True(t, ok)
		byName[name] = doc
	}
	text, ok := byName["ix_texto"]
	require.True(t, ok)
	assert.NotNil(t, text["weights"])
	assert.Equal(t, "spanish", text["default_language"])
	partial, ok := byName["ix_destacados"]
	require.True(t, ok)
	assert.NotNil(t, partial["partialFilterExpression"])
}

// ux_google_id y ux_telefono_hash son parciales: muchas cuentas sin el campo no chocan, dos
// con el mismo valor sí. ux_codigo_amigo no es parcial: toda cuenta trae su código.
func TestIndexes_PartialGoogleIDAllowsManyWithout(t *testing.T) {
	db := freshDB(t)
	ctx := context.Background()
	require.NoError(t, mongo.EnsureIndexes(ctx, db, indexes.All()...))
	col := db.Collection(usuarios.Collection)

	_, err := col.InsertMany(ctx, []any{bson.D{{Key: "correo", Value: "a@x.test"}, {Key: "codigo_amigo", Value: "AAAA2222"}}, bson.D{{Key: "correo", Value: "b@x.test"}, {Key: "codigo_amigo", Value: "BBBB2222"}}})
	require.NoError(t, err)
	_, err = col.InsertOne(ctx, bson.D{{Key: "correo", Value: "c@x.test"}, {Key: "codigo_amigo", Value: "CCCC2222"}, {Key: "google_id", Value: "g-1"}})
	require.NoError(t, err)
	_, err = col.InsertOne(ctx, bson.D{{Key: "correo", Value: "d@x.test"}, {Key: "codigo_amigo", Value: "DDDD2222"}, {Key: "google_id", Value: "g-1"}})
	assert.True(t, mongodrv.IsDuplicateKeyError(err))
	_, err = col.InsertOne(ctx, bson.D{{Key: "correo", Value: "a@x.test"}, {Key: "codigo_amigo", Value: "EEEE2222"}})
	assert.True(t, mongodrv.IsDuplicateKeyError(err), "ux_correo")
}

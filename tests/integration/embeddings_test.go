package integration

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/embeddings"
	"github.com/WhereverApp2026/wherever-backend/internal/ia"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/iatest"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// Recálculo de vectores (nv-50) con MongoDB y Redis reales; Gemini es un servidor local.
func recalculador(t *testing.T, env *authtest.Env, porMinuto int) (*embeddings.Recalculador, *iatest.ServidorVectores) {
	t.Helper()
	srv := iatest.NuevoServidorVectores(t)
	g, err := ia.NewGeminiVectores(ia.GeminiVectoresConfig{APIKey: iatest.LlaveFalsa, ModeloID: "gemini-embedding-2", Timeout: 2 * time.Second, URLBase: srv.URL})
	require.NoError(t, err)
	lim, err := ia.NewVectoresLimitados(g, ia.NewLimitadorRedis(env.Redis), ia.LimitesVectores{PorMinuto: porMinuto, PorDia: 1000})
	require.NoError(t, err)
	rec, err := embeddings.NewRecalculador(embeddings.NewRepository(env.Mongo.DB()), lim, 2)
	require.NoError(t, err)
	return rec, srv
}

func vectoresGuardados(t *testing.T, env *authtest.Env) map[bson.ObjectID]embeddings.Vector {
	t.Helper()
	cur, err := env.Mongo.DB().Collection(embeddings.Collection).Find(context.Background(), bson.D{})
	require.NoError(t, err)
	var docs []embeddings.Vector
	require.NoError(t, cur.All(context.Background(), &docs))
	out := make(map[bson.ObjectID]embeddings.Vector, len(docs))
	for _, d := range docs {
		out[d.NegocioID] = d
	}
	return out
}

// cincoNegocios son 3 aprobados más un borrador y un eliminado, que no se vectorizan.
func cincoNegocios(t *testing.T, env *authtest.Env) []negocios.Business {
	t.Helper()
	return insertPlaces(t, env, []placeSpec{
		{slug: "emb-a"}, {slug: "emb-b", category: "bar"}, {slug: "emb-c", category: "cafeteria"},
		{slug: "emb-borrador", state: negocios.StatusDraft}, {slug: "emb-eliminado", deleted: true},
	})
}

func TestRecalcular_SoloVisiblesEIdempotente(t *testing.T) {
	env := negociosEnv(t)
	docs := cincoNegocios(t, env)
	rec, srv := recalculador(t, env, 80)
	ctx := context.Background()

	rep, err := rec.Recalcular(ctx, false)
	require.NoError(t, err)
	assert.Equal(t, embeddings.Reporte{Visibles: 3, Calculados: 3}, rep)
	assert.Equal(t, 2, srv.Llamadas(), "lotes de 2")
	guardados := vectoresGuardados(t, env)
	require.Len(t, guardados, 3)
	for _, b := range docs[:3] {
		v := guardados[b.ID]
		assert.Len(t, v.Vector, ia.Dimensiones)
		assert.Equal(t, "gemini/gemini-embedding-2", v.Modelo)
		assert.True(t, strings.HasPrefix(v.Version, "v1:gemini/gemini-embedding-2:"))
	}

	rep, err = rec.Recalcular(ctx, false)
	require.NoError(t, err)
	assert.Equal(t, embeddings.Reporte{Visibles: 3}, rep, "sin cambios no recalcula")
	assert.Equal(t, 2, srv.Llamadas())
}

func TestRecalcular_SoloElDesactualizadoYTodosConLaBandera(t *testing.T) {
	env := negociosEnv(t)
	docs := cincoNegocios(t, env)
	rec, srv := recalculador(t, env, 80)
	ctx := context.Background()
	_, err := rec.Recalcular(ctx, false)
	require.NoError(t, err)
	antes := len(srv.Textos())

	_, err = env.Mongo.DB().Collection(negocios.Collection).UpdateByID(ctx, docs[1].ID,
		bson.D{{Key: "$set", Value: bson.D{{Key: "descripcion", Value: "Bar de mezcal con música en vivo"}}}})
	require.NoError(t, err)
	rep, err := rec.Recalcular(ctx, false)
	require.NoError(t, err)
	assert.Equal(t, 1, rep.Calculados)
	nuevos := srv.Textos()[antes:]
	require.Len(t, nuevos, 1)
	assert.Contains(t, nuevos[0], "mezcal")

	rep, err = rec.Recalcular(ctx, true)
	require.NoError(t, err)
	assert.Equal(t, 3, rep.Calculados)
}

func TestRecalcular_BorraVectoresDeNegociosQueDejanDeSerVisibles(t *testing.T) {
	env := negociosEnv(t)
	docs := cincoNegocios(t, env)
	rec, _ := recalculador(t, env, 80)
	ctx := context.Background()
	_, err := rec.Recalcular(ctx, false)
	require.NoError(t, err)

	_, err = env.Mongo.DB().Collection(negocios.Collection).UpdateByID(ctx, docs[0].ID,
		bson.D{{Key: "$set", Value: bson.D{{Key: "estado", Value: negocios.StatusPaused}}}})
	require.NoError(t, err)
	rep, err := rec.Recalcular(ctx, false)

	require.NoError(t, err)
	assert.Equal(t, int64(1), rep.Borrados)
	assert.NotContains(t, vectoresGuardados(t, env), docs[0].ID)
}

func TestRecalcular_FallaOLimiteNoRompenYSigueDondeQuedo(t *testing.T) {
	env := negociosEnv(t)
	cincoNegocios(t, env)
	rec, srv := recalculador(t, env, 2)
	ctx := context.Background()

	// Límite de 2 por minuto: el primer lote pasa, el segundo no llega al proveedor.
	rep, err := rec.Recalcular(ctx, false)
	require.ErrorIs(t, err, ia.ErrLimiteGlobal)
	assert.Equal(t, 2, rep.Calculados)
	assert.Equal(t, 1, srv.Llamadas())

	// Con el proveedor caído no se guarda nada a medias.
	rec, srv = recalculador(t, env, 80)
	srv.Responder(http.StatusInternalServerError)
	_, err = rec.Recalcular(ctx, false)
	require.ErrorIs(t, err, ia.ErrProveedor)
	assert.Len(t, vectoresGuardados(t, env), 2)

	// Al volver, solo calcula el que faltaba.
	srv.Responder(http.StatusOK)
	rep, err = rec.Recalcular(ctx, false)
	require.NoError(t, err)
	assert.Equal(t, 1, rep.Calculados)
}

func TestImportar_CalculaVectoresYSiFallanLaSemillaSaleIgual(t *testing.T) {
	env := negociosEnv(t)
	admin := env.CreateAdmin(t, "admin-embeddings@wherever.test")
	rec, srv := recalculador(t, env, 80)
	db := env.Mongo.DB()
	svc := negocios.NewService(negocios.NewRepository(db), usuarios.NewRepository(db), slog.New(slog.NewTextHandler(io.Discard, nil))).WithEmbeddings(rec)
	ctx := context.Background()

	report, err := svc.Import(ctx, admin.ID.Hex(), strings.NewReader(seedCSV(3)), "csv")
	require.NoError(t, err)
	assert.Equal(t, 3, report.Created)
	assert.Len(t, vectoresGuardados(t, env), 3)
	for _, texto := range srv.Textos() {
		assert.NotContains(t, texto, "5512345678", "el teléfono no viaja")
		assert.NotContains(t, texto, "Avenida Ejemplo", "la dirección no viaja")
	}

	srv.Responder(http.StatusServiceUnavailable)
	report, err = svc.Import(ctx, admin.ID.Hex(), strings.NewReader(seedCSV(4)), "csv")
	require.NoError(t, err, "si fallan los vectores la importación sale igual")
	assert.Equal(t, 1, report.Created)
	assert.Len(t, vectoresGuardados(t, env), 3)
}

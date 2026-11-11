package ia_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/ia"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/iatest"
)

func vectores(t *testing.T) (*ia.GeminiVectores, *iatest.ServidorVectores) {
	t.Helper()
	srv := iatest.NuevoServidorVectores(t)
	g, err := ia.NewGeminiVectores(ia.GeminiVectoresConfig{APIKey: iatest.LlaveFalsa, ModeloID: "gemini-embedding-2", Timeout: time.Second, URLBase: srv.URL})
	require.NoError(t, err)
	return g, srv
}

func TestGeminiVectores_DocumentosConFormatoYLlaveEnEncabezado(t *testing.T) {
	g, srv := vectores(t)

	vs, err := g.Documentos(context.Background(), []ia.Documento{
		{Titulo: "Café Luna", Texto: "Cafeterías. Etiquetas: café."},
		{Texto: "Bares."},
	})

	require.NoError(t, err)
	require.Len(t, vs, 2)
	assert.Len(t, vs[0], ia.Dimensiones)
	assert.Equal(t, []string{"title: Café Luna | text: Cafeterías. Etiquetas: café.", "title: none | text: Bares."}, srv.Textos())
	assert.Equal(t, []string{"/models/gemini-embedding-2:batchEmbedContents"}, srv.Rutas(), "una sola llamada y sin la llave en la URL")
	assert.Equal(t, []string{iatest.LlaveFalsa}, srv.Llaves())
	assert.Equal(t, "gemini/gemini-embedding-2", g.Nombre())
}

func TestGeminiVectores_ConsultaAnonimizadaConPrefijo(t *testing.T) {
	g, srv := vectores(t)

	v, err := g.Consulta(context.Background(), "tacos baratos, llámame al 771 123 4567 o ana@example.com")

	require.NoError(t, err)
	assert.Len(t, v, ia.Dimensiones)
	textos := srv.Textos()
	require.Len(t, textos, 1)
	assert.True(t, strings.HasPrefix(textos[0], "task: search result | query: tacos baratos"))
	assert.NotContains(t, textos[0], "771")
	assert.NotContains(t, textos[0], "ana@example.com")
}

func TestGeminiVectores_ErroresDelProveedor(t *testing.T) {
	casos := map[int]error{
		http.StatusTooManyRequests:     ia.ErrLimiteProveedor,
		http.StatusInternalServerError: ia.ErrProveedor,
		http.StatusBadRequest:          ia.ErrProveedor,
	}
	for estado, want := range casos {
		g, srv := vectores(t)
		srv.Responder(estado)
		_, err := g.Documentos(context.Background(), []ia.Documento{{Texto: "x"}})
		assert.ErrorIs(t, err, want, "HTTP %d", estado)
	}
}

func TestGeminiVectores_LoteMayorAlMaximoNoLlama(t *testing.T) {
	g, srv := vectores(t)
	_, err := g.Documentos(context.Background(), make([]ia.Documento, ia.MaxLote+1))
	require.Error(t, err)
	assert.Zero(t, srv.Llamadas())
}

func TestNormalizarVector(t *testing.T) {
	v := make([]float32, ia.Dimensiones)
	v[0], v[1] = 3, 4
	n, err := ia.NormalizarVector(v)
	require.NoError(t, err)
	assert.InDelta(t, 0.6, n[0], 1e-6)
	assert.InDelta(t, 0.8, n[1], 1e-6)

	_, err = ia.NormalizarVector(make([]float32, 10))
	require.ErrorIs(t, err, ia.ErrVectorInvalido, "otra dimensión")
	_, err = ia.NormalizarVector(make([]float32, ia.Dimensiones))
	require.ErrorIs(t, err, ia.ErrVectorInvalido, "vector nulo")
}

func TestNewGeminiVectores_SinLlaveOModelo(t *testing.T) {
	_, err := ia.NewGeminiVectores(ia.GeminiVectoresConfig{ModeloID: "m"})
	require.Error(t, err)
	_, err = ia.NewGeminiVectores(ia.GeminiVectoresConfig{APIKey: "k"})
	require.Error(t, err)
}

// sumador cuenta en memoria; es el doble del contador de Redis para la prueba unitaria.
type sumador struct {
	totales map[string]int64
	err     error
}

func (s *sumador) Sumar(_ context.Context, n int64, ventanas ...ia.Ventana) ([]ia.Cuenta, error) {
	if s.err != nil {
		return nil, s.err
	}
	out := make([]ia.Cuenta, len(ventanas))
	for i, v := range ventanas {
		s.totales[v.Llave] += n
		out[i] = ia.Cuenta{Total: s.totales[v.Llave], Resta: v.Vida}
	}
	return out, nil
}

func TestVectoresLimitados_CadaTextoCuentaYAlPasarseNoLlama(t *testing.T) {
	g, srv := vectores(t)
	s := &sumador{totales: map[string]int64{}}
	lim, err := ia.NewVectoresLimitados(g, s, ia.LimitesVectores{PorMinuto: 3, PorDia: 100})
	require.NoError(t, err)
	ctx := context.Background()

	_, err = lim.Documentos(ctx, []ia.Documento{{Texto: "a"}, {Texto: "b"}})
	require.NoError(t, err)
	_, err = lim.Consulta(ctx, "c")
	require.NoError(t, err)
	_, err = lim.Consulta(ctx, "d")

	require.ErrorIs(t, err, ia.ErrLimiteGlobal)
	assert.Equal(t, 2, srv.Llamadas(), "la cuarta no llega al proveedor")
	for llave := range s.totales {
		assert.True(t, strings.HasPrefix(llave, "rate:ia_emb_"), llave)
	}
}

func TestVectoresLimitados_SinRedisSeCierra(t *testing.T) {
	g, srv := vectores(t)
	lim, err := ia.NewVectoresLimitados(g, &sumador{err: errors.New("redis caído")}, ia.LimitesVectores{PorMinuto: 3, PorDia: 3})
	require.NoError(t, err)

	_, err = lim.Consulta(context.Background(), "x")

	require.ErrorIs(t, err, ia.ErrLimiteDesconocido)
	assert.Zero(t, srv.Llamadas())
}

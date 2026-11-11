package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/recomendador"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
)

// recoSpec es un negocio de la prueba del recomendador con lo que nv-21 no fija.
type recoSpec struct {
	placeSpec
	tags      []string
	allergens []string
	reviews   int
}

// diezNegocios son los 10 negocios fijos de la spec: 5 pasan los filtros y 5 no (alergia,
// sin dato de alérgenos, cerrado, borrador y fuera del radio). 0.0045° ≈ 500 m.
func diezNegocios() []recoSpec {
	ninguno := []string{negocios.AllergenNone}
	cerrado := []negocios.VentanaMin{{Dia: 1, Desde: 18 * 60, Hasta: 23 * 60}}
	cierraPronto := []negocios.VentanaMin{{Dia: 1, Desde: 9 * 60, Hasta: 12*60 + 20}}
	return []recoSpec{
		{placeSpec{slug: "cafe-ideal", category: "cafeteria", avgPrice: 150, rating: 4.5, dLat: 0.0045}, []string{"postres"}, ninguno, 20},
		{placeSpec{slug: "cafe-caro", category: "cafeteria", avgPrice: 300, rating: 4.5, dLat: 0.0045}, []string{"postres"}, ninguno, 20},
		{placeSpec{slug: "cafe-cierra-pronto", category: "cafeteria", avgPrice: 150, rating: 4, dLat: 0.0045, windows: cierraPronto}, []string{"postres"}, ninguno, 20},
		{placeSpec{slug: "rest-cerca", category: "restaurante", avgPrice: 150, rating: 4}, nil, []string{"gluten"}, 10},
		{placeSpec{slug: "bar-lejos", category: "bar", avgPrice: 150, rating: 5, dLat: 0.036}, []string{"postres"}, ninguno, 50},
		{placeSpec{slug: "cafe-cacahuate", category: "cafeteria", avgPrice: 150, rating: 5}, []string{"postres"}, []string{"cacahuate"}, 50},
		{placeSpec{slug: "cafe-sin-dato", category: "cafeteria", avgPrice: 150, rating: 5}, []string{"postres"}, nil, 50},
		{placeSpec{slug: "cafe-cerrado", category: "cafeteria", avgPrice: 150, rating: 5, windows: cerrado}, []string{"postres"}, ninguno, 50},
		{placeSpec{slug: "cafe-borrador", category: "cafeteria", avgPrice: 150, rating: 5, state: negocios.StatusDraft}, []string{"postres"}, ninguno, 50},
		{placeSpec{slug: "cafe-lejano", category: "cafeteria", avgPrice: 150, rating: 5, dLat: 0.06}, []string{"postres"}, ninguno, 50},
	}
}

func insertReco(t *testing.T) *negocios.Repository {
	t.Helper()
	return negocios.NewRepository(insertRecoEnv(t).Mongo.DB())
}

// insertRecoEnv carga los 10 negocios y devuelve el entorno con la app (nv-24).
func insertRecoEnv(t *testing.T) *authtest.Env {
	t.Helper()
	env := negociosEnv(t)
	docs := make([]any, 0, 10)
	for _, s := range diezNegocios() {
		b := placeDoc(s.placeSpec)
		b.Tags, b.Allergens = s.tags, s.allergens
		b.Rating.TotalResenas = s.reviews
		docs = append(docs, b)
	}
	_, err := env.Mongo.DB().Collection(negocios.Collection).InsertMany(context.Background(), docs)
	require.NoError(t, err)
	return env
}

func recoCtx() recomendador.Contexto {
	ids, unknown := recomendador.ParseAlergias("cacahuate")
	return recomendador.Contexto{
		Lat: centerLat, Lng: centerLng, Momento: lunesMediodia,
		PresupuestoMin: 100, PresupuestoMax: 200, Gustos: []string{"Cafeterías", "postres"},
		Alergenos: ids, AlergiaSinCatalogo: unknown,
	}
}

func recoSlugs(recs []recomendador.Recomendacion) []string {
	out := make([]string, 0, len(recs))
	for _, r := range recs {
		out = append(out, r.Negocio.Slug)
	}
	return out
}

func TestRecomendador_OrdenEsperadoConDiezNegocios(t *testing.T) {
	repo := insertReco(t)
	pesos, err := recomendador.PesosPorDefecto()
	require.NoError(t, err)
	recs, err := recomendador.Recomendar(context.Background(), repo, pesos, recoCtx(), 10)
	require.NoError(t, err)
	assert.Equal(t, []string{"cafe-ideal", "cafe-caro", "cafe-cierra-pronto", "rest-cerca", "bar-lejos"}, recoSlugs(recs))
	assert.Equal(t, []float64{91.5, 81.5, 80, 67, 66.5}, []float64{recs[0].Puntaje, recs[1].Puntaje, recs[2].Puntaje, recs[3].Puntaje, recs[4].Puntaje})
	assert.Equal(t, []string{"gustos", "presupuesto", "distancia"}, recs[0].Razones)
	assert.Equal(t, "Te lo recomendamos porque coincide con tus gustos, entra en tu presupuesto y está cerca de ti.", recs[0].PorQue)

	again, err := recomendador.Recomendar(context.Background(), repo, pesos, recoCtx(), 10)
	require.NoError(t, err)
	assert.Equal(t, recs, again, "determinista: misma entrada, mismo resultado")
}

func TestRecomendador_NingunaAlergiaSeCuela(t *testing.T) {
	repo := insertReco(t)
	pesos, err := recomendador.PesosPorDefecto()
	require.NoError(t, err)
	for _, raw := range []string{"cacahuate", "gluten y cacahuate", "kiwi"} {
		c := recoCtx()
		c.Alergenos, c.AlergiaSinCatalogo = recomendador.ParseAlergias(raw)
		recs, err := recomendador.Recomendar(context.Background(), repo, pesos, c, 10)
		require.NoError(t, err)
		require.NotEmpty(t, recs, raw)
		for _, r := range recs {
			assert.NotEmpty(t, r.Negocio.Allergens, "%s: %s sin dato de alérgenos", raw, r.Negocio.Slug)
			for _, a := range r.Negocio.Allergens {
				assert.NotContains(t, c.Alergenos, a, "%s: %s", raw, r.Negocio.Slug)
			}
			if c.AlergiaSinCatalogo {
				assert.Equal(t, []string{negocios.AllergenNone}, r.Negocio.Allergens, r.Negocio.Slug)
			}
		}
	}
}

func TestRecomendador_SinAlergiasEntraElSinDato(t *testing.T) {
	repo := insertReco(t)
	pesos, err := recomendador.PesosPorDefecto()
	require.NoError(t, err)
	c := recoCtx()
	c.Alergenos = nil
	recs, err := recomendador.Recomendar(context.Background(), repo, pesos, c, 10)
	require.NoError(t, err)
	assert.Contains(t, recoSlugs(recs), "cafe-sin-dato")
	assert.Contains(t, recoSlugs(recs), "cafe-cacahuate")
	assert.NotContains(t, recoSlugs(recs), "cafe-cerrado")
	assert.NotContains(t, recoSlugs(recs), "cafe-borrador")
	assert.NotContains(t, recoSlugs(recs), "cafe-lejano")
}

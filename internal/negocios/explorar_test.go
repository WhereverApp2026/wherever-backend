package negocios

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

// Los filtros embebidos se leen del query string igual que los campos propios.
func TestExploreFilters_QueryParserLeeLosEmbebidos(t *testing.T) {
	app := fiber.New()
	var got ListQuery
	app.Get("/", func(c *fiber.Ctx) error { return c.QueryParser(&got) })
	resp, err := app.Test(httptest.NewRequest("GET", "/?categorias=cafeteria,bar&precio=1,2&con_sello=true&con_cupon=1&de_amigos=true&categoria=bar&precio_max=3&abierto_ahora=true&limite=5", nil))
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, ExploreFilters{Category: "bar", Categories: "cafeteria,bar", MaxPrice: 3, Prices: "1,2", OpenNow: true, Seal: true, Coupon: true, Friends: true}, got.ExploreFilters)
	assert.Equal(t, 5, got.Limit)
}

func TestExploreFilters_ValidatorRevisaLosEmbebidos(t *testing.T) {
	v, err := validation.New()
	require.NoError(t, err)
	q := ListQuery{Limit: 10}
	q.Categories = string(make([]byte, 301))
	assert.Error(t, v.Struct(q))
	q = ListQuery{Limit: 10}
	q.MaxPrice = 5
	assert.Error(t, v.Struct(q))
	assert.NoError(t, v.Struct(ListQuery{Limit: 10, ExploreFilters: ExploreFilters{Categories: "cafeteria,bar"}}))
}

func TestCategoriesOf_UneSinRepetirYRechazaDesconocidas(t *testing.T) {
	got, err := categoriesOf(ExploreFilters{Category: "cafeteria", Categories: " bar ,cafeteria,,restaurante"})
	require.NoError(t, err)
	assert.Equal(t, []string{"cafeteria", "bar", "restaurante"}, got)

	_, err = categoriesOf(ExploreFilters{Categories: "cafeteria,{\"$ne\":1}"})
	assertField(t, err, "categoria")
	_, err = categoriesOf(ExploreFilters{Categories: "restaurante,cafeteria,bar,entretenimiento,servicios,comercio,otro"})
	require.Error(t, err)
	none, err := categoriesOf(ExploreFilters{})
	require.NoError(t, err)
	assert.Empty(t, none)
}

func TestPricesOf_NivelesDe1A4(t *testing.T) {
	got, err := pricesOf("3, 1,3")
	require.NoError(t, err)
	assert.Equal(t, []int{1, 3}, got)
	for _, bad := range []string{"0", "5", "a", "1,,2", "-1"} {
		_, err := pricesOf(bad)
		assertField(t, err, "precio")
	}
	none, err := pricesOf("")
	require.NoError(t, err)
	assert.Nil(t, none)
}

func assertField(t *testing.T, err error, field string) {
	t.Helper()
	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, 422, appErr.Kind.Status())
	require.NotEmpty(t, appErr.Fields)
	assert.Equal(t, field, appErr.Fields[0].Field)
}

func TestMatchConditions_SelloCuponYCategorias(t *testing.T) {
	s := &Service{now: time.Now}
	match, err := s.matchConditions(context.Background(), ExploreFilters{Categories: "cafeteria,bar", Seal: true, Coupon: true, Prices: "2"}, nil)
	require.NoError(t, err)
	assert.Contains(t, match, bson.D{{Key: "categoria", Value: bson.D{{Key: "$in", Value: []string{"cafeteria", "bar"}}}}})
	assert.Contains(t, match, bson.D{{Key: "sello_excelencia", Value: true}})
	assert.Contains(t, match, bson.D{{Key: "cupones_activos", Value: bson.D{{Key: "$gt", Value: 0}}}})
	assert.Contains(t, match, bson.D{{Key: "nivel_precio", Value: bson.D{{Key: "$in", Value: []int{2}}}}})
}

func TestLoadViewer_DeAmigosSinSesionEs401(t *testing.T) {
	s := &Service{now: time.Now}
	_, err := s.loadViewer(context.Background(), "", ExploreFilters{Friends: true}, false)
	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, 401, appErr.Kind.Status())

	v, err := s.loadViewer(context.Background(), "", ExploreFilters{}, true)
	require.NoError(t, err)
	assert.Nil(t, v)

	_, err = s.loadViewer(context.Background(), bson.NewObjectID().Hex(), ExploreFilters{Friends: true}, false)
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, 503, appErr.Kind.Status(), "sin WithExplore no hay amigos")
}

// Con `de_amigos` el cursor queda atado a quien consulta: el de otra persona es 422.
func TestFiltersScope_DeAmigosAtaAQuienConsulta(t *testing.T) {
	signer, err := pagination.NewSigner([]byte("solo_para_pruebas_cursor_no_usar_en_entornos_0789"))
	require.NoError(t, err)
	s := (&Service{}).WithCursor(signer)
	base := listParams{Order: OrderRating, RadiusM: DefaultRadiusM}
	ana, beto := &viewer{ID: bson.NewObjectID()}, &viewer{ID: bson.NewObjectID()}
	friends := ListQuery{ExploreFilters: ExploreFilters{Friends: true}}
	assert.NotEqual(t, s.listScope(friends, base, ana), s.listScope(friends, base, beto))
	assert.Equal(t, s.listScope(ListQuery{}, base, ana), s.listScope(ListQuery{}, base, beto))
	for _, f := range []ExploreFilters{{Seal: true}, {Coupon: true}, {Categories: "cafeteria"}, {Prices: "1"}} {
		assert.NotEqual(t, s.listScope(ListQuery{}, base, nil), s.listScope(ListQuery{ExploreFilters: f}, base, nil))
	}
}

func TestCrowdLevel_TresNivelesONil(t *testing.T) {
	// Miércoles 2026-11-18 a las 14:00 en México.
	now := time.Date(2026, 11, 18, 14, 0, 0, 0, MexicoCity())
	matrix := func(v int) [][]int {
		m := make([][]int, 7)
		for d := range m {
			m[d] = make([]int, 24)
		}
		m[3][14] = v
		m[3][9] = 50
		return m
	}
	for v, want := range map[int]string{0: CrowdLow, 33: CrowdLow, 34: CrowdMedium, 66: CrowdMedium, 67: CrowdPeak, 100: CrowdPeak} {
		got := CrowdLevel(matrix(v), now)
		require.NotNil(t, got, v)
		assert.Equal(t, want, *got, v)
	}
	assert.Nil(t, CrowdLevel(nil, now), "sin curva")
	empty := make([][]int, 7)
	for d := range empty {
		empty[d] = make([]int, 24)
	}
	assert.Nil(t, CrowdLevel(empty, now), "día en ceros")
}

func TestBestCoupon_MayorAhorroConPrecioPromedio(t *testing.T) {
	end := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	pct := CouponOffer{ID: bson.NewObjectID(), Title: "20 %", Percent: 20, Ends: end}  // 200 × 20 = 4000
	monto := CouponOffer{ID: bson.NewObjectID(), Title: "$50", AmountCents: 5000, Ends: end} // 5000
	dos := CouponOffer{ID: bson.NewObjectID(), Title: "2×1", TwoForOne: true, Ends: end}    // 20000
	regalo := CouponOffer{ID: bson.NewObjectID(), Title: "Postre", Ends: end}

	best, ok := BestCoupon([]CouponOffer{pct, monto, regalo}, 200)
	require.True(t, ok)
	assert.Equal(t, "$50", best.Title)
	best, _ = BestCoupon([]CouponOffer{pct, monto, dos}, 200)
	assert.Equal(t, "2×1", best.Title)
	best, _ = BestCoupon([]CouponOffer{regalo}, 200)
	assert.Equal(t, "Postre", best.Title, "un regalo solo también se muestra")

	early := monto
	early.ID, early.Title, early.Ends = bson.NewObjectID(), "$50 antes", end.Add(-time.Hour)
	best, _ = BestCoupon([]CouponOffer{monto, early}, 200)
	assert.Equal(t, "$50 antes", best.Title, "empate: el que vence antes")

	_, ok = BestCoupon(nil, 200)
	assert.False(t, ok)
}

func TestInitials_PrimerYUltimoNombre(t *testing.T) {
	assert.Equal(t, "JR", Initials("jorge alberto rodríguez", "jorge"))
	assert.Equal(t, "Á", Initials("  ángela ", "x"))
	assert.Equal(t, "M", Initials("", "mar_92"))
	assert.Empty(t, Initials("", ""))
}

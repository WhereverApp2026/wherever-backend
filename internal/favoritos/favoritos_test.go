package favoritos

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

func TestDTO_Validacion(t *testing.T) {
	v, err := validation.New()
	require.NoError(t, err)
	id := bson.NewObjectID().Hex()
	cases := []struct {
		name string
		dto  any
		ok   bool
	}{
		{"marcar negocio", TargetParams{Type: TypeBusiness, TargetID: id}, true},
		{"marcar plan", TargetParams{Type: TypePlan, TargetID: id}, true},
		{"tipo desconocido", TargetParams{Type: "ciudad", TargetID: id}, false},
		{"sin tipo", TargetParams{TargetID: id}, false},
		{"id corto", TargetParams{Type: TypePlan, TargetID: "abc"}, false},
		{"id no hexadecimal", TargetParams{Type: TypePlan, TargetID: strings.Repeat("z", 24)}, false},
		{"lista por defecto", ListQuery{Type: TypeBusiness, Limit: NewListQuery().Limit}, true},
		{"lista sin tipo", NewListQuery(), false},
		{"límite 50", ListQuery{Type: TypePlan, Limit: 50}, true},
		{"límite 51", ListQuery{Type: TypePlan, Limit: 51}, false},
		{"límite 0", ListQuery{Type: TypePlan}, false},
		{"ids", IDsQuery{Type: TypePlan}, true},
		{"ids sin tipo", IDsQuery{}, false},
		{"buscar sin tipo", SearchQuery{Q: "ca"}, true},
		{"buscar una letra", SearchQuery{Q: "c"}, false},
		{"buscar 61", SearchQuery{Q: strings.Repeat("a", 61)}, false},
		{"buscar tipo malo", SearchQuery{Q: "cafe", Type: "otro"}, false},
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

func TestNewListQuery_LimitePorDefecto(t *testing.T) {
	assert.Equal(t, 20, NewListQuery().Limit)
}

func TestSearchQuery_Kinds(t *testing.T) {
	assert.Equal(t, []string{TypePlan}, SearchQuery{Type: TypePlan}.Kinds())
	assert.Equal(t, []string{TypeBusiness, TypePlan}, SearchQuery{}.Kinds())
}

func TestMatchesPrefix_SinAcentosNiMayusculas(t *testing.T) {
	assert.True(t, MatchesPrefix("Café Ñandú", "cafe"))
	assert.True(t, MatchesPrefix("café ñandú", "CAFÉ  ñan"))
	assert.False(t, MatchesPrefix("Bar Café", "cafe"), "solo prefijo")
	assert.False(t, MatchesPrefix("Café", "   "), "consulta vacía no coincide")
}

func TestView_NegocioRefrescaResumenYCalculaCuponYHorario(t *testing.T) {
	// Lunes 12:00 en Ciudad de México (UTC−6).
	noon := time.Date(2026, 11, 16, 18, 0, 0, 0, time.UTC)
	s := &Service{now: func() time.Time { return noon }}
	f := Favorite{Type: TypeBusiness, TargetID: bson.NewObjectID(), Summary: Summary{Name: "viejo"}, CreatedAt: noon}
	b := negocios.Business{
		Name: "Nuevo", Category: "cafeteria", Coupons: 1,
		Photos:      []negocios.Foto{{URL: "https://example.test/2.jpg", Orden: 2}, {URL: "https://example.test/1.jpg", Orden: 1}},
		ScheduleMin: []negocios.VentanaMin{{Dia: int(time.Monday), Desde: 9 * 60, Hasta: 18 * 60}},
	}
	v := s.view(f, &b)
	assert.Equal(t, "Nuevo", v.Summary.Name)
	assert.Equal(t, "cafeteria", v.Summary.Category)
	assert.Equal(t, negocios.Cover(b.Photos), v.Summary.PhotoURL)
	require.NotNil(t, v.CouponToday)
	require.NotNil(t, v.OpenNow)
	assert.True(t, *v.CouponToday)
	assert.True(t, *v.OpenNow)

	b.Coupons, b.ScheduleMin = 0, nil
	v = s.view(f, &b)
	assert.False(t, *v.CouponToday)
	assert.False(t, *v.OpenNow)
}

func TestView_PlanSoloNombre(t *testing.T) {
	s := &Service{now: time.Now}
	v := s.view(Favorite{Type: TypePlan, TargetID: bson.NewObjectID(), Summary: Summary{Name: "Salida"}}, nil)
	assert.Equal(t, "Salida", v.Summary.Name)
	assert.Nil(t, v.CouponToday)
	assert.Nil(t, v.OpenNow)
}

func TestWithCoupon_SoloConCupon(t *testing.T) {
	yes, no := true, false
	got := withCoupon([]View{{TargetID: "a", CouponToday: &yes}, {TargetID: "b", CouponToday: &no}, {TargetID: "c"}})
	require.Len(t, got, 1)
	assert.Equal(t, "a", got[0].TargetID)
	assert.NotNil(t, withCoupon(nil), "lista vacía, nunca null")
}

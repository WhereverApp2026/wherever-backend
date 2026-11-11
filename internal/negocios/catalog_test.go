package negocios

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCategoryCatalog_HasTheSixGirosInOrder(t *testing.T) {
	want := []CatalogItem{
		{ID: "restaurante", Name: "Restaurantes"},
		{ID: "cafeteria", Name: "Cafeterías"},
		{ID: "bar", Name: "Bares"},
		{ID: "entretenimiento", Name: "Entretenimiento"},
		{ID: "servicios", Name: "Servicios"},
		{ID: "comercio", Name: "Comercio"},
	}
	assert.Equal(t, want, CategoryCatalog())
	for _, item := range want {
		assert.True(t, ValidCategory(item.ID), item.ID)
		assert.Equal(t, item.Name, CategoryName(item.ID))
	}
	assert.False(t, ValidCategory("otro"))
	assert.Equal(t, "otro", CategoryName("otro"))
}

func TestCatalog_StaticFilters(t *testing.T) {
	assert.Equal(t, []int{1, 2, 3, 4}, PriceLevelCatalog())
	assert.Equal(t, []string{"vegetariano", "vegano", "sin_gluten"}, DietCatalog())
	assert.Equal(t, []string{"silla_ruedas", "apto_ninos", "pet_friendly"}, AccessibilityCatalog())
	assert.Len(t, AllergenCatalog(), 9)
	assert.True(t, ValidDietOption("vegano"))
	assert.False(t, ValidDietOption("carnivoro"))
	assert.True(t, ValidAccessibilityOption("pet_friendly"))
	assert.False(t, ValidAccessibilityOption("otra"))
	assert.True(t, ValidAllergen("ajonjoli"))
	assert.False(t, ValidAllergen("polvo"))
}

func TestFacetsResponse_CompletesSixAndMapsCounts(t *testing.T) {
	out := facetsResponse(FacetCounts{
		Categories: []FacetCount{{ID: "restaurante", Count: 3}},
		Tags:       []FacetCount{{ID: "tacos", Count: 2}},
		Services:   []FacetCount{{ID: "terraza", Count: 1}},
	})

	require.Len(t, out.Categories, 6)
	assert.Equal(t, ItemCount{ID: "restaurante", Name: "Restaurantes", Count: 3}, out.Categories[0])
	assert.Equal(t, ItemCount{ID: "cafeteria", Name: "Cafeterías", Count: 0}, out.Categories[1])
	assert.Equal(t, []ItemCount{{ID: "tacos", Name: "tacos", Count: 2}}, out.Tags)
	assert.Equal(t, []ItemCount{{ID: "terraza", Name: "terraza", Count: 1}}, out.Services)
	assert.Equal(t, PriceLevelCatalog(), out.Filters.PriceLevels)
	assert.Equal(t, DietCatalog(), out.Filters.DietOptions)
	assert.Equal(t, AccessibilityCatalog(), out.Filters.Accessibility)
}

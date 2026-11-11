// Package negocios guarda el catálogo de negocios aprobados (hojas A2 y A5, spec nv-20):
// modelo, índices, importación semilla y el catálogo de categorías que pintan los chips.
package negocios

import "slices"

// CatalogItem es una entrada de un catálogo cerrado.
type CatalogItem struct {
	ID   string `json:"id"`
	Name string `json:"nombre"`
}

// categoryCatalog son los 6 giros de A2 §4, en el orden en que los pinta la app.
var categoryCatalog = []CatalogItem{
	{ID: "restaurante", Name: "Restaurantes"},
	{ID: "cafeteria", Name: "Cafeterías"},
	{ID: "bar", Name: "Bares"},
	{ID: "entretenimiento", Name: "Entretenimiento"},
	{ID: "servicios", Name: "Servicios"},
	{ID: "comercio", Name: "Comercio"},
}

var priceLevelCatalog = []int{1, 2, 3, 4}

var dietCatalog = []string{"vegetariano", "vegano", "sin_gluten"}

var accessibilityCatalog = []string{"silla_ruedas", "apto_ninos", "pet_friendly"}

var allergenCatalog = []string{"gluten", "lacteos", "huevo", "soya", "cacahuate", "nueces", "pescado", "mariscos", "ajonjoli"}

// AllergenNone marca en `alergenos` que el negocio declaró no contener ninguno del
// catálogo. Va solo. Una lista vacía significa «sin dato», y NV-23 no recomienda ese
// negocio a quien declara alergias.
const AllergenNone = "ninguno"

// CategoryCatalog devuelve los giros en el orden de A2. Es una copia para que nadie mute
// el catálogo vivo.
func CategoryCatalog() []CatalogItem { return slices.Clone(categoryCatalog) }

// PriceLevelCatalog son los niveles de precio 1–4.
func PriceLevelCatalog() []int { return slices.Clone(priceLevelCatalog) }

// DietCatalog son las opciones de dieta que acepta el filtro duro de NV-23.
func DietCatalog() []string { return slices.Clone(dietCatalog) }

// AccessibilityCatalog son las banderas del objeto `accesibilidad`.
func AccessibilityCatalog() []string { return slices.Clone(accessibilityCatalog) }

// AllergenCatalog son los alérgenos que acepta NV-23.
func AllergenCatalog() []string { return slices.Clone(allergenCatalog) }

// ValidCategory dice si id es uno de los 6 giros.
func ValidCategory(id string) bool {
	return slices.ContainsFunc(categoryCatalog, func(item CatalogItem) bool { return item.ID == id })
}

// CategoryName devuelve el nombre de pantalla del giro; si no existe, devuelve el id.
func CategoryName(id string) string {
	for _, item := range categoryCatalog {
		if item.ID == id {
			return item.Name
		}
	}
	return id
}

// ValidDietOption dice si value está en el catálogo de dietas.
func ValidDietOption(value string) bool { return slices.Contains(dietCatalog, value) }

// ValidAccessibilityOption dice si value es una bandera de accesibilidad conocida.
func ValidAccessibilityOption(value string) bool { return slices.Contains(accessibilityCatalog, value) }

// ValidAllergen dice si value está en el catálogo de alérgenos o es AllergenNone.
func ValidAllergen(value string) bool {
	return value == AllergenNone || slices.Contains(allergenCatalog, value)
}

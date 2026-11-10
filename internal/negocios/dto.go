package negocios

// ItemCount es una categoría, etiqueta o servicio con su conteo de negocios visibles.
type ItemCount struct {
	ID    string `json:"id"`
	Name  string `json:"nombre"`
	Count int    `json:"conteo"`
}

// Filters son los filtros estáticos del catálogo, tomados del mismo catálogo cerrado.
type Filters struct {
	PriceLevels   []int    `json:"nivel_precio"`
	DietOptions   []string `json:"opciones_dieta"`
	Accessibility []string `json:"accesibilidad"`
}

// FacetsResponse es la respuesta de GET /api/v1/categorias.
type FacetsResponse struct {
	Categories []ItemCount `json:"categorias"`
	Tags       []ItemCount `json:"etiquetas"`
	Services   []ItemCount `json:"servicios"`
	Filters    Filters     `json:"filtros"`
}

// RowError es un error de una fila de la semilla. `Fila` es la fila de datos (la primera
// después del encabezado es 1).
type RowError struct {
	Row     int    `json:"fila"`
	Field   string `json:"campo,omitempty"`
	Message string `json:"mensaje"`
}

// ImportReport es la respuesta de la semilla, incluso cuando hay errores parciales.
type ImportReport struct {
	Created int        `json:"creados"`
	Updated int        `json:"actualizados"`
	Errors  []RowError `json:"errores"`
}

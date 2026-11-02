package pagination

// PageResult es la respuesta de toda lista paginada. NextCursor vacío = no hay más.
type PageResult[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"siguiente_cursor"`
	Limit      int    `json:"limite"`
}

// NewPageResult arma la página; una lista nil se serializa como [] y no como null.
func NewPageResult[T any](items []T, next string, limit int) PageResult[T] {
	if items == nil {
		items = []T{}
	}
	return PageResult[T]{Items: items, NextCursor: next, Limit: limit}
}

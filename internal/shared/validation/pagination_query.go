package validation

import "github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"

// PaginationQuery es el DTO de `?cursor=&limite=` que reutiliza toda lista paginada.
type PaginationQuery struct {
	Cursor string `query:"cursor" validate:"omitempty,cursor"`
	Limit  int    `query:"limite" validate:"min=1,max=50"`
}

// NewPaginationQuery trae `limite` por defecto; el parser de query solo pisa lo que llega,
// así un `limite` ausente queda en 20 y un `limite=0` explícito falla con 422.
func NewPaginationQuery() PaginationQuery {
	return PaginationQuery{Limit: pagination.DefaultLimit}
}

// Position devuelve el cursor ya validado; ok=false es la primera página.
func (q PaginationQuery) Position() (pagination.Cursor, bool, error) {
	return pagination.Decode(q.Cursor)
}

package validation

import (
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
)

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

// Position verifica la firma y el ámbito del cursor (nv-17); ok=false es la primera página.
// Un cursor alterado, de otra llave o de otro ámbito es 422 con el campo `cursor`.
func (q PaginationQuery) Position(signer *pagination.Signer, scope string) (pagination.Cursor, bool, error) {
	return Position(signer, scope, q.Cursor)
}

// Position es el único camino de un cursor del cliente a una posición; lo usan también
// las listas con su propio DTO de consulta.
func Position(signer *pagination.Signer, scope, raw string) (pagination.Cursor, bool, error) {
	c, ok, err := signer.Decode(scope, raw)
	if err != nil {
		return pagination.Cursor{}, false, apperr.Validation("cursor inválido",
			apperr.FieldError{Field: "cursor", Message: "cursor inválido"})
	}
	return c, ok, nil
}

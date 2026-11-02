// Package service fija la forma de los services de los módulos (`nv-04+`): cada uno
// recibe sus dependencias por constructor (`NewService(repo, ...)`), sin estado global,
// y devuelve errores de internal/shared/apperr para que el ErrorHandler los traduzca.
package service

import (
	"context"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

// Lister es el contrato de un service que lista recursos paginados por cursor.
type Lister[T any] interface {
	List(ctx context.Context, q validation.PaginationQuery) (pagination.PageResult[T], error)
}

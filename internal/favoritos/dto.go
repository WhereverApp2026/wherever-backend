package favoritos

import (
	"strings"
	"time"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/text"
)

// TargetParams son los parámetros de ruta de PUT y DELETE /favoritos/{tipo}/{objetivo_id}.
type TargetParams struct {
	Type     string `params:"tipo" json:"tipo" validate:"required,oneof=negocio plan"`
	TargetID string `params:"objetivo_id" json:"objetivo_id" validate:"required,len=24,hexadecimal"`
}

// ListQuery es GET /favoritos. `con_cupon` solo filtra negocios.
type ListQuery struct {
	Type       string `query:"tipo" validate:"required,oneof=negocio plan"`
	WithCoupon bool   `query:"con_cupon"`
	Cursor     string `query:"cursor" validate:"omitempty,max=300,cursor"`
	Limit      int    `query:"limite" validate:"min=1,max=50"`
}

// NewListQuery trae el límite por defecto (20).
func NewListQuery() ListQuery { return ListQuery{Limit: pagination.DefaultLimit} }

// IDsQuery es GET /favoritos/ids.
type IDsQuery struct {
	Type string `query:"tipo" validate:"required,oneof=negocio plan"`
}

// SearchQuery es GET /favoritos/buscar; sin `tipo` busca en los dos.
type SearchQuery struct {
	Q    string `query:"q" validate:"required,min=2,max=60"`
	Type string `query:"tipo" validate:"omitempty,oneof=negocio plan"`
}

// Kinds son los tipos donde busca la consulta.
func (q SearchQuery) Kinds() []string {
	if q.Type != "" {
		return []string{q.Type}
	}
	return []string{TypeBusiness, TypePlan}
}

// SummaryView es el resumen público del objetivo.
type SummaryView struct {
	Name     string `json:"nombre"`
	PhotoURL string `json:"foto_url,omitempty"`
	Category string `json:"categoria,omitempty"`
}

// View es un favorito. `cupon_hoy` y `abierto_ahora` solo salen en negocios.
type View struct {
	Type        string      `json:"tipo"`
	TargetID    string      `json:"objetivo_id"`
	Summary     SummaryView `json:"resumen"`
	CouponToday *bool       `json:"cupon_hoy,omitempty"`
	OpenNow     *bool       `json:"abierto_ahora,omitempty"`
	CreatedAt   time.Time   `json:"creado_en"`
}

// IDsView son los objetivos de un tipo, para pintar corazones.
type IDsView struct {
	Type string   `json:"tipo"`
	IDs  []string `json:"ids"`
}

// SearchView es el resultado de la búsqueda entre mis favoritos.
type SearchView struct {
	Items []View `json:"items"`
}

// MatchesPrefix dice si el nombre empieza con q, sin acentos, mayúsculas ni espacios de más
// (la normalización de `personas`).
func MatchesPrefix(name, q string) bool {
	prefix := text.Normalize(q)
	return prefix != "" && strings.HasPrefix(text.Normalize(name), prefix)
}

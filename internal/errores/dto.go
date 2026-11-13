package errores

import (
	"time"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
)

// defaultLimit de la lista del administrador; el máximo (100) está en la etiqueta de ListQuery.
const defaultLimit = 20

// ReportRequest es el cuerpo de POST /errores. Un campo de más es 400 (decodificación estricta).
type ReportRequest struct {
	Module  string `json:"modulo" validate:"required,modulo"`
	Message string `json:"mensaje" validate:"required,min=1,max=500"`
	Version string `json:"version" validate:"required,max=20,semver"`
	TraceID string `json:"traza_id" validate:"omitempty,traceid"`
}

// ListQuery es GET /admin/errores.
type ListQuery struct {
	Module string `query:"modulo" validate:"omitempty,modulo"`
	Since  string `query:"desde" validate:"omitempty,datetime=2006-01-02T15:04:05Z07:00"`
	Cursor string `query:"cursor" validate:"omitempty,max=300,cursor"`
	Limit  int    `query:"limite" validate:"min=1,max=100"`
}

// NewListQuery trae el límite por defecto (20).
func NewListQuery() ListQuery { return ListQuery{Limit: defaultLimit} }

// View es un grupo de errores tal como lo ve el administrador.
type View struct {
	ID        string    `json:"id"`
	Origin    string    `json:"origen"`
	Version   string    `json:"version"`
	Module    string    `json:"modulo"`
	Message   string    `json:"mensaje"`
	TraceID   string    `json:"traza_id,omitempty"`
	Count     int64     `json:"conteo"`
	CreatedAt time.Time `json:"creado_en"`
	LastAt    time.Time `json:"ultimo_en"`
}

func newView(r Report) View {
	return View{
		ID: r.ID.Hex(), Origin: r.Origin, Version: r.Version, Module: r.Module, Message: r.Message,
		TraceID: r.TraceID, Count: r.Count, CreatedAt: r.CreatedAt.UTC(), LastAt: r.LastAt.UTC(),
	}
}

func newPage(rows []Report, next string, limit int) pagination.PageResult[View] {
	views := make([]View, 0, len(rows))
	for _, r := range rows {
		views = append(views, newView(r))
	}
	return pagination.NewPageResult(views, next, limit)
}

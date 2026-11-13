package notificaciones

import (
	"time"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
)

// DeviceRequest es el cuerpo de PUT /dispositivos. El token va en el cuerpo, nunca en la
// ruta (la ruta queda en el log de acceso).
type DeviceRequest struct {
	Token    string `json:"token" validate:"required,min=20,max=4096"`
	Platform string `json:"plataforma" validate:"omitempty,oneof=android"`
}

// DeviceRemoveRequest es el cuerpo de DELETE /dispositivos.
type DeviceRemoveRequest struct {
	Token string `json:"token" validate:"required,min=20,max=4096"`
}

// MarkRequest es el cuerpo de PATCH /notificaciones/{id}.
type MarkRequest struct {
	Read *bool `json:"leida" validate:"required"`
}

// ListQuery es GET /notificaciones.
type ListQuery struct {
	Cursor string `query:"cursor" validate:"omitempty,max=300,cursor"`
	Limit  int    `query:"limite" validate:"min=1,max=50"`
}

// NewListQuery trae el límite por defecto (20).
func NewListQuery() ListQuery { return ListQuery{Limit: pagination.DefaultLimit} }

// Item es un aviso del centro.
type Item struct {
	ID        string            `json:"id"`
	Kind      string            `json:"tipo"`
	Data      map[string]string `json:"datos"`
	Read      bool              `json:"leida"`
	CreatedAt time.Time         `json:"creada_en"`
}

// UnreadResponse es la respuesta de GET /notificaciones/no-leidas.
type UnreadResponse struct {
	Total int64 `json:"total"`
}

func newItem(n Notification) Item {
	data := n.Data
	if data == nil {
		data = map[string]string{}
	}
	return Item{ID: n.ID.Hex(), Kind: n.Kind, Data: data, Read: n.ReadAt != nil, CreatedAt: n.CreatedAt}
}

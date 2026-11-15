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

// DevicePatch es el cuerpo de PUT /dispositivos/{id} (nv-71): al menos un campo; los campos
// de más son 400 (decodificación estricta).
type DevicePatch struct {
	AppVersion *string   `json:"app_version" validate:"omitempty,max=20,semver"`
	Channels   *[]string `json:"canales_activos" validate:"omitempty,max=5,unique,dive,oneof=reservas cupones grupos resenas promociones"`
}

// DeviceView es un teléfono en la respuesta; nunca lleva el token ni su hash.
type DeviceView struct {
	ID         string    `json:"id"`
	Platform   string    `json:"plataforma"`
	AppVersion string    `json:"app_version,omitempty"`
	Channels   []string  `json:"canales_activos"`
	LastUsed   time.Time `json:"ultimo_uso"`
}

// AllChannels son los canales cuando el teléfono no mandó lista.
func AllChannels() []string {
	return []string{ChannelBookings, ChannelCoupons, ChannelGroups, ChannelReviews, ChannelPromotions}
}

func newDeviceView(d Device) DeviceView {
	channels := AllChannels()
	if d.Channels != nil {
		channels = *d.Channels
	}
	return DeviceView{ID: d.ID.Hex(), Platform: d.Platform, AppVersion: d.AppVersion, Channels: channels, LastUsed: d.LastUsed}
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

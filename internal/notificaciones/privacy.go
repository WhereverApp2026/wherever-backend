package notificaciones

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Privacy es la parte de notificaciones en ARCO (nv-05): cumple privacidad.Purger y
// privacidad.Exporter sobre `notificaciones` y `dispositivos`.
type Privacy struct {
	repo *Repository
}

// NewPrivacy recibe el repositorio por constructor.
func NewPrivacy(repo *Repository) *Privacy { return &Privacy{repo: repo} }

// Name identifica al módulo en el registro y en el JSON exportado.
func (p *Privacy) Name() string { return Collection }

// PurgeHistory borra los avisos (son historial); los teléfonos se quedan para seguir
// recibiendo push.
func (p *Privacy) PurgeHistory(ctx context.Context, userID bson.ObjectID) error {
	return p.repo.DeleteByUser(ctx, userID)
}

// PurgeAccount borra los avisos y los teléfonos.
func (p *Privacy) PurgeAccount(ctx context.Context, userID bson.ObjectID) error {
	if err := p.repo.DeleteByUser(ctx, userID); err != nil {
		return err
	}
	return p.repo.DeleteDevicesByUser(ctx, userID)
}

type exportedDevice struct {
	Platform  string    `json:"plataforma"`
	LastUsed  time.Time `json:"ultimo_uso"`
	CreatedAt time.Time `json:"creado_en"`
}

type exported struct {
	Notifications []Item           `json:"avisos"`
	Devices       []exportedDevice `json:"dispositivos"`
}

// Export devuelve los avisos y los teléfonos (plataforma y fechas; nunca el token).
func (p *Privacy) Export(ctx context.Context, userID bson.ObjectID) (any, error) {
	notes, err := p.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	devices, err := p.repo.Devices(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := exported{Notifications: make([]Item, 0, len(notes)), Devices: make([]exportedDevice, 0, len(devices))}
	for _, n := range notes {
		out.Notifications = append(out.Notifications, newItem(n))
	}
	for _, d := range devices {
		out.Devices = append(out.Devices, exportedDevice{Platform: d.Platform, LastUsed: d.LastUsed, CreatedAt: d.CreatedAt})
	}
	return out, nil
}

package auth

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Privacy es la parte de auth en los derechos ARCO (nv-05): cumple privacidad.Purger y
// privacidad.Exporter sobre `sesiones` y `tokens_un_uso`.
type Privacy struct {
	repo *Repository
}

// NewPrivacy recibe el repositorio de auth.
func NewPrivacy(repo *Repository) *Privacy { return &Privacy{repo: repo} }

// Name identifica al módulo en el registro y en el JSON exportado.
func (p *Privacy) Name() string { return SessionsCollection }

// PurgeHistory no borra nada: las sesiones no son historial de uso.
func (p *Privacy) PurgeHistory(context.Context, bson.ObjectID) error { return nil }

// PurgeAccount borra sesiones y enlaces del usuario.
func (p *Privacy) PurgeAccount(ctx context.Context, userID bson.ObjectID) error {
	return p.repo.DeleteUserData(ctx, userID)
}

type exportedSession struct {
	Device    Device     `json:"dispositivo"`
	Method    string     `json:"metodo"`
	IP        string     `json:"ip"`
	CreatedAt time.Time  `json:"creado_en"`
	LastUsed  time.Time  `json:"ultimo_uso"`
	RevokedAt *time.Time `json:"revocada_en,omitempty"`
}

// Export devuelve las sesiones del usuario sin hashes ni jti.
func (p *Privacy) Export(ctx context.Context, userID bson.ObjectID) (any, error) {
	sessions, err := p.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]exportedSession, 0, len(sessions))
	for _, s := range sessions {
		out = append(out, exportedSession{Device: s.Device, Method: s.Method, IP: s.IP, CreatedAt: s.CreatedAt, LastUsed: s.LastUsed, RevokedAt: s.RevokedAt})
	}
	return out, nil
}

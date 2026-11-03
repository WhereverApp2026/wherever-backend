package account

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// Privacy es la parte de `usuarios` en los derechos ARCO: cumple privacidad.Purger y
// privacidad.Exporter. Se registra al final para que el documento del usuario sea lo último
// que se borra (si otro módulo falla, la cuenta sigue pendiente y se reintenta).
type Privacy struct {
	users *usuarios.Repository
}

// NewPrivacy recibe el repositorio de usuarios.
func NewPrivacy(users *usuarios.Repository) *Privacy { return &Privacy{users: users} }

// Name identifica al módulo en el registro y en el JSON exportado.
func (p *Privacy) Name() string { return usuarios.Collection }

// PurgeHistory no borra nada aquí: el historial (planes, check-ins, señales) vive en sus
// módulos, que registrarán su propio Purger.
func (p *Privacy) PurgeHistory(context.Context, bson.ObjectID) error { return nil }

// PurgeAccount borra el documento del usuario.
func (p *Privacy) PurgeAccount(ctx context.Context, userID bson.ObjectID) error {
	return p.users.Delete(ctx, userID)
}

// Export devuelve el perfil sin hashes ni google_id; una cuenta ya borrada exporta nada.
func (p *Privacy) Export(ctx context.Context, userID bson.ObjectID) (any, error) {
	user, err := p.users.FindByID(ctx, userID)
	if errors.Is(err, usuarios.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return profileResponse(user), nil
}

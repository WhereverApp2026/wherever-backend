package panel

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios/menu"
)

// Privacy exporta los negocios del comerciante (ARCO): perfil, horarios, menú y
// `reservas_config`. Cumple privacidad.Exporter. El expediente lo exporta expediente.Privacy y
// la baja al borrar la cuenta la hace alta.Privacy.
type Privacy struct {
	repo *negocios.Repository
	menu *menu.Repository
}

// NewPrivacy recibe los repositorios por constructor.
func NewPrivacy(repo *negocios.Repository, m *menu.Repository) *Privacy {
	return &Privacy{repo: repo, menu: m}
}

// Name es la llave de la exportación.
func (p *Privacy) Name() string { return negocios.Collection }

// Export devuelve los negocios no dados de baja de la persona.
func (p *Privacy) Export(ctx context.Context, userID bson.ObjectID) (any, error) {
	rows, err := p.repo.ListByOwner(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	var errs []error
	for _, b := range rows {
		items, err := p.menu.DeNegocio(ctx, b.ID)
		errs = append(errs, err)
		platillos := make([]map[string]any, 0, len(items))
		for _, it := range items {
			platillos = append(platillos, map[string]any{"seccion": it.Seccion, "nombre": it.Nombre, "descripcion": it.Descripcion,
				"precio": it.Precio, "etiquetas": nonNil(it.Etiquetas), "disponible": it.Disponible})
		}
		out = append(out, map[string]any{
			"nombre": b.Name, "categoria": b.Category, "descripcion": b.Description, "direccion": b.Address, "telefono": b.Phone,
			"horarios": nonNil(b.Schedules), "nivel_precio": b.PriceLevel, "precio_promedio": b.AveragePrice,
			"servicios": nonNil(b.Services), "opciones_dieta": nonNil(b.DietOptions), "alergenos": nonNil(b.Allergens),
			"estado": b.State, "reservas_config": b.ReservasConfig, "menu": platillos, "creado_en": b.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	return out, errors.Join(errs...)
}

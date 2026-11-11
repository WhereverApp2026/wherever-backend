package recomendador

import (
	"slices"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
)

// Filtro son las reglas duras de nv-23. Condiciones las lleva a MongoDB (van en la
// consulta, antes de puntuar); Cumple es la misma regla en Go y se vuelve a aplicar a cada
// candidato para que una alergia nunca se cuele. Aprobado y no eliminado lo pone
// negocios.VisibleFilter.
type Filtro struct{ c Contexto }

// NewFiltro arma el filtro de un contexto.
func NewFiltro(c Contexto) Filtro { return Filtro{c: c} }

// Condiciones son los filtros duros como un `$and` de MongoDB.
func (f Filtro) Condiciones() bson.A {
	out := bson.A{negocios.OpenAtCondition(f.c.Momento)}
	if cond := f.alergias(); cond != nil {
		out = append(out, cond)
	}
	if len(f.c.Dietas) > 0 {
		out = append(out, bson.D{{Key: "opciones_dieta", Value: bson.D{{Key: "$all", Value: f.c.Dietas}}}})
	}
	for _, field := range f.banderas() {
		out = append(out, bson.D{{Key: "accesibilidad." + field, Value: true}})
	}
	if len(f.c.Excluidos) > 0 {
		out = append(out, bson.D{{Key: "_id", Value: bson.D{{Key: "$nin", Value: f.c.Excluidos}}}})
	}
	return out
}

// alergias exige dato declarado: una lista vacía es «sin dato» y no pasa. Con una alergia
// fuera del catálogo solo pasa «ninguno».
func (f Filtro) alergias() bson.D {
	switch {
	case f.c.AlergiaSinCatalogo:
		return bson.D{{Key: "alergenos", Value: bson.A{negocios.AllergenNone}}}
	case len(f.c.Alergenos) > 0:
		return bson.D{
			{Key: "alergenos.0", Value: bson.D{{Key: "$exists", Value: true}}},
			{Key: "alergenos", Value: bson.D{{Key: "$nin", Value: f.c.Alergenos}}},
		}
	default:
		return nil
	}
}

// banderas son los campos de `accesibilidad` que el contexto exige.
func (f Filtro) banderas() []string {
	var out []string
	if f.c.SillaRuedas {
		out = append(out, "silla_ruedas")
	}
	if f.c.ConNinos {
		out = append(out, "apto_ninos")
	}
	if f.c.ConMascota {
		out = append(out, "pet_friendly")
	}
	return out
}

// Cumple aplica en Go las mismas reglas de Condiciones más la visibilidad.
func (f Filtro) Cumple(b negocios.Business) bool {
	return b.State == negocios.StatusApproved && b.DeletedAt == nil &&
		negocios.OpenAt(b.ScheduleMin, f.c.Momento) &&
		f.cumpleAlergias(b.Allergens) &&
		containsAll(b.DietOptions, f.c.Dietas) &&
		f.cumpleAccesibilidad(b.Accessibility) &&
		!slices.Contains(f.c.Excluidos, b.ID)
}

func (f Filtro) cumpleAlergias(declared []string) bool {
	switch {
	case f.c.AlergiaSinCatalogo:
		return slices.Equal(declared, []string{negocios.AllergenNone})
	case len(f.c.Alergenos) > 0:
		return len(declared) > 0 && !slices.ContainsFunc(declared, func(a string) bool {
			return slices.Contains(f.c.Alergenos, a)
		})
	default:
		return true
	}
}

func (f Filtro) cumpleAccesibilidad(a negocios.Accesibilidad) bool {
	return (!f.c.SillaRuedas || a.SillaRuedas) && (!f.c.ConNinos || a.AptoNinos) && (!f.c.ConMascota || a.PetFriendly)
}

func containsAll(have, want []string) bool {
	for _, w := range want {
		if !slices.Contains(have, w) {
			return false
		}
	}
	return true
}

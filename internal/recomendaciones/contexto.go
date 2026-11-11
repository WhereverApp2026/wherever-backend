package recomendaciones

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/recomendador"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// companiaTerminos son los gustos que suma cada compañía: el nombre y su etiqueta habitual.
// Solo suben el puntaje de gustos; no son filtros duros.
var companiaTerminos = map[string][]string{
	"solo":    {"solo", "individual"},
	"pareja":  {"pareja", "romantico"},
	"amigos":  {"amigos", "grupos"},
	"familia": {"familia", "familiar"},
}

// Momento es la hora pedida (`HH:MM`, hoy en la zona de los horarios) o ahora.
func Momento(now time.Time, hora string) time.Time {
	if hora == "" {
		return now
	}
	t, err := time.Parse("15:04", hora)
	if err != nil {
		return now
	}
	local := now.In(negocios.MexicoCity())
	return time.Date(local.Year(), local.Month(), local.Day(), t.Hour(), t.Minute(), 0, 0, local.Location())
}

// Contexto arma lo que el motor sabe del usuario según su privacidad (requisito 3):
//   - Alergias: solo con el consentimiento de datos sensibles (nv-05) y solo como filtro
//     duro; nunca suman puntos ni van en el porqué.
//   - Dietas del catálogo: siempre filtro duro, las indicó el usuario.
//   - Gustos y presupuesto guardados: solo con `aprendizaje_personalizado`.
//   - Compañía y hora: vienen en la petición, siempre se usan.
func Contexto(u usuarios.User, q Query, now time.Time, excluded []bson.ObjectID) recomendador.Contexto {
	c := recomendador.Contexto{
		Lat: *q.Lat, Lng: *q.Lng, RadioM: float64(q.RadiusM),
		Momento: Momento(now, q.Hour), Excluidos: excluded,
	}
	if t := u.Tastes; t != nil {
		if u.Privacy.SensitiveData {
			c.Alergenos, c.AlergiaSinCatalogo = recomendador.ParseAlergias(t.Allergies)
		}
		c.Dietas = recomendador.DietasDe(t.Restrictions)
		if u.Privacy.Learning {
			c.Gustos = append(c.Gustos, t.Categories...)
			c.PresupuestoMin, c.PresupuestoMax = t.BudgetMin, t.BudgetMax
		}
	}
	c.Gustos = append(c.Gustos, companiaTerminos[q.Company]...)
	return c
}

package recomendador

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
)

// lunesMediodia es lunes 2026-11-09 a las 12:00 en Ciudad de México.
var lunesMediodia = time.Date(2026, 11, 9, 18, 0, 0, 0, time.UTC)

func todoElDia() []negocios.VentanaMin {
	out := make([]negocios.VentanaMin, 0, 7)
	for d := 0; d < 7; d++ {
		out = append(out, negocios.VentanaMin{Dia: d, Desde: 9 * 60, Hasta: 22 * 60})
	}
	return out
}

func negocioBase() negocios.Business {
	return negocios.Business{
		ID: bson.NewObjectID(), State: negocios.StatusApproved, Category: "restaurante",
		ScheduleMin: todoElDia(), Allergens: []string{negocios.AllergenNone},
	}
}

func ctxBase() Contexto { return Contexto{Momento: lunesMediodia} }

func TestFiltro_NoAprobadoNiEliminado(t *testing.T) {
	f := NewFiltro(ctxBase())
	b := negocioBase()
	assert.True(t, f.Cumple(b))
	b.State = negocios.StatusPaused
	assert.False(t, f.Cumple(b))
	b = negocioBase()
	now := time.Now()
	b.DeletedAt = &now
	assert.False(t, f.Cumple(b))
}

func TestFiltro_Cerrado(t *testing.T) {
	b := negocioBase()
	b.ScheduleMin = []negocios.VentanaMin{{Dia: 1, Desde: 18 * 60, Hasta: 23 * 60}}
	assert.False(t, NewFiltro(ctxBase()).Cumple(b))
}

func TestFiltro_AlergiaDeclaradaNoSeCuela(t *testing.T) {
	c := ctxBase()
	c.Alergenos = []string{"cacahuate"}
	f := NewFiltro(c)
	b := negocioBase()
	b.Allergens = []string{"gluten", "cacahuate"}
	assert.False(t, f.Cumple(b))
	b.Allergens = []string{"gluten"}
	assert.True(t, f.Cumple(b))
}

func TestFiltro_AlergiaSinDatoNoPasa(t *testing.T) {
	c := ctxBase()
	c.Alergenos = []string{"mariscos"}
	b := negocioBase()
	b.Allergens = nil
	assert.False(t, NewFiltro(c).Cumple(b), "lista vacía = sin dato")
	assert.True(t, NewFiltro(ctxBase()).Cumple(b), "sin alergias, el dato no importa")
}

func TestFiltro_AlergiaFueraDeCatalogoSoloNinguno(t *testing.T) {
	c := ctxBase()
	c.AlergiaSinCatalogo = true
	f := NewFiltro(c)
	b := negocioBase()
	assert.True(t, f.Cumple(b))
	b.Allergens = []string{"gluten"}
	assert.False(t, f.Cumple(b))
}

func TestFiltro_Dieta(t *testing.T) {
	c := ctxBase()
	c.Dietas = []string{"vegano", "sin_gluten"}
	b := negocioBase()
	b.DietOptions = []string{"vegano"}
	assert.False(t, NewFiltro(c).Cumple(b))
	b.DietOptions = []string{"vegano", "sin_gluten", "vegetariano"}
	assert.True(t, NewFiltro(c).Cumple(b))
}

func TestFiltro_AccesibilidadMascotasNinos(t *testing.T) {
	c := ctxBase()
	c.SillaRuedas, c.ConNinos, c.ConMascota = true, true, true
	b := negocioBase()
	b.Accessibility = negocios.Accesibilidad{SillaRuedas: true, AptoNinos: true}
	assert.False(t, NewFiltro(c).Cumple(b))
	b.Accessibility.PetFriendly = true
	assert.True(t, NewFiltro(c).Cumple(b))
}

func TestFiltro_NoMeInteresa(t *testing.T) {
	b := negocioBase()
	c := ctxBase()
	c.Excluidos = []bson.ObjectID{b.ID}
	assert.False(t, NewFiltro(c).Cumple(b))
}

func TestFiltro_CondicionesCombinadas(t *testing.T) {
	c := ctxBase()
	c.Alergenos = []string{"huevo"}
	c.Dietas = []string{"vegano"}
	c.ConMascota = true
	c.Excluidos = []bson.ObjectID{bson.NewObjectID()}
	conds := NewFiltro(c).Condiciones()
	assert.Len(t, conds, 5)
	assert.Equal(t, negocios.OpenAtCondition(lunesMediodia), conds[0])
	assert.Equal(t, bson.D{
		{Key: "alergenos.0", Value: bson.D{{Key: "$exists", Value: true}}},
		{Key: "alergenos", Value: bson.D{{Key: "$nin", Value: []string{"huevo"}}}},
	}, conds[1])
	assert.Equal(t, bson.D{{Key: "accesibilidad.pet_friendly", Value: true}}, conds[3])

	c = ctxBase()
	c.AlergiaSinCatalogo = true
	assert.Equal(t, bson.D{{Key: "alergenos", Value: bson.A{negocios.AllergenNone}}}, NewFiltro(c).Condiciones()[1])
	assert.Len(t, NewFiltro(ctxBase()).Condiciones(), 1, "sin restricciones solo queda abierto ahora")
}

func TestParseAlergias(t *testing.T) {
	ids, unknown := ParseAlergias("Cacahuate, camarones y LECHE; nuez")
	assert.Equal(t, []string{"cacahuate", "mariscos", "lacteos", "nueces"}, ids)
	assert.False(t, unknown)

	ids, unknown = ParseAlergias("ninguna")
	assert.Empty(t, ids)
	assert.False(t, unknown)

	ids, unknown = ParseAlergias("gluten, kiwi")
	assert.Equal(t, []string{"gluten"}, ids)
	assert.True(t, unknown, "lo que no reconoce no se descarta")
}

func TestDietasDe(t *testing.T) {
	assert.Equal(t, []string{"vegano", "sin_gluten"}, DietasDe([]string{"Vegano", "sin gluten", "sin picante", "vegano"}))
	assert.Empty(t, DietasDe(nil))
}

package asistente

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/recomendador"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

func todoElDia() []negocios.VentanaMin {
	out := make([]negocios.VentanaMin, 0, 7)
	for d := 0; d < 7; d++ {
		out = append(out, negocios.VentanaMin{Dia: d, Desde: 0, Hasta: 24 * 60})
	}
	return out
}

// rec es un candidato a dLat grados al norte del Zócalo (0.001° ≈ 111 m).
func rec(nombre, giro string, precio int, dLat float64, horario []negocios.VentanaMin) recomendador.Recomendacion {
	if horario == nil {
		horario = todoElDia()
	}
	return recomendador.Recomendacion{
		Negocio: negocios.Business{
			ID: bson.NewObjectID(), Name: nombre, Category: giro, AveragePrice: precio,
			Location:    negocios.Location{Type: "Point", Coordinates: []float64{-99.1332, 19.4326 + dLat}},
			ScheduleMin: horario,
		},
		PorQue: "Cerca de ti",
	}
}

func pedido(presupuesto int) Pedido {
	return Pedido{Inicio: sabadoTarde, Personas: 2, Movilidad: planes.MobilityWalk, PresupuestoMax: presupuesto}
}

func nombres(paradas []recomendador.Recomendacion) []string {
	out := make([]string, 0, len(paradas))
	for _, r := range paradas {
		out = append(out, r.Negocio.Name)
	}
	return out
}

func TestArmar_TresParadasDeGirosDistintosEnOrden(t *testing.T) {
	recs := []recomendador.Recomendacion{
		rec("cafe-a", "cafeteria", 100, 0.001, nil),
		rec("cafe-b", "cafeteria", 100, 0.002, nil),
		rec("rest-a", "restaurante", 200, 0.003, nil),
		rec("bar-a", "bar", 150, 0.004, nil),
	}
	plan, usados, err := Armar(recs, pedido(0), planes.DefaultVelocidades())
	require.NoError(t, err)
	assert.Equal(t, []string{"cafe-a", "rest-a", "bar-a"}, nombres(usados))
	require.Len(t, plan.Paradas, 3)
	assert.Equal(t, sabadoTarde, plan.Paradas[0].Llega)
	for i := 1; i < 3; i++ {
		prev, cur := plan.Paradas[i-1], plan.Paradas[i]
		fin := prev.Llega.Add(time.Duration(prev.Minutos+plan.Traslados[i-1].Minutos) * time.Minute)
		assert.False(t, cur.Llega.Before(fin), "la parada %d empieza después del traslado", i)
		assert.Zero(t, cur.Llega.Minute()%5, "llegada redondeada a 5 min")
	}
	assert.Equal(t, 60, plan.Paradas[0].Minutos)
	assert.Equal(t, 90, plan.Paradas[1].Minutos)
	assert.Len(t, plan.Traslados, 2)
	assert.Equal(t, 450, plan.CostoPorPersona)
	assert.Equal(t, 900, plan.CostoTotal)
	assert.Equal(t, planes.OriginAI, plan.Origen)
	assert.Equal(t, "Plan de la tarde", plan.Nombre)
}

func TestArmar_RespetaPresupuestoYRepiteGiroSiFalta(t *testing.T) {
	recs := []recomendador.Recomendacion{
		rec("rest-caro", "restaurante", 500, 0.001, nil),
		rec("cafe-a", "cafeteria", 120, 0.001, nil),
		rec("cafe-b", "cafeteria", 120, 0.002, nil),
		rec("cafe-c", "cafeteria", 120, 0.003, nil),
	}
	plan, usados, err := Armar(recs, pedido(250), planes.DefaultVelocidades())
	require.NoError(t, err)
	assert.Equal(t, []string{"cafe-a", "cafe-b"}, nombres(usados))
	assert.Equal(t, 240, plan.CostoPorPersona)
	assert.Equal(t, 500, plan.Presupuesto, "presupuesto por persona × personas")
}

func TestArmar_OmiteLoQueCierraAntesDeTerminar(t *testing.T) {
	cierra := []negocios.VentanaMin{{Dia: int(time.Saturday), Desde: 9 * 60, Hasta: 15 * 60}}
	recs := []recomendador.Recomendacion{
		rec("rest-cierra", "restaurante", 100, 0.001, cierra),
		rec("bar-a", "bar", 100, 0.002, nil),
	}
	_, usados, err := Armar(recs, pedido(0), planes.DefaultVelocidades())
	require.NoError(t, err)
	assert.Equal(t, []string{"bar-a"}, nombres(usados))
}

func TestArmar_SinCandidatosOMovilidadDesconocida(t *testing.T) {
	_, _, err := Armar(nil, pedido(0), planes.DefaultVelocidades())
	require.ErrorIs(t, err, ErrSinCandidatos)
	sinUbicacion := rec("x", "bar", 100, 0, nil)
	sinUbicacion.Negocio.Location.Coordinates = nil
	_, _, err = Armar([]recomendador.Recomendacion{sinUbicacion}, pedido(0), planes.DefaultVelocidades())
	require.ErrorIs(t, err, ErrSinCandidatos)
	p := pedido(0)
	p.Movilidad = "patineta"
	_, _, err = Armar([]recomendador.Recomendacion{rec("a", "bar", 1, 0, nil)}, p, planes.DefaultVelocidades())
	require.ErrorIs(t, err, planes.ErrMobility)
}

func TestNombrePlan_PorFranja(t *testing.T) {
	dia := func(h int) time.Time { return time.Date(2026, 11, 14, h, 0, 0, 0, zona) }
	assert.Equal(t, "Plan de la mañana", nombrePlan(dia(9)))
	assert.Equal(t, "Plan de la tarde", nombrePlan(dia(13)))
	assert.Equal(t, "Plan de la noche", nombrePlan(dia(21)))
	assert.Equal(t, "Plan de la noche", nombrePlan(dia(2)))
}

func perfil() *usuarios.Tastes {
	return &usuarios.Tastes{
		Categories: []string{"Cafeterías"}, BudgetMin: 100, BudgetMax: 400,
		Restrictions: []string{"Vegano"}, Allergies: "cacahuate, kiwi",
	}
}

// Alergias y dieta del perfil entran siempre como filtros duros, aunque la frase no las diga.
func TestContexto_AlergiasYDietaSiempreComoFiltroDuro(t *testing.T) {
	in := Interpretacion{Compania: "amigos", PresupuestoMax: 300, PalabrasClave: []string{"tacos"}, ConNinos: true}
	c := contexto(19.43, -99.13, sabadoTarde, in, perfil())
	assert.Equal(t, []string{"cacahuate"}, c.Alergenos)
	assert.True(t, c.AlergiaSinCatalogo, "kiwi no está en el catálogo: pasa solo «ninguno»")
	assert.Equal(t, []string{"vegano"}, c.Dietas)
	assert.Equal(t, 300, c.PresupuestoMax, "manda el monto de la frase")
	assert.Zero(t, c.PresupuestoMin)
	assert.Equal(t, []string{"Cafeterías", "tacos"}, c.Gustos)
	assert.True(t, c.ConNinos)
	assert.Equal(t, sabadoTarde, c.Momento)

	sinPerfil := contexto(19.43, -99.13, sabadoTarde, in, nil)
	assert.Empty(t, sinPerfil.Alergenos)
	assert.Equal(t, []string{"tacos"}, sinPerfil.Gustos)
}

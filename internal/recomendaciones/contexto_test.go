package recomendaciones

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/recomendador"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// lunes 2026-11-09 a las 12:00 en Ciudad de México.
var mediodia = time.Date(2026, 11, 9, 18, 0, 0, 0, time.UTC)

func ptr(v float64) *float64 { return &v }

func query() Query { return Query{Lat: ptr(19.43), Lng: ptr(-99.13), Limit: DefaultLimit} }

func userWith(learning, sensitive bool) usuarios.User {
	return usuarios.User{
		Tastes: &usuarios.Tastes{
			Categories: []string{"cafeteria"}, BudgetMin: 100, BudgetMax: 200,
			Restrictions: []string{"Vegetariano", "sin picante"}, Allergies: "cacahuate",
		},
		Privacy: usuarios.Privacy{Learning: learning, SensitiveData: sensitive},
	}
}

func TestContexto_ConAprendizajeYConsentimientoUsaTodo(t *testing.T) {
	excluded := []bson.ObjectID{bson.NewObjectID()}
	c := Contexto(userWith(true, true), query(), mediodia, excluded)
	assert.Equal(t, []string{"cafeteria"}, c.Gustos)
	assert.Equal(t, 100, c.PresupuestoMin)
	assert.Equal(t, 200, c.PresupuestoMax)
	assert.Equal(t, []string{"cacahuate"}, c.Alergenos)
	assert.False(t, c.AlergiaSinCatalogo)
	assert.Equal(t, []string{"vegetariano"}, c.Dietas)
	assert.Equal(t, excluded, c.Excluidos)
	assert.Equal(t, 19.43, c.Lat)
	assert.Equal(t, mediodia, c.Momento)
}

func TestContexto_AprendizajeApagadoSoloFiltrosDuros(t *testing.T) {
	c := Contexto(userWith(false, true), query(), mediodia, nil)
	assert.Empty(t, c.Gustos, "sin aprendizaje no se usan los gustos guardados")
	assert.Zero(t, c.PresupuestoMax)
	assert.Equal(t, []string{"cacahuate"}, c.Alergenos, "la alergia sigue como filtro duro")
	assert.Equal(t, []string{"vegetariano"}, c.Dietas)
}

func TestContexto_SinConsentimientoNoUsaAlergias(t *testing.T) {
	c := Contexto(userWith(true, false), query(), mediodia, nil)
	assert.Empty(t, c.Alergenos)
	assert.False(t, c.AlergiaSinCatalogo)
	assert.Equal(t, []string{"cafeteria"}, c.Gustos)
}

func TestContexto_AlergiaFueraDelCatalogo(t *testing.T) {
	u := userWith(true, true)
	u.Tastes.Allergies = "kiwi"
	c := Contexto(u, query(), mediodia, nil)
	assert.True(t, c.AlergiaSinCatalogo)
}

func TestContexto_UsuarioSinGustosYCompania(t *testing.T) {
	q := query()
	q.Company = "familia"
	c := Contexto(usuarios.User{Privacy: usuarios.Privacy{Learning: true}}, q, mediodia, nil)
	assert.Equal(t, []string{"familia", "familiar"}, c.Gustos)
	assert.Empty(t, c.Alergenos)
	assert.Empty(t, c.Dietas)
}

func TestMomento(t *testing.T) {
	assert.Equal(t, mediodia, Momento(mediodia, ""))
	assert.Equal(t, mediodia, Momento(mediodia, "25:99"))
	got := Momento(mediodia, "20:30").In(negocios.MexicoCity())
	assert.Equal(t, 20, got.Hour())
	assert.Equal(t, 30, got.Minute())
	assert.Equal(t, 9, got.Day())
}

func TestQuery_Validation(t *testing.T) {
	v, err := validation.New()
	require.NoError(t, err)
	require.NoError(t, v.Struct(query()))

	cases := map[string]func(*Query){
		"sin lat":         func(q *Query) { q.Lat = nil },
		"sin lng":         func(q *Query) { q.Lng = nil },
		"lat fuera":       func(q *Query) { q.Lat = ptr(91) },
		"radio grande":    func(q *Query) { q.RadiusM = 20001 },
		"compania":        func(q *Query) { q.Company = "jefe" },
		"hora":            func(q *Query) { q.Hour = "8pm" },
		"limite cero":     func(q *Query) { q.Limit = 0 },
		"limite excedido": func(q *Query) { q.Limit = MaxLimit + 1 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			q := query()
			mutate(&q)
			assert.Error(t, v.Struct(q))
		})
	}
}

func TestToCard_TraePorQueYRazonesNoNulas(t *testing.T) {
	b := negocios.Business{
		ID: bson.NewObjectID(), Slug: "cafe", Name: "Café",
		Location: negocios.Location{Type: "Point", Coordinates: []float64{-99.13, 19.43}},
	}
	card := toCard(recomendador.Recomendacion{Negocio: b, DistanciaM: 120, Puntaje: 71.5, PorQue: "Te lo recomendamos."}, mediodia)
	assert.Equal(t, b.ID.Hex(), card.ID)
	assert.Equal(t, 71.5, card.Score)
	assert.Equal(t, "Te lo recomendamos.", card.Why)
	assert.NotNil(t, card.Reasons)
	require.NotNil(t, card.DistanceM)
	assert.Equal(t, 120.0, *card.DistanceM)
}

func TestNewService_RechazaPesosInvalidos(t *testing.T) {
	_, err := NewService(nil, nil, nil, recomendador.Pesos{"gustos": 2}, nil)
	assert.ErrorIs(t, err, recomendador.ErrPesos)
	pesos, err := recomendador.PesosPorDefecto()
	require.NoError(t, err)
	_, err = NewService(nil, nil, nil, pesos, nil)
	assert.NoError(t, err)
}

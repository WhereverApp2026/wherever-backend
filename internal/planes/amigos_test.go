package planes

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

func TestSharedWithFriends_AmigosYPublico(t *testing.T) {
	assert.True(t, Plan{Visibilidad: VisibilityFriends}.SharedWithFriends())
	assert.True(t, Plan{Visibilidad: VisibilityPublic}.SharedWithFriends())
	assert.False(t, Plan{Visibilidad: VisibilityPrivate}.SharedWithFriends())
	assert.False(t, Plan{}.SharedWithFriends(), "los planes de antes son privados")
	assert.False(t, Plan{Visibilidad: VisibilityFriends}.Public(), "amigos no es público: no sale en populares ni en el perfil")
}

func TestVisibilidad_AceptaAmigos(t *testing.T) {
	v, err := validation.New()
	require.NoError(t, err)
	amigos, otra := "amigos", "todos"
	assert.NoError(t, v.Struct(PatchRequest{Visibilidad: &amigos}))
	assert.Error(t, v.Struct(PatchRequest{Visibilidad: &otra}))
	req := CreateRequest{Nombre: "Plan", Fecha: "2026-12-05", Personas: 2, Movilidad: MobilityWalk, Visibilidad: "amigos",
		Paradas: []StopRequest{{NegocioID: bson.NewObjectID().Hex(), Llega: "2026-12-05T10:00:00-06:00", Minutos: 30}}}
	require.NoError(t, v.Struct(req))
	p, err := req.toPlan(bson.NewObjectID())
	require.NoError(t, err)
	assert.Equal(t, VisibilityFriends, p.Visibilidad)
	req.Visibilidad = "todos"
	assert.Error(t, v.Struct(req))
}

func TestNewFriendsWindow_HoyA30DiasYCreados7(t *testing.T) {
	// 2026-11-22 03:00 UTC = 21 de noviembre 21:00 en México.
	now := time.Date(2026, 11, 22, 3, 0, 0, 0, time.UTC)
	w := NewFriendsWindow(now)
	assert.Equal(t, time.Date(2026, 11, 21, 6, 0, 0, 0, time.UTC), w.From.UTC(), "hoy es el día de México")
	assert.Equal(t, w.From.AddDate(0, 0, 31), w.To)
	assert.Equal(t, now.Add(-7*24*time.Hour), w.CreatedSince)
}

func TestNewFriendPlan_ApuntadosYPrivacidad(t *testing.T) {
	me, jorge, oculta, ajena := bson.NewObjectID(), bson.NewObjectID(), bson.NewObjectID(), bson.NewObjectID()
	extra1, extra2, extra3 := bson.NewObjectID(), bson.NewObjectID(), bson.NewObjectID()
	friends := map[bson.ObjectID]usuarios.User{
		jorge: {ID: jorge, Name: "Jorge"}, oculta: {ID: oculta, Privacy: usuarios.Privacy{Social: usuarios.Social{Activity: usuarios.ActivityNobody}}},
		extra1: {ID: extra1}, extra2: {ID: extra2}, extra3: {ID: extra3},
	}
	lugar, cerrado := bson.NewObjectID(), bson.NewObjectID()
	p := Plan{ID: bson.NewObjectID(), Nombre: "Sábado de cafés", CostoPorPersona: 420, MeGustaTotal: 4, ComentariosTotal: 2,
		Paradas: []Parada{{NegocioID: lugar, Llega: time.Now(), Nota: "secreta"}, {NegocioID: cerrado, Llega: time.Now()}}}
	out := NewFriendPlan(p, usuarios.User{ID: bson.NewObjectID(), Name: "Mariana"}, map[bson.ObjectID]string{lugar: "Café"},
		[]bson.ObjectID{oculta, jorge, me, ajena, extra1, extra2, extra3}, friends, me, true)

	assert.Equal(t, "Mariana", out.Autor.Nombre)
	assert.Equal(t, 7, out.Apuntados.Total)
	require.Len(t, out.Apuntados.Amigos, 3, "hasta 3 con nombre")
	assert.Equal(t, jorge.Hex(), out.Apuntados.Amigos[0].ID, "quien oculta su actividad y quien no es mi amiga no salen")
	assert.Equal(t, MineView{Apuntado: true, MeGusta: true}, out.Yo)
	require.Len(t, out.Paradas, 2)
	assert.Equal(t, &PlaceRef{ID: lugar.Hex(), Nombre: "Café"}, out.Paradas[0].Lugar)
	assert.Nil(t, out.Paradas[1].Lugar, "un lugar que ya no se ve sale sin datos")
}

package personas

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/contadores"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

func social(actividad string, resenas, planes, postales bool) usuarios.Social {
	return usuarios.Social{Activity: actividad, ShareReviews: &resenas, SharePlans: &planes, SharePostcards: &postales}
}

// La matriz de nv-82 para pestañas y contadores (nv-81).
func TestVeActividad_SegunActividadYAmistad(t *testing.T) {
	cases := []struct {
		actividad string
		amigo     bool
		ve        bool
	}{
		{usuarios.ActivityNobody, true, false},
		{usuarios.ActivityNobody, false, false},
		{usuarios.ActivityFriends, true, true},
		{usuarios.ActivityFriends, false, false},
		{"", true, true}, // cuenta previa: amigos por omisión
		{"", false, false},
		{usuarios.ActivityAll, false, true},
		{usuarios.ActivityAll, true, true},
	}
	for _, c := range cases {
		assert.Equal(t, c.ve, VeActividad(usuarios.Social{Activity: c.actividad}, c.amigo), "%q amigo=%v", c.actividad, c.amigo)
	}
}

func TestPestanaVisible_InterruptorPorPestana(t *testing.T) {
	todo := social(usuarios.ActivityAll, true, true, true)
	for _, p := range []string{PestanaMomentos, PestanaPlanes, PestanaResenas} {
		assert.True(t, PestanaVisible(todo, false, p), p)
	}
	assert.False(t, PestanaVisible(todo, false, "otra"), "pestaña desconocida = cerrada")
	sinResenas := social(usuarios.ActivityAll, false, true, true)
	assert.False(t, PestanaVisible(sinResenas, true, PestanaResenas))
	assert.True(t, PestanaVisible(sinResenas, true, PestanaPlanes))
	sinPlanes := social(usuarios.ActivityAll, true, false, true)
	assert.False(t, PestanaVisible(sinPlanes, true, PestanaPlanes))
	sinPostales := social(usuarios.ActivityAll, true, true, false)
	assert.False(t, PestanaVisible(sinPostales, true, PestanaMomentos))
	nadie := social(usuarios.ActivityNobody, true, true, true)
	assert.False(t, PestanaVisible(nadie, true, PestanaMomentos), "nadie cierra todo aunque el interruptor esté encendido")
}

type contadoresFijos struct{ c contadores.Contadores }

func (f contadoresFijos) De(context.Context, bson.ObjectID) (contadores.Contadores, error) {
	return f.c, nil
}

func TestContadoresDe_NullSegunPrivacidad(t *testing.T) {
	s := &Service{}
	s.WithContadores(contadoresFijos{contadores.Contadores{Planes: 9, PlanesPublicos: 2, Amigos: 5, Resenas: 3}})
	u := usuarios.User{ID: bson.NewObjectID()}

	u.Privacy.Social = social(usuarios.ActivityAll, true, true, true)
	out, err := s.contadoresDe(context.Background(), u, false)
	require.NoError(t, err)
	require.NotNil(t, out.Planes)
	assert.Equal(t, int64(2), *out.Planes, "otra persona solo cuenta los planes públicos")
	assert.Equal(t, int64(5), *out.Amigos)
	assert.Equal(t, int64(3), *out.Resenas)

	u.Privacy.Social = social(usuarios.ActivityAll, false, false, true)
	out, err = s.contadoresDe(context.Background(), u, false)
	require.NoError(t, err)
	assert.Nil(t, out.Planes)
	assert.Nil(t, out.Resenas)
	assert.NotNil(t, out.Amigos)

	for _, c := range []struct {
		actividad string
		amigo     bool
	}{{usuarios.ActivityNobody, true}, {usuarios.ActivityFriends, false}} {
		u.Privacy.Social = social(c.actividad, true, true, true)
		out, err = s.contadoresDe(context.Background(), u, c.amigo)
		require.NoError(t, err)
		raw, _ := json.Marshal(out)
		assert.JSONEq(t, `{"planes":null,"amigos":null,"resenas":null}`, string(raw), c.actividad)
	}
}

func TestProfileResponse_ContadoresSiempreVisibles(t *testing.T) {
	raw, err := json.Marshal(ProfileResponse{Person: Person{ID: "1"}, Relation: RelationNone})
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"contadores":{"planes":null,"amigos":null,"resenas":null}`)
	assert.Contains(t, string(raw), `"insignias_total":0`)
}

// Una cuenta previa a nv-79 no tiene número de nivel: va en el 1 con el nombre de la tabla.
func TestNivelDe_TablaDeNv79(t *testing.T) {
	assert.Equal(t, &NivelView{Numero: 1, Nombre: "Explorador"}, nivelDe(usuarios.User{}))
	assert.Equal(t, &NivelView{Numero: 6, Nombre: "Guía local"}, nivelDe(usuarios.User{LevelNumber: 6}))
}

package insignias

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestCatalogo_EsElDeLaMaqueta(t *testing.T) {
	ids := make([]string, 0, len(Catalogo))
	fuentes := map[string]bool{FuenteCafeterias: true, FuenteResenas: true, FuenteGrupos: true, FuenteLugares: true, FuentePostales: true, FuenteRacha: true, FuenteMeGusta: true}
	vistos := map[string]bool{}
	for _, d := range Catalogo {
		assert.False(t, vistos[d.ID], "id repetido: %s", d.ID)
		vistos[d.ID] = true
		ids = append(ids, d.ID)
		assert.Positive(t, d.Meta, d.ID)
		assert.True(t, fuentes[d.Fuente], "fuente desconocida en %s", d.ID)
		assert.NotEmpty(t, d.Nombre, d.ID)
		assert.NotEmpty(t, d.Descripcion, d.ID)
	}
	assert.Equal(t, []string{"catador_cafe", "resenista", "anfitrion_grupo", "viajero_pueblo", "cronista", "racha_3", "sello_oro", "querido", "explorador_total"}, ids)
	for _, viejo := range []string{"primer_plan", "planificador", "primera_resena", "critico", "explorador", "trotamundos", "buena_compania", "fotografo"} {
		assert.False(t, vistos[viejo], "el id de nv-78 %s ya no se muestra", viejo)
	}
}

func TestDefinicion_GanadaYProgreso(t *testing.T) {
	d := Definicion{ID: "viajero_pueblo", Fuente: FuenteLugares, Meta: 10}
	assert.False(t, d.Ganada(map[string]int{}))
	assert.Equal(t, 0, d.Progreso(map[string]int{}))
	assert.False(t, d.Ganada(map[string]int{FuenteLugares: 9}))
	assert.Equal(t, 9, d.Progreso(map[string]int{FuenteLugares: 9}))
	assert.True(t, d.Ganada(map[string]int{FuenteLugares: 10}))
	assert.Equal(t, 10, d.Progreso(map[string]int{FuenteLugares: 30}), "el avance no pasa la meta")
}

func TestNivelDe_Bordes(t *testing.T) {
	casos := []struct {
		puntos, numero int
		nombre         string
	}{
		{0, 1, NivelExplorador}, {149, 1, NivelExplorador}, {150, 2, NivelExplorador}, {299, 2, NivelExplorador},
		{300, 3, NivelExplorador}, {499, 3, NivelExplorador}, {500, 4, NivelAnfitrion}, {799, 4, NivelAnfitrion},
		{800, 5, NivelAnfitrion}, {1199, 5, NivelAnfitrion}, {1200, 6, NivelGuiaLocal}, {1700, 7, NivelGuiaLocal},
		{2299, 7, NivelGuiaLocal}, {2300, 8, NivelLeyenda}, {2999, 8, NivelLeyenda}, {3000, 9, NivelLeyenda}, {3700, 10, NivelLeyenda},
		{-10, 1, NivelExplorador},
	}
	for _, c := range casos {
		n := NivelDe(c.puntos)
		assert.Equal(t, c.numero, n.Numero, "%d puntos", c.puntos)
		assert.Equal(t, c.nombre, n.Nombre, "%d puntos", c.puntos)
	}
	assert.Equal(t, Nivel{Numero: 4, Nombre: NivelAnfitrion, Puntos: 500}, NivelNumero(4))
	assert.Equal(t, Nivel{Numero: 9, Nombre: NivelLeyenda, Puntos: 3000}, NivelNumero(9))
	assert.Equal(t, 1, NivelNumero(0).Numero)
}

// 2026-11-21 es sábado de la semana ISO 47.
var hoy = time.Date(2026, 11, 21, 18, 0, 0, 0, time.UTC)

func TestRacha(t *testing.T) {
	assert.Zero(t, Racha(nil, hoy))
	assert.Equal(t, 1, Racha([]string{"2026-11-16"}, hoy), "la semana actual cuenta")
	assert.Equal(t, 1, Racha([]string{"2026-11-10"}, hoy), "sin visita esta semana, cuenta hasta la anterior")
	assert.Equal(t, 3, Racha([]string{"2026-11-20", "2026-11-19", "2026-11-12", "2026-11-03"}, hoy))
	assert.Equal(t, 2, Racha([]string{"2026-11-20", "2026-11-12", "2026-10-28"}, hoy), "un hueco corta la racha")
	assert.Zero(t, Racha([]string{"2026-11-02"}, hoy), "dos semanas sin visita")
	assert.Equal(t, 1, Racha([]string{"basura", "2026-11-17"}, hoy))
}

func TestRacha_CambioDeAnio(t *testing.T) {
	// 2027-01-06 es miércoles de la semana 1 de 2027; la 53 de 2026 empieza el 2026-12-28.
	ahora := time.Date(2027, 1, 6, 18, 0, 0, 0, time.UTC)
	assert.Equal(t, 3, Racha([]string{"2027-01-05", "2026-12-29", "2026-12-22"}, ahora))
	// Domingo 2027-01-03 a las 23:30 en México ya es lunes en UTC: la semana es la 53.
	domingo := time.Date(2027, 1, 4, 5, 30, 0, 0, time.UTC)
	assert.Equal(t, 1, Racha([]string{"2027-01-03"}, domingo))
	assert.Equal(t, "2026-W53", semanaISO(domingo))
}

func TestEsperados_ReferenciasYTopeDePlanes(t *testing.T) {
	yo := bson.NewObjectID()
	visita, resena, postal := bson.NewObjectID(), bson.NewObjectID(), bson.NewObjectID()
	lunes := time.Date(2026, 11, 16, 18, 0, 0, 0, time.UTC)
	a := Actividad{Visitas: []Visita{{ID: visita, NegocioID: bson.NewObjectID(), Dia: "2026-11-16"}}, Resenas: []bson.ObjectID{resena}, Postales: []bson.ObjectID{postal}}
	for i := range 7 {
		a.Planes = append(a.Planes, PlanCreado{ID: bson.NewObjectID(), CreadoEn: lunes.Add(time.Duration(i) * time.Hour)})
	}
	a.Planes = append(a.Planes, PlanCreado{ID: bson.NewObjectID(), CreadoEn: lunes.AddDate(0, 0, 7)})
	got := Esperados(yo, a, map[string]time.Time{"resenista": hoy, "primer_plan": hoy})
	cuenta := map[string]int{}
	refs := map[string]bool{}
	for _, p := range got {
		cuenta[p.Motivo]++
		refs[p.Motivo+"/"+p.RefID] = true
	}
	assert.Equal(t, map[string]int{MotivoRegistro: 1, MotivoCheckin: 1, MotivoResena: 1, MotivoPostal: 1, MotivoPlan: 6, MotivoInsignia: 1}, cuenta,
		"5 planes en una semana y 1 en la siguiente; la insignia de nv-78 no suma")
	for _, r := range []string{"registro/" + yo.Hex(), "checkin/" + visita.Hex(), "resena/" + resena.Hex(), "postal/" + postal.Hex(), "plan/2026-W47#1", "plan/2026-W47#5", "plan/2026-W48#1", "insignia/resenista"} {
		assert.True(t, refs[r], r)
	}
	assert.False(t, refs["plan/2026-W47#6"])
	require.False(t, sumaAlCrear(MotivoRegistro), "los 50 de bienvenida ya se dieron al crear la cuenta")
	for m, v := range PuntosPor {
		assert.Positive(t, v, m)
		if m != MotivoRegistro {
			assert.True(t, sumaAlCrear(m), m)
		}
	}
	assert.Equal(t, map[string]int{"registro": 50, "checkin": 20, "resena": 30, "postal": 10, "plan": 5, "insignia": 50}, PuntosPor)
}

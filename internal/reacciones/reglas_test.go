package reacciones

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

func ptr(id bson.ObjectID) *bson.ObjectID { return &id }

func TestMirada_PersonaVisible(t *testing.T) {
	yo, dueno, bloqueado, oculto, otro := bson.NewObjectID(), bson.NewObjectID(), bson.NewObjectID(), bson.NewObjectID(), bson.NewObjectID()
	m := Mirada{Yo: yo, Dueno: dueno, Bloqueados: map[bson.ObjectID]bool{bloqueado: true}, Ocultos: map[bson.ObjectID]bool{oculto: true}}
	assert.True(t, m.PersonaVisible(yo))
	assert.True(t, m.PersonaVisible(otro))
	assert.False(t, m.PersonaVisible(bloqueado), "un bloqueo oculta en los dos sentidos")
	assert.False(t, m.PersonaVisible(oculto), "actividad: nadie no sale para terceros (nv-82)")

	m.Yo = dueno
	assert.True(t, m.PersonaVisible(oculto), "el dueño del objeto sí ve quién reaccionó")
	m.Ocultos = map[bson.ObjectID]bool{dueno: true}
	assert.True(t, m.PersonaVisible(dueno), "cada quien se ve a sí mismo")
}

func TestMirada_ComentarioVisible(t *testing.T) {
	yo, otro := bson.NewObjectID(), bson.NewObjectID()
	m := Mirada{Yo: yo, Dueno: bson.NewObjectID(), Bloqueados: map[bson.ObjectID]bool{}, Ocultos: map[bson.ObjectID]bool{}}
	for _, tc := range []struct {
		name  string
		c     Comentario
		wants bool
	}{
		{"publicado ajeno", Comentario{AutorID: ptr(otro), Estado: EstadoPublicado}, true},
		{"en moderación propio", Comentario{AutorID: ptr(yo), Estado: EstadoEnModeracion}, true},
		{"en moderación ajeno", Comentario{AutorID: ptr(otro), Estado: EstadoEnModeracion}, false},
		{"rechazado ajeno", Comentario{AutorID: ptr(otro), Estado: EstadoRechazado}, false},
		{"eliminado sin autor", Comentario{Estado: EstadoEliminado}, true},
		{"publicado sin autor", Comentario{Estado: EstadoPublicado}, false},
	} {
		assert.Equal(t, tc.wants, m.ComentarioVisible(tc.c), tc.name)
	}
	m.Bloqueados[otro] = true
	assert.False(t, m.ComentarioVisible(Comentario{AutorID: ptr(otro), Estado: EstadoPublicado}), "bloqueado")
}

func TestPuedeBorrar_AutorODueno(t *testing.T) {
	autor, dueno, otro := bson.NewObjectID(), bson.NewObjectID(), bson.NewObjectID()
	c := Comentario{AutorID: ptr(autor), DuenoID: dueno, Estado: EstadoPublicado}
	assert.True(t, PuedeBorrar(c, autor))
	assert.True(t, PuedeBorrar(c, dueno))
	assert.False(t, PuedeBorrar(c, otro))
	c.Estado = EstadoEliminado
	assert.False(t, PuedeBorrar(c, autor), "ya eliminado")
}

func TestLimpiarTexto(t *testing.T) {
	got, ok := LimpiarTexto("  hola  ")
	assert.True(t, ok)
	assert.Equal(t, "hola", got)
	_, ok = LimpiarTexto("   ")
	assert.False(t, ok, "solo espacios")
	_, ok = LimpiarTexto(strings.Repeat("ñ", MaxTexto))
	assert.True(t, ok, "300 runas aunque sean 600 bytes")
	_, ok = LimpiarTexto(strings.Repeat("a", MaxTexto+1))
	assert.False(t, ok)
}

func TestPadreValido_UnNivel(t *testing.T) {
	yo, autor, obj := bson.NewObjectID(), bson.NewObjectID(), bson.NewObjectID()
	m := Mirada{Yo: yo, Bloqueados: map[bson.ObjectID]bool{}, Ocultos: map[bson.ObjectID]bool{}}
	padre := Comentario{ObjetoTipo: ObjetoMomento, ObjetoID: obj, AutorID: ptr(autor), Estado: EstadoPublicado}
	assert.True(t, PadreValido(padre, ObjetoMomento, obj, m))
	assert.False(t, PadreValido(padre, ObjetoPlan, obj, m), "otro tipo")
	assert.False(t, PadreValido(padre, ObjetoMomento, bson.NewObjectID(), m), "otro objeto")
	resp := padre
	resp.PadreID = ptr(bson.NewObjectID())
	assert.False(t, PadreValido(resp, ObjetoMomento, obj, m), "respuesta a una respuesta")
	dudoso := padre
	dudoso.Estado = EstadoEnModeracion
	assert.False(t, PadreValido(dudoso, ObjetoMomento, obj, m), "en moderación")
}

func TestTotales_TodosCuentaUnaVez(t *testing.T) {
	assert.Equal(t, TotalesView{Todos: 10, MeGusta: 8, Van: 4}, Totales(8, 4, 2))
}

func TestOrdenar_Pestanas(t *testing.T) {
	a, b, c := bson.NewObjectID(), bson.NewObjectID(), bson.NewObjectID()
	likes := []MeGusta{{UsuarioID: a}, {UsuarioID: b}}
	van := []bson.ObjectID{b, c}

	todos, marcas := ordenar(PestanaTodos, likes, van)
	assert.Equal(t, []bson.ObjectID{a, b, c}, todos)
	assert.Equal(t, marca{meGusta: true, va: true}, marcas[b])
	meGusta, _ := ordenar(PestanaMeGusta, likes, van)
	assert.Equal(t, []bson.ObjectID{a, b}, meGusta)
	vanLista, _ := ordenar(PestanaVan, likes, van)
	assert.Equal(t, []bson.ObjectID{b, c}, vanLista)
}

func TestRelacion(t *testing.T) {
	yo, amigo, env, rec, bloq, nadie := bson.NewObjectID(), bson.NewObjectID(), bson.NewObjectID(), bson.NewObjectID(), bson.NewObjectID(), bson.NewObjectID()
	rels := map[bson.ObjectID]amistades.Friendship{
		amigo: {From: amigo, To: yo, Status: amistades.StatusAccepted},
		env:   {From: yo, To: env, Status: amistades.StatusPending},
		rec:   {From: rec, To: yo, Status: amistades.StatusPending},
		bloq:  {From: bloq, To: yo, Status: amistades.StatusBlocked},
	}
	assert.Equal(t, RelacionYo, relacion(yo, yo, rels))
	assert.Equal(t, RelacionAmigo, relacion(yo, amigo, rels))
	assert.Equal(t, RelacionEnviada, relacion(yo, env, rels))
	assert.Equal(t, RelacionRecibida, relacion(yo, rec, rels))
	assert.Equal(t, RelacionNinguna, relacion(yo, nadie, rels))
	assert.Empty(t, relacion(yo, bloq, rels), "un bloqueo no se muestra")
}

func TestVista_EliminadoSinAutorNiTexto(t *testing.T) {
	yo := bson.NewObjectID()
	c := Comentario{ID: bson.NewObjectID(), AutorID: ptr(yo), Texto: "secreto", Estado: EstadoEliminado}
	v := vista(c, nil, yo)
	assert.Nil(t, v.Autor)
	assert.Empty(t, v.Texto)
	assert.False(t, v.Propio)
	assert.False(t, v.PuedeBorrar)
}

func TestArmarHilos_RespuestasYEliminados(t *testing.T) {
	yo, ana := bson.NewObjectID(), bson.NewObjectID()
	m := Mirada{Yo: yo, Bloqueados: map[bson.ObjectID]bool{}, Ocultos: map[bson.ObjectID]bool{}}
	autores := map[bson.ObjectID]usuarios.User{ana: {ID: ana, Name: "Ana"}, yo: {ID: yo, Name: "Yo"}}
	conHijos := Comentario{ID: bson.NewObjectID(), Estado: EstadoEliminado}
	solo := Comentario{ID: bson.NewObjectID(), Estado: EstadoEliminado}
	normal := Comentario{ID: bson.NewObjectID(), AutorID: ptr(ana), Texto: "hola", Estado: EstadoPublicado}
	resp := []Comentario{
		{ID: bson.NewObjectID(), PadreID: ptr(conHijos.ID), AutorID: ptr(yo), Texto: "r1", Estado: EstadoEnModeracion},
		{ID: bson.NewObjectID(), PadreID: ptr(normal.ID), AutorID: ptr(ana), Estado: EstadoEliminado},
	}
	got := armarHilos([]Comentario{conHijos, solo, normal}, resp, m, autores)
	if assert.Len(t, got, 2, "el eliminado sin respuestas no sale") {
		assert.Nil(t, got[0].Autor)
		assert.Len(t, got[0].Respuestas, 1, "mi respuesta en moderación la veo yo")
		assert.Equal(t, "Ana", got[1].Autor.Nombre)
		assert.Empty(t, got[1].Respuestas, "una respuesta eliminada no sale")
	}
}

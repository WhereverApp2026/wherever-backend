package reacciones

import (
	"strings"
	"unicode/utf8"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Mirada es quién mira una lista: la persona, el dueño del objeto, sus bloqueos y quiénes
// ocultan su actividad (nv-82).
type Mirada struct {
	Yo         bson.ObjectID
	Dueno      bson.ObjectID
	Bloqueados map[bson.ObjectID]bool
	Ocultos    map[bson.ObjectID]bool
}

// PersonaVisible dice si una persona aparece en una lista del objeto: nunca con bloqueo; quien
// oculta su actividad (`actividad: nadie`) solo para sí y para el dueño del objeto.
func (m Mirada) PersonaVisible(id bson.ObjectID) bool {
	switch {
	case id == m.Yo:
		return true
	case m.Bloqueados[id]:
		return false
	case m.Ocultos[id]:
		return m.Yo == m.Dueno
	default:
		return true
	}
}

// ComentarioVisible: el eliminado sale como «Comentario eliminado» para todos; en moderación o
// rechazado, solo para su autor; el publicado, según su autor (PersonaVisible).
func (m Mirada) ComentarioVisible(c Comentario) bool {
	switch {
	case c.Estado == EstadoEliminado:
		return true
	case c.De(m.Yo):
		return true
	case c.Estado != EstadoPublicado || c.AutorID == nil:
		return false
	default:
		return m.PersonaVisible(*c.AutorID)
	}
}

// PuedeBorrar: el autor o el dueño del objeto, mientras no esté eliminado.
func PuedeBorrar(c Comentario, yo bson.ObjectID) bool {
	return c.Estado != EstadoEliminado && (c.De(yo) || c.DuenoID == yo)
}

// LimpiarTexto quita espacios de las orillas; vacío o con más de MaxTexto runas = inválido.
func LimpiarTexto(texto string) (string, bool) {
	t := strings.TrimSpace(texto)
	return t, t != "" && utf8.RuneCountInString(t) <= MaxTexto
}

// PadreValido dice si se puede responder a `padre` en el objeto: del mismo objeto, de primer
// nivel (respuestas de un nivel), publicado y visible para quien responde.
func PadreValido(padre Comentario, tipo string, objetoID bson.ObjectID, m Mirada) bool {
	return padre.ObjetoTipo == tipo && padre.ObjetoID == objetoID && padre.PadreID == nil &&
		padre.Estado == EstadoPublicado && m.ComentarioVisible(padre)
}

// Totales de quién reaccionó: `todos` cuenta una vez a quien dio me gusta y además va.
func Totales(meGusta, van, ambos int64) TotalesView {
	return TotalesView{Todos: meGusta + van - ambos, MeGusta: meGusta, Van: van}
}

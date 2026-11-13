// Package chat es la conversación dentro de un grupo de nv-37 (nv-57, hoja J3): mensajes de
// texto, reacciones y avisos del sistema, con historial paginado, búsqueda, push agrupado y
// ARCO. Solo los integrantes confirmados leen y escriben.
package chat

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Colecciones: los mensajes y el candado del push agrupado (uno por grupo y persona).
const (
	Collection       = "mensajes"
	NoticeCollection = "mensajes_avisos"
)

// Tipos de mensaje. Los avisos del sistema (p. ej. el cierre de una votación) no tienen autor.
const (
	KindText   = "texto"
	KindSystem = "sistema"
)

// Límites de la spec.
const (
	MaxText = 1000
	// PreviewRunes es el largo de `grupos.ultimo_mensaje.texto`.
	PreviewRunes = 80
	// SendLimit mensajes por SendWindow y por persona, contados en MongoDB (sin Redis).
	SendLimit  = 30
	SendWindow = time.Minute
	// NoticeEvery: máximo un push de chat por grupo y persona en ese lapso.
	NoticeEvery = 5 * time.Minute
	// DefaultLimit y MaxLimit de la lista.
	DefaultLimit = 30
	MaxLimit     = 100
)

// Emojis es la lista fija de reacciones: no se guarda texto libre.
var Emojis = []string{"👍", "❤️", "😂", "😮", "😢", "🎉"}

// Reaction es la reacción de una persona; una por persona y mensaje.
type Reaction struct {
	UserID bson.ObjectID `bson:"usuario_id"`
	Emoji  string        `bson:"emoji"`
}

// Message es un documento de `mensajes`. El `_id` ordena por tiempo. Un mensaje borrado
// conserva su lugar con `texto` y `reacciones` vacíos y `eliminado_en`.
type Message struct {
	ID        bson.ObjectID  `bson:"_id"`
	GroupID   bson.ObjectID  `bson:"grupo_id"`
	AuthorID  *bson.ObjectID `bson:"autor_id,omitempty"`
	Kind      string         `bson:"tipo"`
	Text      string         `bson:"texto"`
	Reactions []Reaction     `bson:"reacciones"`
	ClientID  string         `bson:"id_cliente,omitempty"`
	DeletedAt *time.Time     `bson:"eliminado_en,omitempty"`
	CreatedAt time.Time      `bson:"creado_en"`
}

// Preview recorta el texto para `grupos.ultimo_mensaje` sin partir caracteres.
func Preview(text string) string {
	runes := []rune(text)
	if len(runes) <= PreviewRunes {
		return text
	}
	return string(runes[:PreviewRunes])
}

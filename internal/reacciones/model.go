// Package reacciones es el «me gusta» y los comentarios de momentos y planes (nv-83): quién
// reaccionó, respuestas de un nivel, moderación del texto y avisos agrupados. Los permisos los
// deciden postales (momentos) y planes; este paquete solo pregunta.
package reacciones

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	platformmongo "github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
)

// Colecciones de nv-83.
const (
	MeGustaCollection     = "me_gusta"
	ComentariosCollection = "comentarios"
)

// Tipos de objeto que admiten me gusta y comentarios.
const (
	ObjetoMomento = "momento"
	ObjetoPlan    = "plan"
)

// Estados de un comentario. `eliminado` queda como «Comentario eliminado», sin texto ni autor.
const (
	EstadoPublicado    = "publicado"
	EstadoEnModeracion = "en_moderacion"
	EstadoRechazado    = "rechazado"
	EstadoEliminado    = "eliminado"
)

// TipoCola es el tipo de los comentarios dudosos en la cola de nv-43.
const TipoCola = "comentario"

// Pestañas de GET /{objeto}/{id}/reacciones.
const (
	PestanaTodos   = "todos"
	PestanaMeGusta = "me_gusta"
	PestanaVan     = "van"
)

// Límites de la spec.
const (
	MaxTexto = 300
	// MaxRespuestas son las respuestas que trae una página de comentarios, entre todos sus hilos.
	MaxRespuestas = 500
	// MaxPersonas es el tope de la lista de quién reaccionó (las más recientes primero).
	MaxPersonas = 100
)

// MeGusta es un documento de `me_gusta`: uno por persona y objeto (`ux_me_gusta`).
type MeGusta struct {
	ID         bson.ObjectID `bson:"_id"`
	ObjetoTipo string        `bson:"objeto_tipo"`
	ObjetoID   bson.ObjectID `bson:"objeto_id"`
	UsuarioID  bson.ObjectID `bson:"usuario_id"`
	// DuenoID es el dueño del objeto: al borrar su cuenta se van los me gusta de sus objetos.
	DuenoID  bson.ObjectID `bson:"dueno_id"`
	CreadoEn time.Time     `bson:"creado_en"`
}

// Comentario es un documento de `comentarios`. `padre_id` solo en respuestas (un nivel); al
// borrar la cuenta (ARCO) pierde `autor_id` y `texto`.
type Comentario struct {
	ID         bson.ObjectID  `bson:"_id"`
	ObjetoTipo string         `bson:"objeto_tipo"`
	ObjetoID   bson.ObjectID  `bson:"objeto_id"`
	AutorID    *bson.ObjectID `bson:"autor_id,omitempty"`
	// DuenoID es el dueño del objeto: puede borrar el comentario y recibe el aviso.
	DuenoID     bson.ObjectID  `bson:"dueno_id"`
	PadreID     *bson.ObjectID `bson:"padre_id,omitempty"`
	Texto       string         `bson:"texto"`
	Estado      string         `bson:"estado"`
	CreadoEn    time.Time      `bson:"creado_en"`
	EliminadoEn *time.Time     `bson:"eliminado_en,omitempty"`
}

// De dice si el comentario es de la persona.
func (c Comentario) De(id bson.ObjectID) bool { return c.AutorID != nil && *c.AutorID == id }

func idx(keys bson.D, o *options.IndexOptionsBuilder) mongo.IndexModel {
	return mongo.IndexModel{Keys: keys, Options: o}
}

// Indexes son los índices de nv-83; los crea solo `scripts/indexes`.
func Indexes() []platformmongo.IndexSet {
	return []platformmongo.IndexSet{
		{Collection: MeGustaCollection, Models: []mongo.IndexModel{
			idx(bson.D{{Key: "objeto_tipo", Value: 1}, {Key: "objeto_id", Value: 1}, {Key: "usuario_id", Value: 1}}, options.Index().SetName("ux_me_gusta").SetUnique(true)),
			idx(bson.D{{Key: "objeto_tipo", Value: 1}, {Key: "objeto_id", Value: 1}, {Key: "creado_en", Value: -1}}, options.Index().SetName("ix_me_gusta_objeto")),
			idx(bson.D{{Key: "usuario_id", Value: 1}}, options.Index().SetName("ix_me_gusta_usuario")),
			idx(bson.D{{Key: "dueno_id", Value: 1}}, options.Index().SetName("ix_me_gusta_dueno")),
		}},
		{Collection: ComentariosCollection, Models: []mongo.IndexModel{
			idx(bson.D{{Key: "objeto_tipo", Value: 1}, {Key: "objeto_id", Value: 1}, {Key: "padre_id", Value: 1}, {Key: "creado_en", Value: 1}, {Key: "_id", Value: 1}},
				options.Index().SetName("ix_comentarios_objeto")),
			idx(bson.D{{Key: "autor_id", Value: 1}}, options.Index().SetName("ix_comentarios_autor")),
			idx(bson.D{{Key: "dueno_id", Value: 1}}, options.Index().SetName("ix_comentarios_dueno")),
		}},
	}
}

// Package favoritos guarda los negocios y planes que le gustan a la persona (nv-54, hoja
// A3): modelo, repositorio, service, handler y ARCO. Lee negocios y planes por interfaz;
// ninguno de ellos importa favoritos.
package favoritos

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Collection guarda los favoritos (A3 §1).
const Collection = "favoritos"

// Tipos de objetivo.
const (
	TypeBusiness = "negocio"
	TypePlan     = "plan"
)

// MaxPerType es el tope de favoritos por tipo; también acota ids y búsqueda en memoria.
const MaxPerType = 500

// Summary es lo que se muestra sin leer el objetivo; en un plan solo `nombre`.
type Summary struct {
	Name     string `bson:"nombre"`
	PhotoURL string `bson:"foto_url,omitempty"`
	Category string `bson:"categoria,omitempty"`
}

// Favorite es un documento de `favoritos`.
type Favorite struct {
	ID        bson.ObjectID `bson:"_id"`
	UserID    bson.ObjectID `bson:"usuario_id"`
	Type      string        `bson:"tipo"`
	TargetID  bson.ObjectID `bson:"objetivo_id"`
	Summary   Summary       `bson:"resumen"`
	CreatedAt time.Time     `bson:"creado_en"`
}

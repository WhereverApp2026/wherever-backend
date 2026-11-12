package semantica

import (
	"math"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
)

// Query es GET /lugares/semantica.
type Query struct {
	Q      string `query:"q" validate:"required,min=2,max=200"`
	Limite int    `query:"limite" validate:"omitempty,min=1,max=30"`
}

// Item es la tarjeta de GET /lugares (sin distancia) con la afinidad de 0 a 1 cuando la
// búsqueda fue semántica.
type Item struct {
	negocios.PlaceItem
	Afinidad *float64 `json:"afinidad,omitempty"`
}

// Respuesta dice de dónde salió la lista: `semantica` o `reglas` (con aviso).
type Respuesta struct {
	Fuente string `json:"fuente"`
	Aviso  string `json:"aviso,omitempty"`
	Items  []Item `json:"items"`
}

func respuestaSemantica(bs []negocios.Business, afin map[bson.ObjectID]float64, ahora time.Time) Respuesta {
	items := make([]Item, len(bs))
	for i, b := range bs {
		a := math.Round(afin[b.ID]*1000) / 1000
		items[i] = Item{PlaceItem: negocios.CardOf(b, ahora), Afinidad: &a}
	}
	return Respuesta{Fuente: FuenteSemantica, Items: items}
}

func respuestaReglas(bs []negocios.Business, ahora time.Time) Respuesta {
	items := make([]Item, len(bs))
	for i, b := range bs {
		items[i] = Item{PlaceItem: negocios.CardOf(b, ahora)}
	}
	return Respuesta{Fuente: FuenteReglas, Aviso: avisoReglas, Items: items}
}

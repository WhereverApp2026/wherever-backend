package planes

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
)

func TestRoundCoord_DosDecimales(t *testing.T) {
	assert.Equal(t, 19.43, roundCoord(19.432612))
	assert.Equal(t, -99.13, roundCoord(-99.133209))
	assert.Equal(t, 20.0, roundCoord(19.996))
}

func TestInWindow_HoyYMananaEnMexico(t *testing.T) {
	loc := negocios.MexicoCity()
	now := time.Date(2026, 11, 14, 23, 30, 0, 0, loc) // 05:30 del 15 en UTC
	day := func(d int) time.Time { return time.Date(2026, 11, d, 0, 0, 0, 0, loc).UTC() }
	assert.True(t, inWindow(day(14), now), "hoy")
	assert.True(t, inWindow(day(15), now), "mañana")
	assert.False(t, inWindow(day(16), now), "pasado mañana")
	assert.False(t, inWindow(day(13), now), "ayer")
}

func TestPackageETag(t *testing.T) {
	assert.Equal(t, `"3-17"`, PackageETag(PackageResponse{Version: 3, SyncSeq: 17}))
}

func TestPublicView_SinNotas(t *testing.T) {
	p := Plan{OwnerID: bson.NewObjectID(), Paradas: []Parada{{Nota: "llevar regalo"}, {Nota: "cumpleaños"}}}
	out := publicView(p)
	for _, s := range out.Paradas {
		assert.Empty(t, s.Nota)
	}
	assert.Equal(t, "llevar regalo", p.Paradas[0].Nota, "no toca el plan original")
}

func TestAlternativeConditions(t *testing.T) {
	in := bson.NewObjectID()
	p := Plan{Paradas: []Parada{{NegocioID: in}}}
	current := negocios.Business{Category: "restaurante", PriceLevel: 2}
	stop := Parada{Llega: time.Date(2026, 11, 20, 18, 0, 0, 0, time.UTC)}

	similar := alternativeConditions(p, stop, current, "")
	assert.Len(t, similar, 4, "abierto, fuera del plan, giro y precio")
	assert.Contains(t, similar, bson.D{{Key: "categoria", Value: "restaurante"}})
	assert.Contains(t, similar, bson.D{{Key: "nivel_precio", Value: bson.D{{Key: "$gte", Value: 1}, {Key: "$lte", Value: 3}}}})

	byText := alternativeConditions(p, stop, current, "Café (Tula)")
	assert.Len(t, byText, 3, "con texto no filtra por giro ni precio")
	assert.Contains(t, byText, bson.D{{Key: "nombre_normalizado", Value: bson.D{{Key: "$regex", Value: `cafe \(tula\)`}}}}, "sin acentos y sin operadores de regex")
}

func TestPlan_Normalize(t *testing.T) {
	p := Plan{}
	p.normalize()
	assert.Equal(t, VisibilityPrivate, p.Visibilidad, "los planes de antes son privados")
	assert.False(t, p.Public())
	p.Visibilidad = VisibilityPublic
	assert.True(t, p.Public())
}

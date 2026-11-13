package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/favoritos"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
)

const favPath = "/api/v1/favoritos"

// favSetup es una persona con un plan propio y la app lista.
type favSetup struct {
	env  *authtest.Env
	app  *fiber.App
	me   amigo
	plan planes.Plan
	shop bson.ObjectID
}

func newFavoritos(t *testing.T) favSetup {
	t.Helper()
	env := negociosEnv(t)
	env.ClearRateLimits(t)
	shop := placeDoc(placeSpec{slug: "fav-base"})
	_, err := env.Mongo.DB().Collection(negocios.Collection).InsertOne(context.Background(), shop)
	require.NoError(t, err)
	// newPlan de grupos crea un plan con una parada en shop.
	g := gruposSetup{env: env, app: env.App(), org: seedAmigo(t, env, "favorita"), shop: shop.ID}
	return favSetup{env: env, app: g.app, me: g.org, plan: g.newPlan(t, g.org), shop: shop.ID}
}

func allDay() []negocios.VentanaMin {
	out := make([]negocios.VentanaMin, 0, 7)
	for d := range 7 {
		out = append(out, negocios.VentanaMin{Dia: d, Desde: 0, Hasta: 24 * 60})
	}
	return out
}

// shop inserta un negocio con nombre, cupones y horario dados.
func (s favSetup) shopNamed(t *testing.T, slug, name string, coupons int, windows []negocios.VentanaMin) bson.ObjectID {
	t.Helper()
	b := placeDoc(placeSpec{slug: slug, windows: windows})
	b.Name, b.Coupons = name, coupons
	_, err := s.env.Mongo.DB().Collection(negocios.Collection).InsertOne(context.Background(), b)
	require.NoError(t, err)
	return b.ID
}

func favCall(t *testing.T, app *fiber.App, method, path, token string, want int) map[string]any {
	t.Helper()
	return amistadCall(t, app, method, path, nil, token, want)
}

func favItems(t *testing.T, body map[string]any) []map[string]any {
	t.Helper()
	raw, err := json.Marshal(body["items"])
	require.NoError(t, err)
	var out []map[string]any
	require.NoError(t, json.Unmarshal(raw, &out))
	return out
}

func (s favSetup) count(t *testing.T) int64 {
	t.Helper()
	n, err := s.env.Mongo.DB().Collection(favoritos.Collection).CountDocuments(context.Background(), bson.D{{Key: "usuario_id", Value: s.me.user.ID}})
	require.NoError(t, err)
	return n
}

func TestFavoritos_MarcarDosVecesDejaUnDocumento(t *testing.T) {
	s := newFavoritos(t)
	path := favPath + "/negocio/" + s.shop.Hex()
	first := favCall(t, s.app, http.MethodPut, path, s.me.token, http.StatusOK)
	second := favCall(t, s.app, http.MethodPut, path, s.me.token, http.StatusOK)
	assert.Equal(t, first["creado_en"], second["creado_en"])
	assert.Equal(t, "negocio", first["tipo"])
	assert.Equal(t, int64(1), s.count(t))

	plan := favCall(t, s.app, http.MethodPut, favPath+"/plan/"+s.plan.ID.Hex(), s.me.token, http.StatusOK)
	summary, ok := plan["resumen"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Salida", summary["nombre"])
	assert.NotContains(t, plan, "cupon_hoy")
	assert.Equal(t, int64(2), s.count(t))
}

func TestFavoritos_ObjetivoNoValidoEs404(t *testing.T) {
	s := newFavoritos(t)
	other := seedAmigo(t, s.env, "favajeno")
	paused := placeDoc(placeSpec{slug: "fav-pausado", state: negocios.StatusPaused})
	_, err := s.env.Mongo.DB().Collection(negocios.Collection).InsertOne(context.Background(), paused)
	require.NoError(t, err)
	for _, path := range []string{
		favPath + "/plan/" + s.plan.ID.Hex(),             // plan ajeno
		favPath + "/negocio/" + paused.ID.Hex(),          // negocio pausado
		favPath + "/negocio/" + bson.NewObjectID().Hex(), // inexistente
	} {
		favCall(t, s.app, http.MethodPut, path, other.token, http.StatusNotFound)
	}
	n, err := s.env.Mongo.DB().Collection(favoritos.Collection).CountDocuments(context.Background(), bson.D{{Key: "usuario_id", Value: other.user.ID}})
	require.NoError(t, err)
	assert.Zero(t, n)
}

func TestFavoritos_ListaOrdenCursorCuponYHorario(t *testing.T) {
	s := newFavoritos(t)
	open := s.shopNamed(t, "fav-abierto", "Café Ábaco", 2, allDay())
	closed := s.shopNamed(t, "fav-cerrado", "Bar Brisa", 0, []negocios.VentanaMin{})
	for _, id := range []bson.ObjectID{s.shop, open, closed} {
		favCall(t, s.app, http.MethodPut, favPath+"/negocio/"+id.Hex(), s.me.token, http.StatusOK)
		time.Sleep(5 * time.Millisecond)
	}
	page := favCall(t, s.app, http.MethodGet, favPath+"?tipo=negocio&limite=2", s.me.token, http.StatusOK)
	items := favItems(t, page)
	require.Len(t, items, 2)
	assert.Equal(t, closed.Hex(), items[0]["objetivo_id"], "más reciente primero")
	assert.Equal(t, false, items[0]["abierto_ahora"])
	assert.Equal(t, false, items[0]["cupon_hoy"])
	assert.Equal(t, open.Hex(), items[1]["objetivo_id"])
	assert.Equal(t, true, items[1]["abierto_ahora"])
	assert.Equal(t, true, items[1]["cupon_hoy"])
	next, _ := page["siguiente_cursor"].(string)
	require.NotEmpty(t, next)

	rest := favItems(t, favCall(t, s.app, http.MethodGet, favPath+"?tipo=negocio&limite=2&cursor="+next, s.me.token, http.StatusOK))
	require.Len(t, rest, 1)
	assert.Equal(t, s.shop.Hex(), rest[0]["objetivo_id"])

	coupon := favItems(t, favCall(t, s.app, http.MethodGet, favPath+"?tipo=negocio&con_cupon=true", s.me.token, http.StatusOK))
	require.Len(t, coupon, 1)
	assert.Equal(t, open.Hex(), coupon[0]["objetivo_id"])

	// El cursor de una pestaña no sirve en la otra.
	favCall(t, s.app, http.MethodGet, favPath+"?tipo=plan&cursor="+next, s.me.token, http.StatusUnprocessableEntity)
}

func TestFavoritos_IdsYBuscarSinAcentos(t *testing.T) {
	s := newFavoritos(t)
	cafe := s.shopNamed(t, "fav-cafe", "Café Ñandú", 0, nil)
	favCall(t, s.app, http.MethodPut, favPath+"/negocio/"+cafe.Hex(), s.me.token, http.StatusOK)
	favCall(t, s.app, http.MethodPut, favPath+"/negocio/"+s.shop.Hex(), s.me.token, http.StatusOK)
	favCall(t, s.app, http.MethodPut, favPath+"/plan/"+s.plan.ID.Hex(), s.me.token, http.StatusOK)

	ids := favCall(t, s.app, http.MethodGet, favPath+"/ids?tipo=negocio", s.me.token, http.StatusOK)
	assert.ElementsMatch(t, []any{cafe.Hex(), s.shop.Hex()}, ids["ids"])

	found := favItems(t, favCall(t, s.app, http.MethodGet, favPath+"/buscar?q=CAFE", s.me.token, http.StatusOK))
	require.Len(t, found, 1)
	assert.Equal(t, cafe.Hex(), found[0]["objetivo_id"])
	plans := favItems(t, favCall(t, s.app, http.MethodGet, favPath+"/buscar?q=sal&tipo=plan", s.me.token, http.StatusOK))
	require.Len(t, plans, 1)
	favCall(t, s.app, http.MethodGet, favPath+"/buscar?q=c", s.me.token, http.StatusUnprocessableEntity)

	// Otra persona no ve mis favoritos.
	other := seedAmigo(t, s.env, "favotra")
	none := favCall(t, s.app, http.MethodGet, favPath+"/ids?tipo=negocio", other.token, http.StatusOK)
	assert.Empty(t, none["ids"])
}

func TestFavoritos_QuitarEsIdempotente(t *testing.T) {
	s := newFavoritos(t)
	path := favPath + "/negocio/" + s.shop.Hex()
	favCall(t, s.app, http.MethodPut, path, s.me.token, http.StatusOK)
	favCall(t, s.app, http.MethodDelete, path, s.me.token, http.StatusNoContent)
	favCall(t, s.app, http.MethodDelete, path, s.me.token, http.StatusNoContent)
	assert.Zero(t, s.count(t))
	favCall(t, s.app, http.MethodDelete, favPath+"/otro/"+s.shop.Hex(), s.me.token, http.StatusUnprocessableEntity)
}

func TestFavoritos_PausadoSeOcultaYBorradoSeLimpia(t *testing.T) {
	s := newFavoritos(t)
	gone := s.shopNamed(t, "fav-borrado", "Borrado", 0, nil)
	for _, id := range []bson.ObjectID{s.shop, gone} {
		favCall(t, s.app, http.MethodPut, favPath+"/negocio/"+id.Hex(), s.me.token, http.StatusOK)
	}
	favCall(t, s.app, http.MethodPut, favPath+"/plan/"+s.plan.ID.Hex(), s.me.token, http.StatusOK)
	col := s.env.Mongo.DB().Collection(negocios.Collection)
	ctx := context.Background()
	_, err := col.UpdateByID(ctx, s.shop, bson.D{{Key: "$set", Value: bson.D{{Key: "estado", Value: negocios.StatusPaused}}}})
	require.NoError(t, err)
	_, err = col.UpdateByID(ctx, gone, bson.D{{Key: "$set", Value: bson.D{{Key: "eliminado_en", Value: time.Now().UTC()}}}})
	require.NoError(t, err)
	got := planCall(t, s.app, http.MethodGet, "/api/v1/planes/"+s.plan.ID.Hex(), nil, s.me.token, "")
	require.Equal(t, http.StatusOK, got.status, got.body)
	del := planCall(t, s.app, http.MethodDelete, "/api/v1/planes/"+s.plan.ID.Hex(), nil, s.me.token, got.etag)
	require.Equal(t, http.StatusNoContent, del.status, del.body)

	assert.Empty(t, favItems(t, favCall(t, s.app, http.MethodGet, favPath+"?tipo=negocio", s.me.token, http.StatusOK)))
	assert.Empty(t, favItems(t, favCall(t, s.app, http.MethodGet, favPath+"?tipo=plan", s.me.token, http.StatusOK)))
	assert.Equal(t, int64(1), s.count(t), "solo queda el del negocio pausado")

	_, err = col.UpdateByID(ctx, s.shop, bson.D{{Key: "$set", Value: bson.D{{Key: "estado", Value: negocios.StatusApproved}}}})
	require.NoError(t, err)
	assert.Len(t, favItems(t, favCall(t, s.app, http.MethodGet, favPath+"?tipo=negocio", s.me.token, http.StatusOK)), 1)
}

func TestFavoritos_TopeDe500PorTipo(t *testing.T) {
	s := newFavoritos(t)
	docs := make([]any, 0, favoritos.MaxPerType)
	now := time.Now().UTC()
	for i := range favoritos.MaxPerType {
		docs = append(docs, favoritos.Favorite{
			ID: bson.NewObjectID(), UserID: s.me.user.ID, Type: favoritos.TypeBusiness, TargetID: bson.NewObjectID(),
			Summary: favoritos.Summary{Name: fmt.Sprintf("n%d", i)}, CreatedAt: now,
		})
	}
	_, err := s.env.Mongo.DB().Collection(favoritos.Collection).InsertMany(context.Background(), docs)
	require.NoError(t, err)
	favCall(t, s.app, http.MethodPut, favPath+"/negocio/"+s.shop.Hex(), s.me.token, http.StatusConflict)
	// El tope es por tipo: el plan sí entra.
	favCall(t, s.app, http.MethodPut, favPath+"/plan/"+s.plan.ID.Hex(), s.me.token, http.StatusOK)
}

func TestFavoritos_ARCOExportaYBorraConLaCuenta(t *testing.T) {
	s := newFavoritos(t)
	other := seedAmigo(t, s.env, "favqueda")
	favCall(t, s.app, http.MethodPut, favPath+"/negocio/"+s.shop.Hex(), s.me.token, http.StatusOK)
	favCall(t, s.app, http.MethodPut, favPath+"/plan/"+s.plan.ID.Hex(), s.me.token, http.StatusOK)
	favCall(t, s.app, http.MethodPut, favPath+"/negocio/"+s.shop.Hex(), other.token, http.StatusOK)

	priv := favoritos.NewPrivacy(favoritos.NewRepository(s.env.Mongo.DB()))
	ctx := context.Background()
	got, err := priv.Export(ctx, s.me.user.ID)
	require.NoError(t, err)
	raw, err := json.Marshal(got)
	require.NoError(t, err)
	var rows []map[string]any
	require.NoError(t, json.Unmarshal(raw, &rows))
	require.Len(t, rows, 2)
	for _, r := range rows {
		assert.ElementsMatch(t, []string{"tipo", "objetivo_id", "nombre", "creado_en"}, keys(r))
	}

	require.NoError(t, priv.PurgeHistory(ctx, s.me.user.ID))
	assert.Equal(t, int64(2), s.count(t), "borrar historial no toca favoritos")
	require.NoError(t, priv.PurgeAccount(ctx, s.me.user.ID))
	assert.Zero(t, s.count(t))
	n, err := s.env.Mongo.DB().Collection(favoritos.Collection).CountDocuments(ctx, bson.D{{Key: "usuario_id", Value: other.user.ID}})
	require.NoError(t, err)
	assert.Equal(t, int64(1), n, "los de otra persona quedan")
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

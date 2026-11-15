package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/text"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
)

// extrasSetup: un dueño con un plan de una parada en `a` y lugares alrededor.
type extrasSetup struct {
	env                  *authtest.Env
	app                  *fiber.App
	owner                amigo
	a, b, c, cafe, night bson.ObjectID
}

func extrasPlace(spec placeSpec) negocios.Business {
	b := placeDoc(spec)
	b.NormalizedName = text.Normalize(b.Name)
	return b
}

func newExtras(t *testing.T) extrasSetup {
	t.Helper()
	env := negociosEnv(t)
	env.ClearRateLimits(t)
	night := make([]negocios.VentanaMin, 0, 7)
	for d := range 7 {
		night = append(night, negocios.VentanaMin{Dia: d, Desde: 20 * 60, Hasta: 23 * 60})
	}
	a := extrasPlace(placeSpec{slug: "ext-a", price: 2, avgPrice: 150})
	b := extrasPlace(placeSpec{slug: "ext-b", price: 2, avgPrice: 180, dLat: 0.002})
	c := extrasPlace(placeSpec{slug: "ext-c", price: 3, avgPrice: 300, dLat: 0.004})
	cafe := extrasPlace(placeSpec{slug: "ext-cafe", category: "cafeteria", dLat: 0.003})
	closed := extrasPlace(placeSpec{slug: "ext-noche", price: 2, dLat: 0.001, windows: night})
	_, err := env.Mongo.DB().Collection(negocios.Collection).InsertMany(context.Background(), []any{a, b, c, cafe, closed})
	require.NoError(t, err)
	return extrasSetup{env: env, app: env.App(), owner: seedAmigo(t, env, "extras_dueno"), a: a.ID, b: b.ID, c: c.ID, cafe: cafe.ID, night: closed.ID}
}

// plan crea un plan de who con una parada en negocio, en `day` a las 12:00 de México.
func (s extrasSetup) plan(t *testing.T, who amigo, day time.Time, negocio bson.ObjectID) planes.Plan {
	t.Helper()
	d := day.Format(planes.DayLayout)
	body := map[string]any{
		"nombre": "Comida", "fecha": d, "personas": 2, "presupuesto": 1000, "movilidad": "a_pie",
		"paradas": []map[string]any{{"negocio_id": negocio.Hex(), "llega": d + "T12:00:00-06:00", "minutos": 60, "precio": 100, "nota": "nota privada"}},
	}
	got := planCall(t, s.app, http.MethodPost, "/api/v1/planes", body, who.token, "")
	require.Equal(t, http.StatusCreated, got.status, got.body)
	p := got.plan(t)
	assert.Equal(t, planes.VisibilityPrivate, p.Visibilidad)
	return p
}

func nextWeek() time.Time { return time.Now().AddDate(0, 0, 7) }

func TestPlanesExtras_AlternativasYCambiarParada(t *testing.T) {
	s := newExtras(t)
	p := s.plan(t, s.owner, nextWeek(), s.a)
	stop := p.Paradas[0].ID.Hex()

	var alt planes.AlternativesResponse
	got := planCall(t, s.app, http.MethodGet, planPath(p, "/paradas/"+stop+"/alternativas"), nil, s.owner.token, "")
	require.Equal(t, http.StatusOK, got.status, got.body)
	require.NoError(t, json.Unmarshal([]byte(got.body), &alt))
	ids := []string{}
	for _, it := range alt.Items {
		ids = append(ids, it.Lugar.ID)
	}
	assert.ElementsMatch(t, []string{s.b.Hex(), s.c.Hex()}, ids, "mismo giro, precio ±1, abiertos y fuera del plan")

	byText := planCall(t, s.app, http.MethodGet, planPath(p, "/paradas/"+stop+"/alternativas?q=ext-cafe"), nil, s.owner.token, "")
	require.Equal(t, http.StatusOK, byText.status, byText.body)
	assert.Contains(t, byText.body, s.cafe.Hex(), "con texto busca por nombre aunque sea otro giro")

	// Cambiar: sube la versión y recalcula; versión vieja 409, sin If-Match 400, cerrado 422.
	changed := planCall(t, s.app, http.MethodPost, planPath(p, "/paradas/"+stop+"/cambiar"), map[string]string{"negocio_id": s.b.Hex()}, s.owner.token, `"1"`)
	require.Equal(t, http.StatusOK, changed.status, changed.body)
	cp := changed.plan(t)
	assert.Equal(t, 2, cp.Version)
	assert.Equal(t, s.b, cp.Paradas[0].NegocioID)
	assert.Equal(t, 180, cp.Paradas[0].Precio, "toma el precio promedio del negocio")
	assert.Equal(t, http.StatusConflict, planCall(t, s.app, http.MethodPost, planPath(p, "/paradas/"+stop+"/cambiar"), map[string]string{"negocio_id": s.c.Hex()}, s.owner.token, `"1"`).status)
	assert.Equal(t, http.StatusBadRequest, planCall(t, s.app, http.MethodPost, planPath(p, "/paradas/"+stop+"/cambiar"), map[string]string{"negocio_id": s.c.Hex()}, s.owner.token, "").status)
	assert.Equal(t, http.StatusUnprocessableEntity, planCall(t, s.app, http.MethodPost, planPath(p, "/paradas/"+stop+"/cambiar"), map[string]string{"negocio_id": s.night.Hex()}, s.owner.token, `"2"`).status)
	assert.Equal(t, http.StatusUnprocessableEntity, planCall(t, s.app, http.MethodPost, planPath(p, "/paradas/"+stop+"/cambiar"), map[string]string{"negocio_id": s.b.Hex()}, s.owner.token, `"2"`).status, "ya está en el plan")

	// Ajeno = 404 en las dos rutas.
	other := seedAmigo(t, s.env, "extras_ajeno")
	assert.Equal(t, http.StatusNotFound, planCall(t, s.app, http.MethodGet, planPath(p, "/paradas/"+stop+"/alternativas"), nil, other.token, "").status)
	assert.Equal(t, http.StatusNotFound, planCall(t, s.app, http.MethodPost, planPath(p, "/paradas/"+stop+"/cambiar"), map[string]string{"negocio_id": s.c.Hex()}, other.token, `"2"`).status)
}

func TestPlanesExtras_PopularesYCopiarPublico(t *testing.T) {
	s := newExtras(t)
	pub := s.plan(t, s.owner, nextWeek(), s.a)
	priv := s.plan(t, s.owner, nextWeek(), s.b)
	made := planCall(t, s.app, http.MethodPatch, planPath(pub, ""), map[string]string{"visibilidad": "publico"}, s.owner.token, `"1"`)
	require.Equal(t, http.StatusOK, made.status, made.body)

	// Populares sin sesión: solo el público, sin dueño ni notas, con caché pública.
	url := fmt.Sprintf("/api/v1/planes/populares?lat=%f&lng=%f", centerLat, centerLng)
	resp := sendJSON(t, s.app, http.MethodGet, url, nil, "")
	raw := readAll(t, resp)
	require.Equal(t, http.StatusOK, resp.StatusCode, raw)
	assert.Equal(t, "public, max-age=300", resp.Header.Get(fiber.HeaderCacheControl))
	assert.Contains(t, raw, pub.ID.Hex())
	assert.NotContains(t, raw, priv.ID.Hex())
	assert.NotContains(t, raw, s.owner.id())
	assert.NotContains(t, raw, "nota privada")

	// Otra persona lo lee sin notas y lo copia: la copia es privada y el original suma uno.
	other := seedAmigo(t, s.env, "copia_publico")
	read := planCall(t, s.app, http.MethodGet, planPath(pub, ""), nil, other.token, "")
	require.Equal(t, http.StatusOK, read.status, read.body)
	assert.NotContains(t, read.body, "nota privada")
	assert.NotContains(t, read.body, s.owner.id())
	copied := planCall(t, s.app, http.MethodPost, planPath(pub, "/clonar"), map[string]string{"fecha": nextWeek().AddDate(0, 0, 1).Format(planes.DayLayout)}, other.token, "")
	require.Equal(t, http.StatusCreated, copied.status, copied.body)
	cp := copied.plan(t)
	assert.Equal(t, planes.OriginCloned, cp.Origen)
	assert.Equal(t, planes.VisibilityPrivate, cp.Visibilidad)
	require.NotNil(t, cp.PlanOrigenID)
	assert.Equal(t, pub.ID, *cp.PlanOrigenID)
	var orig planes.Plan
	require.NoError(t, s.env.Mongo.DB().Collection(planes.Collection).FindOne(context.Background(), bson.D{{Key: "_id", Value: pub.ID}}).Decode(&orig))
	assert.Equal(t, 1, orig.VecesClonado)

	// El privado ajeno y el público con bloqueo = 404.
	assert.Equal(t, http.StatusNotFound, planCall(t, s.app, http.MethodGet, planPath(priv, ""), nil, other.token, "").status)
	blocked := seedAmigo(t, s.env, "bloqueado_publico")
	now := time.Now().UTC()
	require.NoError(t, amistades.NewRepository(s.env.Mongo.DB()).Insert(context.Background(), &amistades.Friendship{
		ID: bson.NewObjectID(), From: s.owner.user.ID, To: blocked.user.ID, Status: amistades.StatusBlocked,
		Pair: amistades.PairKey(s.owner.user.ID, blocked.user.ID), CreatedAt: now, UpdatedAt: now,
	}))
	assert.Equal(t, http.StatusNotFound, planCall(t, s.app, http.MethodGet, planPath(pub, ""), nil, blocked.token, "").status)
	assert.Equal(t, http.StatusNotFound, planCall(t, s.app, http.MethodPost, planPath(pub, "/clonar"), map[string]string{"fecha": nextWeek().Format(planes.DayLayout)}, blocked.token, "").status)
}

func TestPlanesExtras_PopularesLimitePorIP(t *testing.T) {
	s := newExtras(t)
	url := fmt.Sprintf("/api/v1/planes/populares?lat=%f&lng=%f", centerLat, centerLng)
	for range 60 {
		resp := sendJSON(t, s.app, http.MethodGet, url, nil, "")
		_ = resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	}
	resp := sendJSON(t, s.app, http.MethodGet, url, nil, "")
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	resp = sendJSON(t, s.app, http.MethodGet, "/api/v1/planes/populares", nil, "")
	_ = resp.Body.Close()
	assert.Contains(t, []int{http.StatusUnprocessableEntity, http.StatusTooManyRequests}, resp.StatusCode)
}

func TestPlanesExtras_PaqueteSinConexion(t *testing.T) {
	s := newExtras(t)
	tomorrow := s.plan(t, s.owner, time.Now().In(negocios.MexicoCity()).AddDate(0, 0, 1), s.a)
	later := s.plan(t, s.owner, nextWeek(), s.b)

	got := planCall(t, s.app, http.MethodGet, planPath(tomorrow, "/paquete-offline"), nil, s.owner.token, "")
	require.Equal(t, http.StatusOK, got.status, got.body)
	var pkg planes.PackageResponse
	require.NoError(t, json.Unmarshal([]byte(got.body), &pkg))
	assert.Equal(t, tomorrow.ID, pkg.Plan.ID)
	require.Len(t, pkg.Lugares, 1)
	assert.Equal(t, s.a.Hex(), pkg.Lugares[0].ID)
	assert.NotNil(t, pkg.Reservas)
	assert.Equal(t, `"1-`+fmt.Sprint(pkg.SyncSeq)+`"`, got.etag)

	// Con el mismo ETag = 304; fuera de ventana = 422; ajeno = 404.
	req := httptest.NewRequest(http.MethodGet, planPath(tomorrow, "/paquete-offline"), nil)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer "+s.owner.token)
	req.Header.Set(fiber.HeaderIfNoneMatch, got.etag)
	resp, err := s.app.Test(req, -1)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusNotModified, resp.StatusCode)
	assert.Equal(t, "private, no-store", resp.Header.Get(fiber.HeaderCacheControl))
	assert.Equal(t, http.StatusUnprocessableEntity, planCall(t, s.app, http.MethodGet, planPath(later, "/paquete-offline"), nil, s.owner.token, "").status)
	other := seedAmigo(t, s.env, "paquete_ajeno")
	assert.Equal(t, http.StatusNotFound, planCall(t, s.app, http.MethodGet, planPath(tomorrow, "/paquete-offline"), nil, other.token, "").status)
}

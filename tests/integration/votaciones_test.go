package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/votaciones"
)

// votacionSetup es un grupo con dos amigos (uno confirmado, otro invitado) sobre un plan de
// una parada en s.shop, y dos lugares más para votar.
type votacionSetup struct {
	gruposSetup
	groupID string
	friends []amigo
	b, c    bson.ObjectID
}

func newVotacion(t *testing.T) votacionSetup {
	t.Helper()
	s := newGrupos(t)
	b := placeDoc(placeSpec{slug: "voto-b", dLat: 0.002})
	c := placeDoc(placeSpec{slug: "voto-c", dLat: 0.003})
	_, err := s.env.Mongo.DB().Collection(negocios.Collection).InsertMany(context.Background(), []any{b, c})
	require.NoError(t, err)
	fr := s.friends(t, "vota", 2)
	g := s.create(t, fr)
	grupoCall(t, s.app, http.MethodPatch, gruposPath+"/"+g.ID+"/integrantes/yo", map[string]string{"estado": "confirmado"}, fr[0].token, http.StatusOK)
	return votacionSetup{gruposSetup: s, groupID: g.ID, friends: fr, b: b.ID, c: c.ID}
}

func votoCall(t *testing.T, app *fiber.App, method, path string, body any, token string, want int) votaciones.View {
	t.Helper()
	resp := sendJSON(t, app, method, path, body, token)
	raw := readAll(t, resp)
	require.Equal(t, want, resp.StatusCode, raw)
	var v votaciones.View
	if want == http.StatusOK || want == http.StatusCreated {
		require.NoError(t, json.Unmarshal([]byte(raw), &v), raw)
	}
	return v
}

func (s votacionSetup) open(t *testing.T, options []bson.ObjectID, want int) votaciones.View {
	t.Helper()
	ids := make([]string, 0, len(options))
	for _, o := range options {
		ids = append(ids, o.Hex())
	}
	body := map[string]any{"parada_id": s.plan.Paradas[0].ID.Hex(), "opciones": ids, "cierra_en": time.Now().Add(time.Hour).Format(time.RFC3339)}
	return votoCall(t, s.app, http.MethodPost, gruposPath+"/"+s.groupID+"/votaciones", body, s.org.token, want)
}

func votePath(v votaciones.View, rest string) string { return "/api/v1/votaciones/" + v.ID + rest }

func (s votacionSetup) planNow(t *testing.T) planes.Plan {
	t.Helper()
	got := planCall(t, s.app, http.MethodGet, planPath(s.plan, ""), nil, s.org.token, "")
	require.Equal(t, http.StatusOK, got.status, got.body)
	return got.plan(t)
}

func TestVotaciones_VotarCambiarYCerrarAplicaLaGanadora(t *testing.T) {
	s := newVotacion(t)
	v := s.open(t, []bson.ObjectID{s.shop, s.b, s.c}, http.StatusCreated)
	assert.Equal(t, votaciones.StatusOpen, v.Status)
	require.Len(t, v.Options, 3)
	require.NotNil(t, v.Options[1].Place)

	cast := func(who amigo, choice bson.ObjectID) votaciones.View {
		return votoCall(t, s.app, http.MethodPut, votePath(v, "/voto"), map[string]string{"negocio_id": choice.Hex()}, who.token, http.StatusOK)
	}
	cast(s.friends[0], s.shop)
	got := cast(s.friends[0], s.b) // cambia su voto
	assert.Equal(t, s.b.Hex(), got.MyVote)
	assert.Equal(t, 1, got.TotalVotes)
	cast(s.friends[1], s.b) // el invitado también vota
	cast(s.org, s.c)

	closed := votoCall(t, s.app, http.MethodPost, votePath(v, "/cerrar"), nil, s.org.token, http.StatusOK)
	assert.Equal(t, votaciones.StatusClosed, closed.Status)
	assert.Equal(t, s.b.Hex(), closed.Winner)
	assert.Equal(t, votaciones.ClosedByOrganizer, closed.ClosedBy)
	require.NotNil(t, closed.Applied)
	assert.True(t, *closed.Applied)

	p := s.planNow(t)
	assert.Equal(t, s.b, p.Paradas[0].NegocioID, "la ganadora entra al plan")
	assert.Equal(t, 2, p.Version)

	// Ya cerrada: votar o cerrar otra vez = 409.
	votoCall(t, s.app, http.MethodPut, votePath(v, "/voto"), map[string]string{"negocio_id": s.c.Hex()}, s.friends[0].token, http.StatusConflict)
	votoCall(t, s.app, http.MethodPost, votePath(v, "/cerrar"), nil, s.org.token, http.StatusConflict)
	listed := decode(t, sendJSON(t, s.app, http.MethodGet, gruposPath+"/"+s.groupID+"/votaciones", nil, s.friends[1].token))
	assert.Len(t, list(t, listed["items"]), 1)
}

func TestVotaciones_EmpateGanaLaPrimera(t *testing.T) {
	s := newVotacion(t)
	v := s.open(t, []bson.ObjectID{s.c, s.b}, http.StatusCreated)
	votoCall(t, s.app, http.MethodPut, votePath(v, "/voto"), map[string]string{"negocio_id": s.b.Hex()}, s.friends[0].token, http.StatusOK)
	votoCall(t, s.app, http.MethodPut, votePath(v, "/voto"), map[string]string{"negocio_id": s.c.Hex()}, s.org.token, http.StatusOK)
	closed := votoCall(t, s.app, http.MethodPost, votePath(v, "/cerrar"), nil, s.org.token, http.StatusOK)
	assert.Equal(t, s.c.Hex(), closed.Winner)
}

func TestVotaciones_PermisosYValidaciones(t *testing.T) {
	s := newVotacion(t)
	// Un integrante no abre ni cierra (403); un ajeno no ve ni vota (404).
	body := map[string]any{"parada_id": s.plan.Paradas[0].ID.Hex(), "opciones": []string{s.b.Hex(), s.c.Hex()}, "cierra_en": time.Now().Add(time.Hour).Format(time.RFC3339)}
	votoCall(t, s.app, http.MethodPost, gruposPath+"/"+s.groupID+"/votaciones", body, s.friends[0].token, http.StatusForbidden)
	v := s.open(t, []bson.ObjectID{s.b, s.c}, http.StatusCreated)
	stranger := seedAmigo(t, s.env, "ajeno_voto")
	votoCall(t, s.app, http.MethodPut, votePath(v, "/voto"), map[string]string{"negocio_id": s.b.Hex()}, stranger.token, http.StatusNotFound)
	votoCall(t, s.app, http.MethodGet, gruposPath+"/"+s.groupID+"/votaciones", nil, stranger.token, http.StatusNotFound)
	votoCall(t, s.app, http.MethodPost, votePath(v, "/cerrar"), nil, s.friends[0].token, http.StatusForbidden)

	// Quien declinó no vota; una opción ajena es 422; la misma parada no abre otra (409).
	grupoCall(t, s.app, http.MethodPatch, gruposPath+"/"+s.groupID+"/integrantes/yo", map[string]string{"estado": "declinado"}, s.friends[1].token, http.StatusOK)
	votoCall(t, s.app, http.MethodPut, votePath(v, "/voto"), map[string]string{"negocio_id": s.b.Hex()}, s.friends[1].token, http.StatusForbidden)
	votoCall(t, s.app, http.MethodPut, votePath(v, "/voto"), map[string]string{"negocio_id": s.shop.Hex()}, s.friends[0].token, http.StatusUnprocessableEntity)
	s.open(t, []bson.ObjectID{s.b, s.c}, http.StatusConflict)

	// Parada que no es del plan, lugar inexistente y hora límite fuera de rango = 422.
	bad := map[string]any{"parada_id": bson.NewObjectID().Hex(), "opciones": []string{s.b.Hex(), s.c.Hex()}, "cierra_en": time.Now().Add(time.Hour).Format(time.RFC3339)}
	votoCall(t, s.app, http.MethodPost, gruposPath+"/"+s.groupID+"/votaciones", bad, s.org.token, http.StatusUnprocessableEntity)
	bad = map[string]any{"parada_id": s.plan.Paradas[0].ID.Hex(), "opciones": []string{s.b.Hex(), bson.NewObjectID().Hex()}, "cierra_en": time.Now().Add(time.Hour).Format(time.RFC3339)}
	votoCall(t, s.app, http.MethodPost, gruposPath+"/"+s.groupID+"/votaciones", bad, s.org.token, http.StatusUnprocessableEntity)
	bad = map[string]any{"parada_id": s.plan.Paradas[0].ID.Hex(), "opciones": []string{s.b.Hex(), s.c.Hex()}, "cierra_en": time.Now().Add(time.Minute).Format(time.RFC3339)}
	votoCall(t, s.app, http.MethodPost, gruposPath+"/"+s.groupID+"/votaciones", bad, s.org.token, http.StatusUnprocessableEntity)
}

// Dos instancias cierran a la vez una votación vencida: se cierra y se aplica una sola vez.
func TestVotaciones_TrabajoCierraUnaSolaVezConDosInstancias(t *testing.T) {
	s := newVotacion(t)
	v := s.open(t, []bson.ObjectID{s.shop, s.b}, http.StatusCreated)
	votoCall(t, s.app, http.MethodPut, votePath(v, "/voto"), map[string]string{"negocio_id": s.b.Hex()}, s.friends[0].token, http.StatusOK)
	col := s.env.Mongo.DB().Collection(votaciones.Collection)
	_, err := col.UpdateOne(context.Background(), bson.D{{Key: "_id", Value: mustID(t, v.ID)}}, bson.D{{Key: "$set", Value: bson.D{{Key: "cierra_en", Value: time.Now().Add(-time.Minute)}}}})
	require.NoError(t, err)

	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.env.Votaciones.FinishDue(context.Background())
			assert.NoError(t, err)
		}()
	}
	wg.Wait()

	var got votaciones.Vote
	require.NoError(t, col.FindOne(context.Background(), bson.D{{Key: "_id", Value: mustID(t, v.ID)}}).Decode(&got))
	assert.Equal(t, votaciones.StatusClosed, got.Status)
	assert.Equal(t, votaciones.ClosedByDeadline, got.ClosedBy)
	require.NotNil(t, got.Applied)
	assert.True(t, *got.Applied)
	p := s.planNow(t)
	assert.Equal(t, s.b, p.Paradas[0].NegocioID)
	assert.Equal(t, 2, p.Version, "el plan cambió una sola vez")

	// Votar después del vencimiento = 409.
	votoCall(t, s.app, http.MethodPut, votePath(v, "/voto"), map[string]string{"negocio_id": s.shop.Hex()}, s.friends[0].token, http.StatusConflict)
}

// Si la ganadora no abre a la hora de la parada, queda constancia y el plan no cambia.
func TestVotaciones_GanadoraQueNoCabeQuedaSinAplicar(t *testing.T) {
	s := newVotacion(t)
	night := make([]negocios.VentanaMin, 0, 7)
	for d := range 7 {
		night = append(night, negocios.VentanaMin{Dia: d, Desde: 20 * 60, Hasta: 23 * 60})
	}
	late := placeDoc(placeSpec{slug: "voto-noche", dLat: 0.004, windows: night})
	_, err := s.env.Mongo.DB().Collection(negocios.Collection).InsertOne(context.Background(), late)
	require.NoError(t, err)
	v := s.open(t, []bson.ObjectID{late.ID, s.b}, http.StatusCreated)
	closed := votoCall(t, s.app, http.MethodPost, votePath(v, "/cerrar"), nil, s.org.token, http.StatusOK)
	assert.Equal(t, late.ID.Hex(), closed.Winner)
	require.NotNil(t, closed.Applied)
	assert.False(t, *closed.Applied)
	assert.NotEmpty(t, closed.Reason)
	assert.Equal(t, s.shop, s.planNow(t).Paradas[0].NegocioID)
}

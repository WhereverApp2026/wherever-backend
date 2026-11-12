package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

const amistadesPath = "/api/v1/amistades"

// amigo es una persona sembrada con su token.
type amigo struct {
	user  usuarios.User
	token string
}

func (a amigo) id() string { return a.user.ID.Hex() }

func seedAmigo(t *testing.T, env *authtest.Env, username string) amigo {
	t.Helper()
	u := seedPersona(t, env, username+"@amigos.mx", username, personaOpts{})
	return amigo{user: u, token: testutil.ValidTokenFor(t, u.ID.Hex(), "jti-"+username, middleware.RoleConsumer)}
}

func amistadesEnv(t *testing.T) (*authtest.Env, *fiber.App) {
	t.Helper()
	env := authEnv(t)
	env.ClearRateLimits(t)
	return env, env.App()
}

func amistadCall(t *testing.T, app *fiber.App, method, path string, body any, token string, want int) map[string]any {
	t.Helper()
	resp := sendJSON(t, app, method, path, body, token)
	raw := readAll(t, resp)
	require.Equal(t, want, resp.StatusCode, raw)
	if raw == "" || want == http.StatusNoContent {
		return nil
	}
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &out))
	assertSnakeCase(t, out)
	return out
}

func solicitar(t *testing.T, app *fiber.App, from, to amigo, want int) map[string]any {
	t.Helper()
	return amistadCall(t, app, http.MethodPost, amistadesPath, map[string]string{"usuario_id": to.id()}, from.token, want)
}

func responder(t *testing.T, app *fiber.App, who amigo, id, accion string, want int) map[string]any {
	t.Helper()
	return amistadCall(t, app, http.MethodPatch, amistadesPath+"/"+id, map[string]string{"accion": accion}, who.token, want)
}

// listar devuelve los nombres de usuario de la lista y el contador de pendientes.
func listar(t *testing.T, app *fiber.App, who amigo, query string) ([]string, float64) {
	t.Helper()
	body := amistadCall(t, app, http.MethodGet, amistadesPath+query, nil, who.token, http.StatusOK)
	names := []string{}
	for _, item := range list(t, body["resultados"]) {
		names = append(names, str(t, obj(t, obj(t, item)["persona"])["nombre_usuario"]))
	}
	pending, ok := body["pendientes"].(float64)
	require.True(t, ok)
	return names, pending
}

func countAmistades(t *testing.T, env *authtest.Env) int64 {
	t.Helper()
	n, err := env.Mongo.DB().Collection(amistades.Collection).CountDocuments(context.Background(), bson.D{})
	require.NoError(t, err)
	return n
}

func TestAmistades_SolicitudApareceEnAmbasListas(t *testing.T) {
	env, app := amistadesEnv(t)
	ana, beto := seedAmigo(t, env, "ana_am"), seedAmigo(t, env, "beto_am")

	out := solicitar(t, app, ana, beto, http.StatusCreated)
	assert.Equal(t, amistades.StatusPending, out["estado"])
	assert.Equal(t, amistades.DirectionSent, out["direccion"])
	assert.Equal(t, "beto_am", obj(t, out["persona"])["nombre_usuario"])
	assert.NotContains(t, obj(t, out["persona"]), "correo")

	sent, pending := listar(t, app, ana, "?estado=enviadas")
	assert.Equal(t, []string{"beto_am"}, sent)
	assert.Zero(t, pending)
	received, pending := listar(t, app, beto, "?estado=recibidas")
	assert.Equal(t, []string{"ana_am"}, received)
	assert.InDelta(t, 1, pending, 0)
	friends, pending := listar(t, app, beto, "")
	assert.Empty(t, friends, "pendiente no es amistad")
	assert.InDelta(t, 1, pending, 0, "el contador sale en cualquier lista")
}

func TestAmistades_SolicitudCruzadaQuedanAmigos(t *testing.T) {
	env, app := amistadesEnv(t)
	ana, beto := seedAmigo(t, env, "ana_cr"), seedAmigo(t, env, "beto_cr")

	solicitar(t, app, ana, beto, http.StatusCreated)
	out := solicitar(t, app, beto, ana, http.StatusCreated)
	assert.Equal(t, amistades.StatusAccepted, out["estado"])

	friendsA, _ := listar(t, app, ana, "?estado=amigos")
	friendsB, _ := listar(t, app, beto, "?estado=amigos")
	assert.Equal(t, []string{"beto_cr"}, friendsA)
	assert.Equal(t, []string{"ana_cr"}, friendsB)
	assert.Equal(t, int64(1), countAmistades(t, env))
}

func TestAmistades_AceptarYYaRespondida(t *testing.T) {
	env, app := amistadesEnv(t)
	ana, beto := seedAmigo(t, env, "ana_ac"), seedAmigo(t, env, "beto_ac")
	id := str(t, solicitar(t, app, ana, beto, http.StatusCreated)["id"])

	out := responder(t, app, beto, id, amistades.ActionAccept, http.StatusOK)
	assert.Equal(t, amistades.StatusAccepted, out["estado"])
	assert.Equal(t, amistades.DirectionReceived, out["direccion"])
	friends, _ := listar(t, app, ana, "")
	assert.Equal(t, []string{"beto_ac"}, friends)

	responder(t, app, beto, id, amistades.ActionAccept, http.StatusConflict)
	solicitar(t, app, ana, beto, http.StatusConflict)
}

func TestAmistades_RechazarSinAviso(t *testing.T) {
	env, app := amistadesEnv(t)
	ana, beto := seedAmigo(t, env, "ana_re"), seedAmigo(t, env, "beto_re")
	id := str(t, solicitar(t, app, ana, beto, http.StatusCreated)["id"])

	out := responder(t, app, beto, id, amistades.ActionReject, http.StatusOK)
	assert.Equal(t, amistades.StatusRejected, out["estado"])
	assert.Zero(t, countAmistades(t, env), "rechazar no deja rastro")

	sent, _ := listar(t, app, ana, "?estado=enviadas")
	assert.Empty(t, sent)
	friends, _ := listar(t, app, ana, "")
	assert.Empty(t, friends)
	solicitar(t, app, ana, beto, http.StatusCreated)
}

func TestAmistades_RepetidaASiMismoEInexistente(t *testing.T) {
	env, app := amistadesEnv(t)
	ana, beto := seedAmigo(t, env, "ana_rp"), seedAmigo(t, env, "beto_rp")

	solicitar(t, app, ana, beto, http.StatusCreated)
	solicitar(t, app, ana, beto, http.StatusConflict)
	solicitar(t, app, ana, ana, http.StatusUnprocessableEntity)
	amistadCall(t, app, http.MethodPost, amistadesPath, map[string]string{"usuario_id": bson.NewObjectID().Hex()}, ana.token, http.StatusNotFound)
	amistadCall(t, app, http.MethodPost, amistadesPath, map[string]string{"usuario_id": "x"}, ana.token, http.StatusUnprocessableEntity)
	assert.Equal(t, int64(1), countAmistades(t, env))
}

func TestAmistades_BloquearOcultaEImpideSolicitudes(t *testing.T) {
	env, app := amistadesEnv(t)
	ana, beto := seedAmigo(t, env, "ana_bl"), seedAmigo(t, env, "beto_bl")
	assert.Equal(t, []string{"beto_bl"}, usernamesOf(t, buscar(t, app, ana.token, "beto_bl", http.StatusOK)))
	id := str(t, solicitar(t, app, ana, beto, http.StatusCreated)["id"])

	amistadCall(t, app, http.MethodPost, amistadesPath+"/"+id+"/bloquear", nil, beto.token, http.StatusNoContent)
	amistadCall(t, app, http.MethodPost, amistadesPath+"/"+id+"/bloquear", nil, beto.token, http.StatusNoContent)

	// Quien fue bloqueado no se entera: 404 como si la persona no existiera.
	solicitar(t, app, ana, beto, http.StatusNotFound)
	solicitar(t, app, beto, ana, http.StatusConflict)
	sent, _ := listar(t, app, ana, "?estado=enviadas")
	assert.Empty(t, sent)
	assert.Empty(t, usernamesOf(t, buscar(t, app, ana.token, "beto_bl", http.StatusOK)), "quien bloqueó no aparece")
	assert.Empty(t, usernamesOf(t, buscar(t, app, beto.token, "ana_bl", http.StatusOK)))
	amistadCall(t, app, http.MethodPost, amistadesPath+"/"+id+"/bloquear", nil, ana.token, http.StatusNotFound)
	amistadCall(t, app, http.MethodDelete, amistadesPath+"/"+id, nil, ana.token, http.StatusNotFound)
}

func TestAmistades_EliminarAmigoYCancelarSolicitud(t *testing.T) {
	env, app := amistadesEnv(t)
	ana, beto, caro := seedAmigo(t, env, "ana_el"), seedAmigo(t, env, "beto_el"), seedAmigo(t, env, "caro_el")
	friendID := str(t, solicitar(t, app, ana, beto, http.StatusCreated)["id"])
	responder(t, app, beto, friendID, amistades.ActionAccept, http.StatusOK)
	pendingID := str(t, solicitar(t, app, ana, caro, http.StatusCreated)["id"])

	amistadCall(t, app, http.MethodDelete, amistadesPath+"/"+friendID, nil, beto.token, http.StatusNoContent)
	amistadCall(t, app, http.MethodDelete, amistadesPath+"/"+pendingID, nil, ana.token, http.StatusNoContent)
	amistadCall(t, app, http.MethodDelete, amistadesPath+"/"+friendID, nil, ana.token, http.StatusNotFound)

	for _, who := range []amigo{ana, beto, caro} {
		friends, pending := listar(t, app, who, "")
		assert.Empty(t, friends)
		assert.Zero(t, pending)
	}
	assert.Zero(t, countAmistades(t, env))
}

func TestAmistades_NadieTocaUnaSolicitudAjena(t *testing.T) {
	env, app := amistadesEnv(t)
	ana, beto, caro := seedAmigo(t, env, "ana_aj"), seedAmigo(t, env, "beto_aj"), seedAmigo(t, env, "caro_aj")
	id := str(t, solicitar(t, app, ana, beto, http.StatusCreated)["id"])

	responder(t, app, caro, id, amistades.ActionAccept, http.StatusNotFound)
	responder(t, app, ana, id, amistades.ActionAccept, http.StatusNotFound)
	amistadCall(t, app, http.MethodDelete, amistadesPath+"/"+id, nil, caro.token, http.StatusNotFound)
	amistadCall(t, app, http.MethodPost, amistadesPath+"/"+id+"/bloquear", nil, caro.token, http.StatusNotFound)
	responder(t, app, beto, "no-es-id", amistades.ActionAccept, http.StatusNotFound)

	received, _ := listar(t, app, beto, "?estado=recibidas")
	assert.Equal(t, []string{"ana_aj"}, received, "la solicitud sigue intacta")
}

func TestAmistades_ListaPorActividadYBusqueda(t *testing.T) {
	env, app := amistadesEnv(t)
	ana := seedAmigo(t, env, "ana_ls")
	beto, caro := seedAmigo(t, env, "beto_ls"), seedAmigo(t, env, "caro_ls")
	_, err := env.Users.Update(context.Background(), caro.user.ID, bson.D{{Key: "nombre", Value: "Carolina Núñez"}})
	require.NoError(t, err)

	for _, other := range []amigo{beto, caro} {
		id := str(t, solicitar(t, app, ana, other, http.StatusCreated)["id"])
		responder(t, app, other, id, amistades.ActionAccept, http.StatusOK)
	}
	friends, _ := listar(t, app, ana, "?estado=amigos")
	assert.Equal(t, []string{"caro_ls", "beto_ls"}, friends, "la actividad más reciente primero")
	found, _ := listar(t, app, ana, "?q=nunez")
	assert.Equal(t, []string{"caro_ls"}, found, "sin acentos ni mayúsculas")
	found, _ = listar(t, app, ana, "?q=beto")
	assert.Equal(t, []string{"beto_ls"}, found)
	amistadCall(t, app, http.MethodGet, amistadesPath+"?estado=bloqueadas", nil, ana.token, http.StatusUnprocessableEntity)
}

func TestAmistades_BorrarCuentaYExportar(t *testing.T) {
	env, app := amistadesEnv(t)
	ana, beto, caro := seedAmigo(t, env, "ana_pr"), seedAmigo(t, env, "beto_pr"), seedAmigo(t, env, "caro_pr")
	solicitar(t, app, ana, beto, http.StatusCreated)
	id := str(t, solicitar(t, app, caro, ana, http.StatusCreated)["id"])
	amistadCall(t, app, http.MethodPost, amistadesPath+"/"+id+"/bloquear", nil, caro.token, http.StatusNoContent)

	privacy := amistades.NewPrivacy(amistades.NewRepository(env.Mongo.DB()))
	exported, err := privacy.Export(context.Background(), ana.user.ID)
	require.NoError(t, err)
	raw, err := json.Marshal(exported)
	require.NoError(t, err)
	assert.Contains(t, string(raw), beto.id())
	assert.NotContains(t, string(raw), caro.id(), "el bloqueo de otra persona no se exporta")

	require.NoError(t, privacy.PurgeHistory(context.Background(), ana.user.ID))
	assert.Equal(t, int64(2), countAmistades(t, env), "borrar historial no toca amistades")
	require.NoError(t, privacy.PurgeAccount(context.Background(), ana.user.ID))
	assert.Zero(t, countAmistades(t, env))
}

// rawPost manda POST /amistades sin require: se usa desde gorutinas.
func rawPost(app *fiber.App, to, token string) (int, error) {
	body, err := json.Marshal(map[string]string{"usuario_id": to})
	if err != nil {
		return 0, err
	}
	req := httptest.NewRequest(http.MethodPost, amistadesPath, bytes.NewReader(body))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer "+token)
	resp, err := app.Test(req, 20000)
	if err != nil {
		return 0, err
	}
	return resp.StatusCode, resp.Body.Close()
}

func TestAmistades_Concurrencia_SolicitudesCruzadasUnSoloDocumento(t *testing.T) {
	env, app := amistadesEnv(t)
	for round := 0; round < 3; round++ {
		ana := seedAmigo(t, env, "ana_cc"+string(rune('a'+round)))
		beto := seedAmigo(t, env, "beto_cc"+string(rune('a'+round)))
		var wg sync.WaitGroup
		statuses := make([]int, 2)
		errs := make([]error, 2)
		for i, pair := range [][2]amigo{{ana, beto}, {beto, ana}} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				statuses[i], errs[i] = rawPost(app, pair[1].id(), pair[0].token)
			}()
		}
		wg.Wait()
		require.NoError(t, errs[0])
		require.NoError(t, errs[1])
		assert.Equal(t, []int{http.StatusCreated, http.StatusCreated}, statuses)
		n, err := env.Mongo.DB().Collection(amistades.Collection).CountDocuments(context.Background(),
			bson.D{{Key: "par", Value: amistades.PairKey(ana.user.ID, beto.user.ID)}, {Key: "estado", Value: amistades.StatusAccepted}})
		require.NoError(t, err)
		assert.Equal(t, int64(1), n, "un solo documento, ya aceptado")
	}
	assert.Equal(t, int64(3), countAmistades(t, env))
}

func TestAmistades_LimitePorHoraYSinToken(t *testing.T) {
	env, app := amistadesEnv(t)
	ana := seedAmigo(t, env, "ana_lm")
	amistadCall(t, app, http.MethodGet, amistadesPath, nil, "", http.StatusUnauthorized)
	for i := 0; i < 20; i++ {
		amistadCall(t, app, http.MethodPost, amistadesPath, map[string]string{"usuario_id": bson.NewObjectID().Hex()}, ana.token, http.StatusNotFound)
	}
	resp := sendJSON(t, app, http.MethodPost, amistadesPath, map[string]string{"usuario_id": bson.NewObjectID().Hex()}, ana.token)
	require.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.NotEmpty(t, resp.Header.Get(fiber.HeaderRetryAfter))
	amistadCall(t, app, http.MethodGet, amistadesPath, nil, ana.token, http.StatusOK)
}

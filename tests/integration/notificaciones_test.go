package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/notificaciones"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
)

const (
	notifPath   = "/api/v1/notificaciones"
	devicesPath = "/api/v1/dispositivos"
)

func notifCall(t *testing.T, app *fiber.App, method, path string, body any, token string, want int) map[string]any {
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

// kinds devuelve los tipos de aviso de who, del más nuevo al más viejo.
func kinds(t *testing.T, app *fiber.App, who amigo) []string {
	t.Helper()
	body := notifCall(t, app, http.MethodGet, notifPath+"?limite=50", nil, who.token, http.StatusOK)
	out := []string{}
	for _, item := range list(t, body["items"]) {
		out = append(out, str(t, obj(t, item)["tipo"]))
	}
	return out
}

func unread(t *testing.T, app *fiber.App, who amigo) float64 {
	t.Helper()
	body := notifCall(t, app, http.MethodGet, notifPath+"/no-leidas", nil, who.token, http.StatusOK)
	n, ok := body["total"].(float64)
	require.True(t, ok)
	return n
}

func emit(t *testing.T, env *authtest.Env, to amigo, n int) {
	t.Helper()
	svc := notificaciones.NewService(notificaciones.NewRepository(env.Mongo.DB()), nil, env.Log)
	for range n {
		svc.Emit(context.Background(), to.user.ID, notificaciones.KindPlanInvite, map[string]string{"grupo_id": bson.NewObjectID().Hex()})
		time.Sleep(2 * time.Millisecond) // creada_en distinta para el orden
	}
}

func TestNotificaciones_CentroListarMarcarYBorrar(t *testing.T) {
	env, app := amistadesEnv(t)
	me := seedAmigo(t, env, "centro")
	other := seedAmigo(t, env, "centro_otro")
	emit(t, env, me, 3)
	emit(t, env, other, 1)

	first := notifCall(t, app, http.MethodGet, notifPath+"?limite=2", nil, me.token, http.StatusOK)
	items := list(t, first["items"])
	require.Len(t, items, 2)
	next := str(t, first["siguiente_cursor"])
	require.NotEmpty(t, next)
	second := notifCall(t, app, http.MethodGet, notifPath+"?limite=2&cursor="+next, nil, me.token, http.StatusOK)
	assert.Len(t, list(t, second["items"]), 1)
	assert.Equal(t, float64(3), unread(t, app, me))

	id := str(t, obj(t, items[0])["id"])
	marked := notifCall(t, app, http.MethodPatch, notifPath+"/"+id, map[string]bool{"leida": true}, me.token, http.StatusOK)
	assert.Equal(t, true, marked["leida"])
	assert.Equal(t, float64(2), unread(t, app, me))
	notifCall(t, app, http.MethodPatch, notifPath+"/"+id, map[string]bool{"leida": false}, me.token, http.StatusOK)
	assert.Equal(t, float64(3), unread(t, app, me))

	// El aviso de otra persona no se toca (404).
	otherID := str(t, obj(t, list(t, notifCall(t, app, http.MethodGet, notifPath, nil, other.token, http.StatusOK)["items"])[0])["id"])
	notifCall(t, app, http.MethodPatch, notifPath+"/"+otherID, map[string]bool{"leida": true}, me.token, http.StatusNotFound)
	notifCall(t, app, http.MethodDelete, notifPath+"/"+otherID, nil, me.token, http.StatusNotFound)

	notifCall(t, app, http.MethodPost, notifPath+"/leer-todas", nil, me.token, http.StatusNoContent)
	assert.Zero(t, unread(t, app, me))
	assert.Equal(t, float64(1), unread(t, app, other), "leer todas no toca las de otros")
	notifCall(t, app, http.MethodDelete, notifPath+"/"+id, nil, me.token, http.StatusNoContent)
	assert.Len(t, kinds(t, app, me), 2)
}

func TestNotificaciones_EventosDeAmistadGrupoYVotacion(t *testing.T) {
	s := newVotacion(t)
	// Invitar dejó un aviso a cada invitado.
	assert.Contains(t, kinds(t, s.app, s.friends[0]), notificaciones.KindPlanInvite)
	assert.Contains(t, kinds(t, s.app, s.friends[1]), notificaciones.KindPlanInvite)

	// Abrir y cerrar una votación avisa a los integrantes, no a quien la abrió.
	v := s.open(t, []bson.ObjectID{s.shop, s.b}, http.StatusCreated)
	votoCall(t, s.app, http.MethodPost, votePath(v, "/cerrar"), nil, s.org.token, http.StatusOK)
	got := kinds(t, s.app, s.friends[0])
	assert.Contains(t, got, notificaciones.KindVoteOpened)
	assert.Contains(t, got, notificaciones.KindVoteClosed)
	assert.NotContains(t, kinds(t, s.app, s.org), notificaciones.KindVoteOpened)

	// Solicitud y aceptación de amistad.
	a := seedAmigo(t, s.env, "avisa_a")
	b := seedAmigo(t, s.env, "avisa_b")
	req := solicitar(t, s.app, a, b, http.StatusCreated)
	assert.Equal(t, []string{notificaciones.KindFriendRequest}, kinds(t, s.app, b))
	responder(t, s.app, b, str(t, req["id"]), "aceptar", http.StatusOK)
	assert.Equal(t, []string{notificaciones.KindFriendAccepted}, kinds(t, s.app, a))
}

func TestNotificaciones_Dispositivos(t *testing.T) {
	env, app := amistadesEnv(t)
	me := seedAmigo(t, env, "telefonos")
	other := seedAmigo(t, env, "telefonos_otro")
	token := func(i int) string { return strings.Repeat("t", 140) + strconv.Itoa(i) }

	for i := range 5 {
		notifCall(t, app, http.MethodPut, devicesPath, map[string]string{"token": token(i)}, me.token, http.StatusNoContent)
	}
	// Volver a registrar uno existente renueva su uso; un sexto distinto = 422.
	notifCall(t, app, http.MethodPut, devicesPath, map[string]string{"token": token(0), "plataforma": "android"}, me.token, http.StatusNoContent)
	notifCall(t, app, http.MethodPut, devicesPath, map[string]string{"token": token(5)}, me.token, http.StatusUnprocessableEntity)

	col := env.Mongo.DB().Collection(notificaciones.DevicesCollection)
	n, err := col.CountDocuments(context.Background(), bson.D{{Key: "usuario_id", Value: me.user.ID}})
	require.NoError(t, err)
	assert.Equal(t, int64(5), n)

	// El mismo teléfono inicia sesión con otra cuenta: el token pasa a esa cuenta.
	notifCall(t, app, http.MethodPut, devicesPath, map[string]string{"token": token(1)}, other.token, http.StatusNoContent)
	n, err = col.CountDocuments(context.Background(), bson.D{{Key: "usuario_id", Value: me.user.ID}})
	require.NoError(t, err)
	assert.Equal(t, int64(4), n)

	// Quitar: el propio sí, el de otra cuenta no (404).
	notifCall(t, app, http.MethodDelete, devicesPath, map[string]string{"token": token(1)}, me.token, http.StatusNotFound)
	notifCall(t, app, http.MethodDelete, devicesPath, map[string]string{"token": token(2)}, me.token, http.StatusNoContent)
}

func TestNotificaciones_ExportarYBorrarCuenta(t *testing.T) {
	env, app := amistadesEnv(t)
	me := seedAmigo(t, env, "arco_avisos")
	emit(t, env, me, 2)
	device := strings.Repeat("x", 150)
	notifCall(t, app, http.MethodPut, devicesPath, map[string]string{"token": device}, me.token, http.StatusNoContent)

	priv := notificaciones.NewPrivacy(notificaciones.NewRepository(env.Mongo.DB()))
	got, err := priv.Export(context.Background(), me.user.ID)
	require.NoError(t, err)
	raw, err := json.Marshal(got)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), device, "el token no se exporta")
	assert.Contains(t, string(raw), "avisos")

	require.NoError(t, priv.PurgeHistory(context.Background(), me.user.ID))
	assert.Zero(t, unread(t, app, me))
	require.NoError(t, priv.PurgeAccount(context.Background(), me.user.ID))
	n, err := env.Mongo.DB().Collection(notificaciones.DevicesCollection).CountDocuments(context.Background(), bson.D{{Key: "usuario_id", Value: me.user.ID}})
	require.NoError(t, err)
	assert.Zero(t, n)
}

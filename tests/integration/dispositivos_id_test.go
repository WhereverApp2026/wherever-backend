package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/notificaciones"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/fcmtest"
)

// registerWithLocation registra el teléfono y devuelve la ruta de `Location` (nv-71).
func registerWithLocation(t *testing.T, app *fiber.App, who amigo, token string) string {
	t.Helper()
	resp := sendJSON(t, app, http.MethodPut, devicesPath, map[string]string{"token": token}, who.token)
	require.Equal(t, http.StatusNoContent, resp.StatusCode, readAll(t, resp))
	loc := resp.Header.Get("Location")
	require.True(t, strings.HasPrefix(loc, devicesPath+"/"), loc)
	return loc
}

func TestDispositivosID_AjustarYBorrar(t *testing.T) {
	env, app := amistadesEnv(t)
	me := seedAmigo(t, env, "disp_id")
	other := seedAmigo(t, env, "disp_id_otro")
	loc := registerWithLocation(t, app, me, phone("id1"))
	assert.Equal(t, loc, registerWithLocation(t, app, me, phone("id1")), "registrar otra vez da el mismo id")

	got := notifCall(t, app, http.MethodPut, loc, map[string]any{"app_version": "1.4.0", "canales_activos": []string{"cupones", "grupos"}}, me.token, http.StatusOK)
	assert.Equal(t, "1.4.0", got["app_version"])
	assert.Equal(t, []any{"cupones", "grupos"}, got["canales_activos"])
	assert.Equal(t, "android", got["plataforma"])
	assert.NotContains(t, got, "token")
	assert.NotContains(t, got, "token_hash")

	resp := sendJSON(t, app, http.MethodPut, loc, map[string]any{"app_version": "1.4.0", "token": "x"}, me.token)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "campo de más")
	resp = sendJSON(t, app, http.MethodPut, loc, nil, me.token)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "cuerpo vacío")
	notifCall(t, app, http.MethodPut, loc, map[string]any{}, me.token, http.StatusUnprocessableEntity)
	notifCall(t, app, http.MethodPut, loc, map[string]any{"app_version": "1.4"}, me.token, http.StatusUnprocessableEntity)
	notifCall(t, app, http.MethodPut, loc, map[string]any{"canales_activos": []string{"grupos", "grupos"}}, me.token, http.StatusUnprocessableEntity)

	// De otra cuenta o inexistente = 404 igual; id mal formado = 422.
	notifCall(t, app, http.MethodPut, loc, map[string]any{"app_version": "2.0.0"}, other.token, http.StatusNotFound)
	notifCall(t, app, http.MethodPut, devicesPath+"/"+bson.NewObjectID().Hex(), map[string]any{"app_version": "2.0.0"}, me.token, http.StatusNotFound)
	notifCall(t, app, http.MethodDelete, loc, nil, other.token, http.StatusNotFound)
	notifCall(t, app, http.MethodDelete, devicesPath+"/no-es-id", nil, me.token, http.StatusUnprocessableEntity)

	notifCall(t, app, http.MethodDelete, loc, nil, me.token, http.StatusNoContent)
	notifCall(t, app, http.MethodDelete, loc, nil, me.token, http.StatusNotFound)

	// ARCO: la exportación lleva app_version y canales, nunca el token.
	loc = registerWithLocation(t, app, me, phone("id2"))
	notifCall(t, app, http.MethodPut, loc, map[string]any{"app_version": "1.5.0"}, me.token, http.StatusOK)
	exported, err := notificaciones.NewPrivacy(notificaciones.NewRepository(env.Mongo.DB())).Export(context.Background(), me.user.ID)
	require.NoError(t, err)
	out, err := json.Marshal(exported)
	require.NoError(t, err)
	raw := string(out)
	assert.Contains(t, raw, `"app_version":"1.5.0"`)
	assert.Contains(t, raw, `"canales_activos":["reservas","cupones","grupos","resenas","promociones"]`)
	assert.NotContains(t, raw, phone("id2"))
}

// El push de un canal apagado no llega a ese teléfono; los avisos sin canal sí.
func TestDispositivosID_CanalApagadoNoRecibePush(t *testing.T) {
	fcm := fcmtest.New(t)
	env, app := pushEnv(t, fcm.URL, fcm.Credentials)
	me := seedAmigo(t, env, "disp_canal")
	quiet := registerWithLocation(t, app, me, phone("callado"))
	registerWithLocation(t, app, me, phone("todo"))
	notifCall(t, app, http.MethodPut, quiet, map[string]any{"canales_activos": []string{}}, me.token, http.StatusOK)

	_, err := env.Users.Update(context.Background(), me.user.ID, bson.D{{Key: "preferencias.notificaciones.grupos", Value: true}})
	require.NoError(t, err)
	repo := notificaciones.NewRepository(env.Mongo.DB())
	cfg := config.FCM{ProjectID: fcmtest.Project, CredentialsB64: fcm.Credentials, BaseURL: fcm.URL}
	svc := notificaciones.NewService(repo, nil, env.Log).WithPush(notificaciones.PusherFromConfig(cfg, env.Users, repo, env.Log))
	svc.Emit(context.Background(), me.user.ID, notificaciones.KindPlanInvite, map[string]string{"grupo_id": bson.NewObjectID().Hex()})
	require.Eventually(t, func() bool { return len(fcm.Sent()) == 1 }, 10*time.Second, 50*time.Millisecond)
	time.Sleep(300 * time.Millisecond)
	require.Len(t, fcm.Sent(), 1, "el teléfono con grupos apagado no recibe")
	assert.Equal(t, phone("todo"), fcm.Sent()[0].Token)

	svc.Emit(context.Background(), me.user.ID, notificaciones.KindFriendRequest, map[string]string{"usuario_id": bson.NewObjectID().Hex()})
	require.Eventually(t, func() bool { return len(fcm.Sent()) == 3 }, 10*time.Second, 50*time.Millisecond, "sin canal llega a los dos")
}

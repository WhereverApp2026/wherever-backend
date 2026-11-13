package integration

import (
	"context"
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
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/fcmtest"
)

// pushEnv arma la app con FCM apuntando al servidor local.
func pushEnv(t *testing.T, base string, creds string) (*authtest.Env, *fiber.App) {
	t.Helper()
	cfg := testutil.Config(config.EnvProd)
	cfg.FCM = config.FCM{ProjectID: fcmtest.Project, CredentialsB64: creds, BaseURL: base}
	env := authEnvWith(t, cfg)
	env.ClearRateLimits(t)
	return env, env.App()
}

func phone(name string) string { return strings.Repeat("p", 140) + name }

func registerPhone(t *testing.T, app *fiber.App, who amigo, token string) {
	t.Helper()
	notifCall(t, app, http.MethodPut, devicesPath, map[string]string{"token": token}, who.token, http.StatusNoContent)
}

func TestPush_SolicitudLlegaAlTelefonoConTextoGenerico(t *testing.T) {
	fcm := fcmtest.New(t)
	env, app := pushEnv(t, fcm.URL, fcm.Credentials)
	a := seedAmigo(t, env, "push_a")
	b := seedAmigo(t, env, "push_b")
	registerPhone(t, app, b, phone("b1"))
	registerPhone(t, app, b, phone("b2"))

	solicitar(t, app, a, b, http.StatusCreated)
	require.Eventually(t, func() bool { return len(fcm.Sent()) == 2 }, 10*time.Second, 50*time.Millisecond, "un push por teléfono")
	for _, m := range fcm.Sent() {
		assert.Equal(t, "Wherever", m.Title)
		assert.Equal(t, "Tienes una nueva solicitud de amistad", m.Body)
		assert.Equal(t, notificaciones.KindFriendRequest, m.Data["tipo"])
		assert.NotContains(t, m.Body, "push_a", "sin nombres en el push")
	}
	assert.Equal(t, []string{notificaciones.KindFriendRequest}, kinds(t, app, b), "el centro cuenta lo mismo")
	assert.NotContains(t, env.Logs.String(), phone("b1"), "el token no llega a los logs")
}

func TestPush_TokenNoRegistradoSeBorraYReintentos(t *testing.T) {
	fcm := fcmtest.New(t)
	env, app := pushEnv(t, fcm.URL, fcm.Credentials)
	a := seedAmigo(t, env, "baja_a")
	b := seedAmigo(t, env, "baja_b")
	registerPhone(t, app, b, phone("viejo"))
	registerPhone(t, app, b, phone("lento"))
	fcm.Unregister(phone("viejo"))
	fcm.FailTimes(phone("lento"), 2) // dos 503 y luego entra

	solicitar(t, app, a, b, http.StatusCreated)
	col := env.Mongo.DB().Collection(notificaciones.DevicesCollection)
	require.Eventually(t, func() bool {
		n, err := col.CountDocuments(context.Background(), bson.D{{Key: "token_hash", Value: notificaciones.HashToken(phone("viejo"))}})
		return err == nil && n == 0
	}, 10*time.Second, 50*time.Millisecond, "el token dado de baja se borra")
	require.Eventually(t, func() bool { return len(fcm.Sent()) == 1 }, 15*time.Second, 100*time.Millisecond, "el 503 se reintenta")
	assert.Equal(t, phone("lento"), fcm.Sent()[0].Token)
}

func TestPush_HorarioSilenciosoYPreferencias(t *testing.T) {
	fcm := fcmtest.New(t)
	env, app := pushEnv(t, fcm.URL, fcm.Credentials)
	a := seedAmigo(t, env, "calla_a")
	b := seedAmigo(t, env, "calla_b")
	registerPhone(t, app, b, phone("calla"))
	_, err := env.Users.Update(context.Background(), b.user.ID, bson.D{{Key: "preferencias.horario_silencioso", Value: bson.D{{Key: "desde", Value: "00:00"}, {Key: "hasta", Value: "23:59"}}}})
	require.NoError(t, err)

	solicitar(t, app, a, b, http.StatusCreated)
	require.Eventually(t, func() bool { return len(kinds(t, app, b)) == 1 }, 5*time.Second, 50*time.Millisecond)
	time.Sleep(500 * time.Millisecond)
	assert.Empty(t, fcm.Sent(), "en horario silencioso no suena; el aviso queda en el centro")
}

func TestPush_FCMCaidoNoRompeLaAccion(t *testing.T) {
	fcm := fcmtest.New(t)
	env, app := pushEnv(t, "http://127.0.0.1:1", fcm.Credentials)
	a := seedAmigo(t, env, "caido_a")
	b := seedAmigo(t, env, "caido_b")
	registerPhone(t, app, b, phone("caido"))
	start := time.Now()
	solicitar(t, app, a, b, http.StatusCreated)
	assert.Less(t, time.Since(start), 2*time.Second, "la solicitud no espera a FCM")
	assert.Equal(t, []string{notificaciones.KindFriendRequest}, kinds(t, app, b))
}

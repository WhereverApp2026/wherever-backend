package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/auth"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/mailer"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
)

// Pruebas HTTP del módulo auth (spec nv-04 "Integration/Auth 12") con la app real en
// proceso, MongoDB y Redis reales y el buzón en memoria como única frontera simulada.

func TestAuth_LoginRefreshLogoutFlow(t *testing.T) {
	env := authEnv(t)
	env.CreateUser(t, ana)
	app := env.App()

	access, refresh := loginHTTP(t, app, ana)
	resp := sendJSON(t, app, http.MethodPost, "/api/v1/auth/refresh", map[string]string{"refresh_token": refresh}, "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body := decode(t, resp)
	assertSnakeCase(t, body)
	newAccess := str(t, body["access_token"])
	assert.NotEqual(t, access, newAccess)

	assert.Equal(t, http.StatusNoContent, sendJSON(t, app, http.MethodPost, "/api/v1/auth/logout", nil, newAccess).StatusCode)
	assert.Equal(t, http.StatusUnauthorized, sendJSON(t, app, http.MethodGet, "/api/v1/auth/sesiones", nil, newAccess).StatusCode)
	assert.Equal(t, http.StatusUnauthorized,
		sendJSON(t, app, http.MethodPost, "/api/v1/auth/refresh", map[string]string{"refresh_token": str(t, body["refresh_token"])}, "").StatusCode)
}

func TestAuth_ReusedRefreshForcesNewLogin(t *testing.T) {
	env := authEnv(t)
	env.CreateUser(t, ana)
	app := env.App()
	_, refresh := loginHTTP(t, app, ana)
	rotated := decode(t, sendJSON(t, app, http.MethodPost, "/api/v1/auth/refresh", map[string]string{"refresh_token": refresh}, ""))

	reuse := sendJSON(t, app, http.MethodPost, "/api/v1/auth/refresh", map[string]string{"refresh_token": refresh}, "")

	assert.Equal(t, http.StatusUnauthorized, reuse.StatusCode)
	assert.Equal(t, http.StatusUnauthorized, sendJSON(t, app, http.MethodGet, "/api/v1/auth/sesiones", nil, str(t, rotated["access_token"])).StatusCode)
	var session auth.Session
	require.NoError(t, env.Mongo.DB().Collection(auth.SessionsCollection).FindOne(context.Background(), bson.D{}).Decode(&session))
	assert.NotNil(t, session.RevokedAt)
	assert.NotEmpty(t, session.PreviousRefreshHash)
}

func TestAuth_ActiveSessionsMarkCurrent(t *testing.T) {
	env := authEnv(t)
	env.CreateUser(t, ana)
	app := env.App()
	phone, _ := loginHTTP(t, app, ana)
	loginHTTP(t, app, ana)

	resp := sendJSON(t, app, http.MethodGet, "/api/v1/auth/sesiones", nil, phone)

	require.Equal(t, http.StatusOK, resp.StatusCode)
	var sessions []map[string]any
	require.NoError(t, decodeInto(resp, &sessions))
	require.Len(t, sessions, 2)
	current := 0
	for _, s := range sessions {
		assertSnakeCase(t, s)
		assert.Equal(t, "Pixel 8", obj(t, s["dispositivo"])["modelo"])
		if boolean(t, s["actual"]) {
			current++
		}
	}
	assert.Equal(t, 1, current)
}

func TestAuth_RevokeSessionOnlyOwn(t *testing.T) {
	env := authEnv(t)
	env.CreateUser(t, ana)
	env.CreateUser(t, "beto@wherever.test")
	app := env.App()
	anaPhone, _ := loginHTTP(t, app, ana)
	anaTablet, _ := loginHTTP(t, app, ana)
	beto, _ := loginHTTP(t, app, "beto@wherever.test")
	tabletSID := parseClaims(t, env, anaTablet).SessionID

	assert.Equal(t, http.StatusNotFound, sendJSON(t, app, http.MethodDelete, "/api/v1/auth/sesiones/"+tabletSID, nil, beto).StatusCode)
	assert.Equal(t, http.StatusNotFound, sendJSON(t, app, http.MethodDelete, "/api/v1/auth/sesiones/no-es-id", nil, anaPhone).StatusCode)
	assert.Equal(t, http.StatusNoContent, sendJSON(t, app, http.MethodDelete, "/api/v1/auth/sesiones/"+tabletSID, nil, anaPhone).StatusCode)
	assert.Equal(t, http.StatusUnauthorized, sendJSON(t, app, http.MethodGet, "/api/v1/auth/sesiones", nil, anaTablet).StatusCode,
		"el access de la sesión cerrada se revoca de inmediato")
	assert.Equal(t, http.StatusNotFound, sendJSON(t, app, http.MethodDelete, "/api/v1/auth/sesiones/"+tabletSID, nil, anaPhone).StatusCode)
}

func TestAuth_RecoverThenReset(t *testing.T) {
	env := authEnv(t)
	env.CreateUser(t, ana)
	app := env.App()
	oldAccess, _ := loginHTTP(t, app, ana)

	assert.Equal(t, http.StatusAccepted, sendJSON(t, app, http.MethodPost, "/api/v1/auth/recuperar", map[string]string{"correo": ana}, "").StatusCode)
	msg, ok := env.Mail.Last(ana, mailer.KindPasswordReset)
	require.True(t, ok)

	weak := sendJSON(t, app, http.MethodPost, "/api/v1/auth/restablecer", map[string]any{"token": msg.Token, "password_nuevo": "corta"}, "")
	assert.Equal(t, http.StatusUnprocessableEntity, weak.StatusCode, "la política se valida antes de gastar el enlace")
	reset := sendJSON(t, app, http.MethodPost, "/api/v1/auth/restablecer",
		map[string]any{"token": msg.Token, "password_nuevo": "NuevaClave9", "dispositivo": deviceJSON}, "")
	resetBody := readAll(t, reset)
	require.Equal(t, http.StatusOK, reset.StatusCode, resetBody)
	assert.Contains(t, resetBody, `"access_token"`)

	assert.Equal(t, http.StatusUnauthorized, sendJSON(t, app, http.MethodGet, "/api/v1/auth/sesiones", nil, oldAccess).StatusCode)
	assert.Equal(t, http.StatusBadRequest,
		sendJSON(t, app, http.MethodPost, "/api/v1/auth/restablecer", map[string]any{"token": msg.Token, "password_nuevo": "OtraClave99"}, "").StatusCode)
	assert.Equal(t, http.StatusOK, sendJSON(t, app, http.MethodPost, "/api/v1/auth/login", loginBody(ana, "NuevaClave9"), "").StatusCode)
}

func TestAuth_VerifyEmail(t *testing.T) {
	env := authEnv(t)
	user := env.CreateUser(t, ana)
	app := env.App()
	access, _ := loginHTTP(t, app, ana)

	assert.Equal(t, http.StatusAccepted, sendJSON(t, app, http.MethodPost, "/api/v1/auth/reenviar-verificacion", nil, access).StatusCode)
	tooSoon := sendJSON(t, app, http.MethodPost, "/api/v1/auth/reenviar-verificacion", nil, access)
	assert.Equal(t, http.StatusTooManyRequests, tooSoon.StatusCode)
	assert.NotEmpty(t, tooSoon.Header.Get("Retry-After"))
	msg, ok := env.Mail.Last(ana, mailer.KindVerifyEmail)
	require.True(t, ok)

	resp := sendJSON(t, app, http.MethodPost, "/api/v1/auth/verificar-correo", map[string]string{"token": msg.Token}, "")

	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, true, decode(t, resp)["correo_verificado"])
	stored, err := env.Users.FindByID(context.Background(), user.ID)
	require.NoError(t, err)
	assert.True(t, stored.EmailVerified)
}

// MongoDB borra lo vencido con los índices TTL; aquí se comprueba que existan con
// expireAfterSeconds 0 y que el service ya rechace lo vencido sin esperar al barrido.
func TestAuth_TTLIndexesOnSessionsAndLinks(t *testing.T) {
	env := authEnv(t)
	for collection, name := range map[string]string{auth.SessionsCollection: "ttl_sesion", auth.TokensCollection: "ttl_token"} {
		specs, err := env.Mongo.DB().Collection(collection).Indexes().ListSpecifications(context.Background())
		require.NoError(t, err)
		found := false
		for _, spec := range specs {
			if spec.Name == name {
				found = true
				require.NotNil(t, spec.ExpireAfterSeconds)
				assert.Equal(t, int32(0), *spec.ExpireAfterSeconds)
			}
		}
		assert.True(t, found, "falta %s en %s", name, collection)
	}
}

func TestAuth_LockoutIs423WithMinutesLeft(t *testing.T) {
	env := authEnv(t)
	env.CreateUser(t, ana)
	app := env.App()
	for range auth.MaxLoginFailures - 1 {
		require.Equal(t, http.StatusUnauthorized, sendJSON(t, app, http.MethodPost, "/api/v1/auth/login", loginBody(ana, "Incorrecta1"), "").StatusCode)
	}

	resp := sendJSON(t, app, http.MethodPost, "/api/v1/auth/login", loginBody(ana, "Incorrecta1"), "")

	assert.Equal(t, http.StatusLocked, resp.StatusCode)
	assert.Contains(t, decode(t, resp)["mensaje"], "15 min")
	env.ClearRateLimits(t)
	assert.Equal(t, http.StatusLocked, sendJSON(t, app, http.MethodPost, "/api/v1/auth/login", loginBody(ana, authtest.Password), "").StatusCode)
}

// Redis caído de verdad (contenedor detenido): login y JWT fallan cerrados con 503; al volver,
// el mismo cliente se reconecta en el puerto fijo y todo sigue.
func TestAuth_RedisDownThenRecovered(t *testing.T) {
	env := authEnv(t)
	env.CreateUser(t, ana)
	app := env.App()
	access, _ := loginHTTP(t, app, ana)
	ctx := context.Background()

	require.NoError(t, stack.StopRedis(ctx))
	restarted := false
	t.Cleanup(func() {
		if !restarted {
			_ = stack.StartRedis(context.Background())
		}
	})

	t.Run("caído", func(t *testing.T) {
		assert.Equal(t, http.StatusServiceUnavailable, sendJSON(t, app, http.MethodPost, "/api/v1/auth/login", loginBody(ana, authtest.Password), "").StatusCode)
		assert.Equal(t, http.StatusServiceUnavailable, sendJSON(t, app, http.MethodGet, "/api/v1/auth/sesiones", nil, access).StatusCode)
	})

	require.NoError(t, stack.StartRedis(ctx))
	restarted = true
	require.Eventually(t, func() bool { return redisClient.Ping(ctx).Err() == nil }, 20*time.Second, 200*time.Millisecond)

	t.Run("recuperado", func(t *testing.T) {
		require.Eventually(t, func() bool {
			return sendJSON(t, app, http.MethodGet, "/api/v1/auth/sesiones", nil, access).StatusCode == http.StatusOK
		}, 10*time.Second, 500*time.Millisecond, "el limitador vuelve a Redis tras su pausa de 5 s")
		assert.Equal(t, http.StatusOK, sendJSON(t, app, http.MethodPost, "/api/v1/auth/login", loginBody(ana, authtest.Password), "").StatusCode)
	})
}

// Dos instancias con las mismas bases: el logout en una invalida el JWT en la otra.
func TestAuth_LogoutInOneInstanceRevokesInTheOther(t *testing.T) {
	env := authEnv(t)
	env.CreateUser(t, ana)
	first := env.App()
	second := authtestBuild(t, env).App()
	access, _ := loginHTTP(t, first, ana)
	require.Equal(t, http.StatusOK, sendJSON(t, second, http.MethodGet, "/api/v1/auth/sesiones", nil, access).StatusCode)

	require.Equal(t, http.StatusNoContent, sendJSON(t, first, http.MethodPost, "/api/v1/auth/logout", nil, access).StatusCode)

	assert.Equal(t, http.StatusUnauthorized, sendJSON(t, second, http.MethodGet, "/api/v1/auth/sesiones", nil, access).StatusCode)
}

func TestAuth_BadBodyIs400AndInvalidFieldsAre422(t *testing.T) {
	env := authEnv(t)
	app := env.App()

	garbage := sendJSONRaw(t, app, "/api/v1/auth/login", "{no es json")
	assert.Equal(t, http.StatusBadRequest, garbage.StatusCode)

	invalid := sendJSON(t, app, http.MethodPost, "/api/v1/auth/login", map[string]any{"correo": "no-es-correo", "password": ""}, "")
	require.Equal(t, http.StatusUnprocessableEntity, invalid.StatusCode)
	fields := map[string]bool{}
	for _, f := range list(t, decode(t, invalid)["campos"]) {
		fields[str(t, obj(t, f)["campo"])] = true
	}
	assert.True(t, fields["correo"])
	assert.True(t, fields["password"])
	assert.True(t, fields["dispositivo"])
}

func TestAuth_ChangePasswordEndpoint(t *testing.T) {
	env := authEnv(t)
	env.CreateUser(t, ana)
	app := env.App()
	current, _ := loginHTTP(t, app, ana)
	other, _ := loginHTTP(t, app, ana)

	assert.Equal(t, http.StatusUnauthorized, sendJSON(t, app, http.MethodPut, "/api/v1/auth/password",
		map[string]string{"password_actual": "Incorrecta1", "password_nuevo": "NuevaClave9"}, current).StatusCode)
	assert.Equal(t, http.StatusUnprocessableEntity, sendJSON(t, app, http.MethodPut, "/api/v1/auth/password",
		map[string]string{"password_actual": authtest.Password, "password_nuevo": "sinnumero"}, current).StatusCode)
	assert.Equal(t, http.StatusNoContent, sendJSON(t, app, http.MethodPut, "/api/v1/auth/password",
		map[string]string{"password_actual": authtest.Password, "password_nuevo": "NuevaClave9"}, current).StatusCode)

	assert.Equal(t, http.StatusOK, sendJSON(t, app, http.MethodGet, "/api/v1/auth/sesiones", nil, current).StatusCode)
	assert.Equal(t, http.StatusUnauthorized, sendJSON(t, app, http.MethodGet, "/api/v1/auth/sesiones", nil, other).StatusCode)
}

// MongoDB caído: las rutas de auth responden 503 (fail-closed), nunca 500 ni un 401 engañoso.
func TestAuth_MongoDownIs503(t *testing.T) {
	env := authEnv(t)
	env.CreateUser(t, ana)
	access, refresh := loginHTTP(t, env.App(), ana)
	dead, err := mongo.New("mongodb://" + testutil.DeadAddr + "/wherever_test")
	require.NoError(t, err)
	t.Cleanup(func() { _ = dead.Close(context.Background()) })
	app := authtest.Build(t, dead, redisClient, env.Cfg).App()

	for name, resp := range map[string]*http.Response{
		"login":     sendJSON(t, app, http.MethodPost, "/api/v1/auth/login", loginBody(ana, authtest.Password), ""),
		"refresh":   sendJSON(t, app, http.MethodPost, "/api/v1/auth/refresh", map[string]string{"refresh_token": refresh}, ""),
		"sesiones":  sendJSON(t, app, http.MethodGet, "/api/v1/auth/sesiones", nil, access),
		"recuperar": sendJSON(t, app, http.MethodPost, "/api/v1/auth/recuperar", map[string]string{"correo": ana}, ""),
		"logout":    sendJSON(t, app, http.MethodPost, "/api/v1/auth/logout", nil, access),
	} {
		assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode, name)
	}
}

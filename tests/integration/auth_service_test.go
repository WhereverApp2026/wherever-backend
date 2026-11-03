package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/auth"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/mailer"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
)

// Pruebas del service de auth (spec nv-04 "Auth service 14") con MongoDB y Redis reales:
// la constitución no permite simular la base de negocio.

const ana = "ana@wherever.test"

func kindOf(t *testing.T, err error) apperr.Kind {
	t.Helper()
	var appErr *apperr.Error
	require.True(t, errors.As(err, &appErr), "se esperaba *apperr.Error, llegó %v", err)
	return appErr.Kind
}

func login(t *testing.T, env *authtest.Env, email, password string) (auth.TokenResponse, error) {
	t.Helper()
	return env.Service.Login(context.Background(), auth.LoginInput{Email: email, Password: password, Device: device, IP: "10.0.0.1"})
}

func mustLogin(t *testing.T, env *authtest.Env, email string) auth.TokenResponse {
	t.Helper()
	res, err := login(t, env, email, authtest.Password)
	require.NoError(t, err)
	return res
}

func TestAuthService_LoginOK(t *testing.T) {
	env := authEnv(t)
	user := env.CreateUser(t, ana)

	res := mustLogin(t, env, "  ANA@wherever.test ")

	claims := parseClaims(t, env, res.AccessToken)
	assert.Equal(t, user.ID.Hex(), claims.Subject)
	assert.Equal(t, []string{middleware.RoleConsumer}, claims.Roles)
	assert.NotEmpty(t, claims.SessionID)
	assert.NotEmpty(t, res.RefreshToken)
	assert.Equal(t, ana, res.User.Email)
}

func TestAuthService_WrongPasswordIs401(t *testing.T) {
	env := authEnv(t)
	env.CreateUser(t, ana)

	_, err := login(t, env, ana, "Incorrecta1")

	assert.Equal(t, apperr.KindUnauthorized, kindOf(t, err))
}

// Correo inexistente: mismo 401, mismo mensaje y un costo comparable (hash falso de Argon2id).
func TestAuthService_UnknownEmailLooksLikeWrongPassword(t *testing.T) {
	env := authEnv(t)
	env.CreateUser(t, ana)

	start := time.Now()
	_, wrong := login(t, env, ana, "Incorrecta1")
	wrongTook := time.Since(start)
	start = time.Now()
	_, unknown := login(t, env, "nadie@wherever.test", "Incorrecta1")
	unknownTook := time.Since(start)

	assert.Equal(t, wrong.Error(), unknown.Error())
	assert.Greater(t, unknownTook, wrongTook/3, "sin el hash falso el correo inexistente respondería mucho más rápido")
}

func TestAuthService_FifthFailureLocks(t *testing.T) {
	env := authEnv(t)
	env.CreateUser(t, ana)

	for range auth.MaxLoginFailures - 1 {
		_, err := login(t, env, ana, "Incorrecta1")
		require.Equal(t, apperr.KindUnauthorized, kindOf(t, err))
	}
	_, err := login(t, env, ana, "Incorrecta1")
	assert.Equal(t, apperr.KindLocked, kindOf(t, err))

	_, err = login(t, env, ana, authtest.Password)
	assert.Equal(t, apperr.KindLocked, kindOf(t, err), "con el bloqueo ni la contraseña correcta entra")
	ttl, err := env.Redis.TTL(context.Background(), "login_fallos:"+ana).Result()
	require.NoError(t, err)
	assert.InDelta(t, auth.LockoutWindow.Seconds(), ttl.Seconds(), 5)
}

func TestAuthService_UnlocksWhenWindowExpires(t *testing.T) {
	env := authEnv(t)
	env.CreateUser(t, ana)
	for range auth.MaxLoginFailures {
		_, _ = login(t, env, ana, "Incorrecta1")
	}

	// Se acorta el TTL en vez de esperar 15 min: lo que se prueba es que el bloqueo vive en la llave.
	require.NoError(t, env.Redis.PExpire(context.Background(), "login_fallos:"+ana, time.Millisecond).Err())
	time.Sleep(20 * time.Millisecond)

	_, err := login(t, env, ana, authtest.Password)
	assert.NoError(t, err)
}

func TestAuthService_RefreshRotates(t *testing.T) {
	env := authEnv(t)
	env.CreateUser(t, ana)
	first := mustLogin(t, env, ana)

	second, err := env.Service.Refresh(context.Background(), first.RefreshToken, "10.0.0.2")

	require.NoError(t, err)
	assert.NotEqual(t, first.RefreshToken, second.RefreshToken)
	assert.Equal(t, parseClaims(t, env, first.AccessToken).SessionID, parseClaims(t, env, second.AccessToken).SessionID)
	exists, err := env.Redis.Exists(context.Background(), middleware.RevokedKey(parseClaims(t, env, first.AccessToken).ID)).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(1), exists, "el access anterior queda revocado al rotar")
}

func TestAuthService_ReusedRefreshRevokesFamily(t *testing.T) {
	env := authEnv(t)
	env.CreateUser(t, ana)
	first := mustLogin(t, env, ana)
	second, err := env.Service.Refresh(context.Background(), first.RefreshToken, "10.0.0.2")
	require.NoError(t, err)

	_, err = env.Service.Refresh(context.Background(), first.RefreshToken, "10.0.0.66")
	assert.Equal(t, apperr.KindUnauthorized, kindOf(t, err))

	_, err = env.Service.Refresh(context.Background(), second.RefreshToken, "10.0.0.2")
	assert.Equal(t, apperr.KindUnauthorized, kindOf(t, err), "el refresh legítimo también cae con la familia")
	revoked, err := env.Redis.Exists(context.Background(), middleware.RevokedKey(parseClaims(t, env, second.AccessToken).ID)).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(1), revoked)
	assert.Contains(t, env.Logs.String(), "refresh reutilizado")
}

func TestAuthService_ExpiredRefreshIs401(t *testing.T) {
	cfg := testutil.Config(config.EnvDev)
	cfg.RefreshTTL = time.Second
	env := authEnvWith(t, cfg)
	env.CreateUser(t, ana)
	res := mustLogin(t, env, ana)

	time.Sleep(1100 * time.Millisecond)
	_, err := env.Service.Refresh(context.Background(), res.RefreshToken, "10.0.0.1")

	assert.Equal(t, apperr.KindUnauthorized, kindOf(t, err))
}

func TestAuthService_LogoutRevokesJTI(t *testing.T) {
	env := authEnv(t)
	env.CreateUser(t, ana)
	res := mustLogin(t, env, ana)
	claims := parseClaims(t, env, res.AccessToken)

	require.NoError(t, env.Service.Logout(context.Background(), claims))

	ttl, err := env.Redis.TTL(context.Background(), middleware.RevokedKey(claims.ID)).Result()
	require.NoError(t, err)
	assert.Greater(t, ttl, time.Duration(0))
	assert.LessOrEqual(t, ttl, config.MaxAccessTTL)
	_, err = env.Service.Refresh(context.Background(), res.RefreshToken, "10.0.0.1")
	assert.Equal(t, apperr.KindUnauthorized, kindOf(t, err), "la sesión cerrada ya no renueva")
}

func TestAuthService_LogoutAllClosesEverySession(t *testing.T) {
	env := authEnv(t)
	env.CreateUser(t, ana)
	phone, tablet := mustLogin(t, env, ana), mustLogin(t, env, ana)

	require.NoError(t, env.Service.LogoutAll(context.Background(), parseClaims(t, env, phone.AccessToken)))

	for _, res := range []auth.TokenResponse{phone, tablet} {
		exists, err := env.Redis.Exists(context.Background(), middleware.RevokedKey(parseClaims(t, env, res.AccessToken).ID)).Result()
		require.NoError(t, err)
		assert.Equal(t, int64(1), exists)
	}
	sessions, err := env.Service.ListSessions(context.Background(), parseClaims(t, env, tablet.AccessToken))
	require.NoError(t, err)
	assert.Empty(t, sessions)
}

func TestAuthService_ChangePasswordClosesOtherSessions(t *testing.T) {
	env := authEnv(t)
	env.CreateUser(t, ana)
	current, other := mustLogin(t, env, ana), mustLogin(t, env, ana)
	claims := parseClaims(t, env, current.AccessToken)

	err := env.Service.ChangePassword(context.Background(), claims, "Incorrecta1", "NuevaClave9")
	assert.Equal(t, apperr.KindUnauthorized, kindOf(t, err))
	require.NoError(t, env.Service.ChangePassword(context.Background(), claims, authtest.Password, "NuevaClave9"))

	sessions, err := env.Service.ListSessions(context.Background(), claims)
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	assert.True(t, sessions[0].Current)
	_, err = env.Service.Refresh(context.Background(), other.RefreshToken, "10.0.0.1")
	assert.Equal(t, apperr.KindUnauthorized, kindOf(t, err))
	_, err = login(t, env, ana, "NuevaClave9")
	assert.NoError(t, err)
}

func TestAuthService_LinkIsSingleUse(t *testing.T) {
	env := authEnv(t)
	user := env.CreateUser(t, ana)
	require.NoError(t, env.Service.SendVerification(context.Background(), user))
	msg, ok := env.Mail.Last(ana, mailer.KindVerifyEmail)
	require.True(t, ok)

	require.NoError(t, env.Service.VerifyEmail(context.Background(), msg.Token))
	err := env.Service.VerifyEmail(context.Background(), msg.Token)

	assert.Equal(t, apperr.KindBadRequest, kindOf(t, err))
	stored, err := env.Users.FindByID(context.Background(), user.ID)
	require.NoError(t, err)
	assert.True(t, stored.EmailVerified)
}

func TestAuthService_ExpiredLinkIs400(t *testing.T) {
	env := authEnv(t)
	user := env.CreateUser(t, ana)
	require.NoError(t, env.Service.RequestPasswordReset(context.Background(), ana))
	msg, ok := env.Mail.Last(ana, mailer.KindPasswordReset)
	require.True(t, ok)
	env.ExpireLinks(t, user.ID)

	_, err := env.Service.ResetPassword(context.Background(), auth.ResetInput{Token: msg.Token, NewPassword: "NuevaClave9"})

	assert.Equal(t, apperr.KindBadRequest, kindOf(t, err))
}

func TestAuthService_ResendBefore60sIs429(t *testing.T) {
	env := authEnv(t)
	user := env.CreateUser(t, ana)
	claims := parseClaims(t, env, mustLogin(t, env, ana).AccessToken)
	require.NoError(t, env.Service.ResendVerification(context.Background(), claims))

	err := env.Service.ResendVerification(context.Background(), claims)

	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.KindRateLimited, appErr.Kind)
	assert.Greater(t, appErr.RetryAfter, 0)
	env.AgeLinks(t, user.ID)
	assert.NoError(t, env.Service.ResendVerification(context.Background(), claims), "pasados 60 s ya se puede")
	assert.Len(t, env.Mail.Sent(), 2)

	// La recuperación tampoco reenvía antes de 60 s, pero responde igual (sin error).
	require.NoError(t, env.Service.RequestPasswordReset(context.Background(), ana))
	require.NoError(t, env.Service.RequestPasswordReset(context.Background(), ana))
	assert.Len(t, env.Mail.Sent(), 3)
}

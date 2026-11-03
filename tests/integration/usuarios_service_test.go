package integration

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/auth"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/mailer"
	"github.com/WhereverApp2026/wherever-backend/internal/privacidad"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios/account"
)

// Pruebas del service de usuarios (spec nv-05 "Usuarios service" y "Privacidad") con MongoDB
// y Redis reales; solo se simula el correo.

const lu = "lu@wherever.test"

func registerReq(email string) account.RegisterRequest {
	return account.RegisterRequest{
		Name: "Lú", BirthDate: "1998-04-12", Email: email, Password: authtest.Password,
		AcceptTerms: true, TermsVersion: "2026-10", PromoEmail: true,
	}
}

func register(t *testing.T, env *authtest.Env, email string) (auth.TokenResponse, *middleware.Claims) {
	t.Helper()
	res, err := env.Account.Register(context.Background(), account.RegisterInput{Request: registerReq(email), IP: "10.0.0.9"})
	require.NoError(t, err)
	return res, parseClaims(t, env, res.AccessToken)
}

func stored(t *testing.T, env *authtest.Env, claims *middleware.Claims) usuarios.User {
	t.Helper()
	id, err := bson.ObjectIDFromHex(claims.Subject)
	require.NoError(t, err)
	u, err := env.Users.FindByID(context.Background(), id)
	require.NoError(t, err)
	return u
}

func TestUsuariosService_RegisterCreatesConsumerWithWelcomePoints(t *testing.T) {
	env := authEnv(t)

	res, claims := register(t, env, "  LU@wherever.test ")

	u := stored(t, env, claims)
	assert.Equal(t, lu, u.Email)
	assert.Equal(t, []string{middleware.RoleConsumer}, u.Roles)
	assert.Equal(t, usuarios.WelcomePoints, u.Points)
	assert.Equal(t, usuarios.InitialLevel, u.Level)
	assert.Equal(t, usuarios.ModeExplore, u.Mode)
	assert.Len(t, u.FriendCode, 8)
	assert.Equal(t, 1, u.Version)
	assert.Equal(t, "2026-10", u.Privacy.TermsVersion)
	assert.True(t, u.Preferences.Notifications.Bookings)
	assert.NotEmpty(t, res.RefreshToken)
	_, sent := env.Mail.Last(lu, mailer.KindVerifyEmail)
	assert.True(t, sent, "manda la verificación")
}

func TestUsuariosService_MinorIs422(t *testing.T) {
	env := authEnv(t)
	req := registerReq(lu)
	req.BirthDate = time.Now().UTC().AddDate(-18, 0, 1).Format(validation.DateLayout)

	_, err := env.Account.Register(context.Background(), account.RegisterInput{Request: req})

	assert.Equal(t, apperr.KindValidation, kindOf(t, err))
}

func TestUsuariosService_WeakPasswordAndTwoCategoriesFailValidation(t *testing.T) {
	v, err := validation.New()
	require.NoError(t, err)
	req := registerReq(lu)
	req.Password = "solo_letras"
	assert.Equal(t, apperr.KindValidation, kindOf(t, v.Struct(req)))
	assert.Equal(t, apperr.KindValidation, kindOf(t, v.Struct(account.TastesRequest{Categories: []string{"cafe", "bar"}})))
}

func TestUsuariosService_DuplicateEmailIs409(t *testing.T) {
	env := authEnv(t)
	register(t, env, lu)

	_, err := env.Account.Register(context.Background(), account.RegisterInput{Request: registerReq("Lu@Wherever.test")})

	assert.Equal(t, apperr.KindConflict, kindOf(t, err))
}

func TestUsuariosService_TermsMustBeAccepted(t *testing.T) {
	env := authEnv(t)
	req := registerReq(lu)
	req.AcceptTerms = false

	_, err := env.Account.Register(context.Background(), account.RegisterInput{Request: req})

	assert.Equal(t, apperr.KindValidation, kindOf(t, err))
}

func TestUsuariosService_TastesDoNotAddWelcomePointsTwice(t *testing.T) {
	env := authEnv(t)
	_, claims := register(t, env, lu)
	tastes := account.TastesRequest{Categories: []string{"cafe", "tacos", "museos"}, BudgetMin: 100, BudgetMax: 400}

	for range 2 {
		_, err := env.Account.SetTastes(context.Background(), claims, tastes)
		require.NoError(t, err)
	}

	u := stored(t, env, claims)
	assert.Equal(t, usuarios.WelcomePoints, u.Points)
	assert.Equal(t, []string{"cafe", "tacos", "museos"}, u.Tastes.Categories)
}

func TestUsuariosService_BookingNotificationsCannotBeTurnedOff(t *testing.T) {
	env := authEnv(t)
	_, claims := register(t, env, lu)
	off, on := false, true

	_, err := env.Account.PatchPreferences(context.Background(), claims, account.PreferencesRequest{Notifications: &account.NotificationsInput{Bookings: &off}})
	assert.Equal(t, apperr.KindValidation, kindOf(t, err))

	prefs, err := env.Account.PatchPreferences(context.Background(), claims, account.PreferencesRequest{Notifications: &account.NotificationsInput{Promotions: &on}})
	require.NoError(t, err)
	assert.True(t, prefs.Notifications.Promotions)
	assert.True(t, prefs.Notifications.Bookings)
}

func TestUsuariosService_BusinessModeNeedsMerchantRole(t *testing.T) {
	env := authEnv(t)
	_, claims := register(t, env, lu)

	_, err := env.Account.SetMode(context.Background(), claims, usuarios.ModeBusiness)

	assert.Equal(t, apperr.KindForbidden, kindOf(t, err))
}

func TestUsuariosService_PhoneIsStoredAsHMACAndUnique(t *testing.T) {
	env := authEnv(t)
	_, claims := register(t, env, lu)
	_, other := register(t, env, "otra@wherever.test")
	phone := "+5215512345678"

	require.NoError(t, env.Account.SetPhone(context.Background(), claims, phone))

	mac := hmac.New(sha256.New, []byte(testutil.PhoneKey))
	mac.Write([]byte(phone))
	u := stored(t, env, claims)
	assert.Equal(t, hex.EncodeToString(mac.Sum(nil)), u.PhoneHash)
	assert.NotContains(t, u.PhoneHash, "5512345678")
	assert.Equal(t, apperr.KindConflict, kindOf(t, env.Account.SetPhone(context.Background(), other, phone)))
}

func TestUsuariosService_StaleIfMatchIs409(t *testing.T) {
	env := authEnv(t)
	_, claims := register(t, env, lu)
	name := "Lucía"

	p, err := env.Account.PatchProfile(context.Background(), claims, 1, account.PatchProfileRequest{Name: &name})
	require.NoError(t, err)
	assert.Equal(t, 2, p.Version)

	_, err = env.Account.PatchProfile(context.Background(), claims, 1, account.PatchProfileRequest{Name: &name})
	assert.Equal(t, apperr.KindConflict, kindOf(t, err))
}

func TestUsuariosService_ChangeEmailLeavesItUnverified(t *testing.T) {
	env := authEnv(t)
	_, claims := register(t, env, lu)
	register(t, env, "ocupado@wherever.test")
	require.NoError(t, env.Users.MarkEmailVerified(context.Background(), stored(t, env, claims).ID))

	_, err := env.Account.ChangeEmail(context.Background(), claims, account.ChangeEmailRequest{NewEmail: "nuevo@wherever.test", Password: "Incorrecta1"})
	assert.Equal(t, apperr.KindUnauthorized, kindOf(t, err))
	_, err = env.Account.ChangeEmail(context.Background(), claims, account.ChangeEmailRequest{NewEmail: "ocupado@wherever.test", Password: authtest.Password})
	assert.Equal(t, apperr.KindConflict, kindOf(t, err))

	p, err := env.Account.ChangeEmail(context.Background(), claims, account.ChangeEmailRequest{NewEmail: "nuevo@wherever.test", Password: authtest.Password})
	require.NoError(t, err)
	assert.Equal(t, "nuevo@wherever.test", p.Email)
	assert.False(t, p.EmailVerified)
	_, sent := env.Mail.Last("nuevo@wherever.test", mailer.KindVerifyEmail)
	assert.True(t, sent)
}

func TestUsuariosService_DeletionNeedsELIMINARAndClosesSessions(t *testing.T) {
	env := authEnv(t)
	res, claims := register(t, env, lu)

	_, err := env.Account.RequestDeletion(context.Background(), claims, "eliminar")
	assert.Equal(t, apperr.KindValidation, kindOf(t, err))

	out, err := env.Account.RequestDeletion(context.Background(), claims, account.DeleteConfirmation)
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now().UTC().AddDate(0, 0, 30), out.DeleteAt, time.Minute)
	assert.Equal(t, usuarios.StatusPendingDeletion, stored(t, env, claims).Status)
	_, err = env.Service.Refresh(context.Background(), res.RefreshToken, "10.0.0.9")
	assert.Equal(t, apperr.KindUnauthorized, kindOf(t, err), "las sesiones quedan cerradas")
}

func TestUsuariosService_CancelDeletionOnlyWithinGracePeriod(t *testing.T) {
	env := authEnv(t)
	_, claims := register(t, env, lu)
	_, err := env.Account.CancelDeletion(context.Background(), claims)
	assert.Equal(t, apperr.KindConflict, kindOf(t, err), "sin eliminación pendiente")

	_, err = env.Account.RequestDeletion(context.Background(), claims, account.DeleteConfirmation)
	require.NoError(t, err)
	p, err := env.Account.CancelDeletion(context.Background(), claims)
	require.NoError(t, err)
	assert.Equal(t, usuarios.StatusActive, p.Status)
	assert.Nil(t, p.DeleteAt)

	_, err = env.Account.RequestDeletion(context.Background(), claims, account.DeleteConfirmation)
	require.NoError(t, err)
	_, err = env.Users.Update(context.Background(), stored(t, env, claims).ID, bson.D{{Key: "eliminar_en", Value: time.Now().UTC().Add(-time.Hour)}})
	require.NoError(t, err)
	_, err = env.Account.CancelDeletion(context.Background(), claims)
	assert.Equal(t, apperr.KindConflict, kindOf(t, err), "fuera de plazo")
}

func TestUsuariosService_FriendCodeRetriesOnCollision(t *testing.T) {
	env := authEnv(t)
	codes := []string{"AAAAAAAA", "AAAAAAAA", "BBBBBBBB"}
	var mu sync.Mutex
	repo := env.Users.WithCodeGenerator(func() (string, error) {
		mu.Lock()
		defer mu.Unlock()
		code := codes[0]
		codes = codes[1:]
		return code, nil
	})
	first := usuarios.User{Email: "a@wherever.test", Status: usuarios.StatusActive}
	second := usuarios.User{Email: "b@wherever.test", Status: usuarios.StatusActive}

	require.NoError(t, repo.Create(context.Background(), &first))
	require.NoError(t, repo.Create(context.Background(), &second))

	assert.Equal(t, "AAAAAAAA", first.FriendCode)
	assert.Equal(t, "BBBBBBBB", second.FriendCode)
}

func TestPrivacidad_NightlyDeletesOnlyDueAccounts(t *testing.T) {
	env := authEnv(t)
	_, due := register(t, env, "vence@wherever.test")
	_, notYet := register(t, env, "aun@wherever.test")
	for _, c := range []*middleware.Claims{due, notYet} {
		_, err := env.Account.RequestDeletion(context.Background(), c, account.DeleteConfirmation)
		require.NoError(t, err)
	}
	dueID := stored(t, env, due).ID
	_, err := env.Users.Update(context.Background(), dueID, bson.D{{Key: "eliminar_en", Value: time.Now().UTC().Add(-time.Minute)}})
	require.NoError(t, err)

	res, err := env.Job.RunOnce(context.Background())

	require.NoError(t, err)
	assert.True(t, res.Ran)
	assert.Equal(t, 1, res.Deleted)
	_, err = env.Users.FindByID(context.Background(), dueID)
	assert.ErrorIs(t, err, usuarios.ErrNotFound)
	n, err := env.Mongo.DB().Collection(auth.SessionsCollection).CountDocuments(context.Background(), bson.D{{Key: "usuario_id", Value: dueID}})
	require.NoError(t, err)
	assert.Zero(t, n, "sus sesiones también se borran")
	assert.Equal(t, usuarios.StatusPendingDeletion, stored(t, env, notYet).Status)
	assert.NotContains(t, env.Logs.String(), "vence@wherever.test", "el log solo lleva conteos")
}

// Dos instancias: mientras una tiene `lock:borrado_nocturno`, la otra no corre; al terminar,
// la primera suelta el candado y la otra ya puede tomarlo.
func TestPrivacidad_LockLetsOnlyOneInstanceRun(t *testing.T) {
	env := authEnv(t)
	second := authtestBuild(t, env)
	ctx := context.Background()

	require.NoError(t, env.Redis.Set(ctx, privacidad.LockKey, "token-de-la-instancia-1", time.Minute).Err())
	res, err := second.Job.RunOnce(ctx)
	require.NoError(t, err)
	assert.False(t, res.Ran, "con el candado de otra instancia no corre")
	held, err := env.Redis.Get(ctx, privacidad.LockKey).Result()
	require.NoError(t, err)
	assert.Equal(t, "token-de-la-instancia-1", held, "no suelta un candado ajeno")

	require.NoError(t, env.Redis.Del(ctx, privacidad.LockKey).Err())
	res, err = second.Job.RunOnce(ctx)
	require.NoError(t, err)
	assert.True(t, res.Ran)
	exists, err := env.Redis.Exists(ctx, privacidad.LockKey).Result()
	require.NoError(t, err)
	assert.Zero(t, exists, "al terminar suelta su candado")
}

func TestPrivacidad_ExportIsOncePerDayAndDownloadsOnce(t *testing.T) {
	env := authEnv(t)
	_, claims := register(t, env, lu)

	require.NoError(t, env.Account.RequestExport(context.Background(), claims))
	assert.Equal(t, apperr.KindRateLimited, kindOf(t, env.Account.RequestExport(context.Background(), claims)))

	res, err := env.Job.RunOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, res.Exported)
	link, ok := env.Mail.Last(lu, mailer.KindExport)
	require.True(t, ok)

	data, err := env.Account.DownloadExport(context.Background(), link.Token)
	require.NoError(t, err)
	assert.Contains(t, data, `"correo":"lu@wherever.test"`)
	assert.Contains(t, data, `"sesiones"`)
	assert.NotContains(t, data, "password_hash")
	assert.NotContains(t, data, "argon2id")
	_, err = env.Account.DownloadExport(context.Background(), link.Token)
	assert.Equal(t, apperr.KindBadRequest, kindOf(t, err), "el enlace sirve una sola vez")

	var doc privacidad.Export
	require.NoError(t, env.Mongo.DB().Collection(privacidad.ExportsCollection).FindOne(context.Background(), bson.D{}).Decode(&doc))
	assert.Equal(t, privacidad.ExportDownloaded, doc.Status)
	assert.Empty(t, doc.Data, "el archivo con datos personales se borra al descargarlo")
	assert.NotNil(t, doc.DownloadedAt)
	assert.Equal(t, apperr.KindRateLimited, kindOf(t, env.Account.RequestExport(context.Background(), claims)), "sigue contando para el límite diario")
}

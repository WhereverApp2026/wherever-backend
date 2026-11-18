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
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/googletest"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// Pruebas de POST /auth/google (spec nv-72) con el verificador real (idtoken) contra las llaves
// locales de googletest; MongoDB y Redis reales.

const googlePath = "/api/v1/auth/google"

func googleBody(token string, extra map[string]any) map[string]any {
	body := map[string]any{"id_token": token, "dispositivo": deviceJSON}
	for k, v := range extra {
		body[k] = v
	}
	return body
}

func signupData() map[string]any {
	return map[string]any{"fecha_nacimiento": "1996-03-14", "acepta_terminos": true, "version_terminos": "2026-10"}
}

func googleUsers(t *testing.T, env *authtest.Env) int64 {
	t.Helper()
	n, err := env.Mongo.DB().Collection(usuarios.Collection).CountDocuments(context.Background(), bson.D{})
	require.NoError(t, err)
	return n
}

func TestAuthGoogle_NewAccountWithSignupData201(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	token := env.Google.Token(t, googletest.Identidad{Sub: "g-nueva", Correo: "nueva@wherever.test", Nombre: "Luz Nueva", Foto: "https://lh3.googleusercontent.com/a/foto"})

	resp := sendJSON(t, app, http.MethodPost, googlePath, googleBody(token, signupData()), "")

	require.Equal(t, http.StatusCreated, resp.StatusCode)
	body := decode(t, resp)
	assertSnakeCase(t, body)
	assert.NotEmpty(t, str(t, body["access_token"]))
	user := obj(t, body["usuario"])
	assert.Equal(t, "Luz Nueva", user["nombre"])
	assert.Equal(t, true, user["correo_verificado"])
	assert.NotContains(t, user, "google_id")

	saved, err := env.Users.FindByEmail(context.Background(), "nueva@wherever.test")
	require.NoError(t, err)
	assert.Equal(t, "g-nueva", saved.GoogleID)
	assert.True(t, saved.Adult)
	require.NotNil(t, saved.BirthDate)
	assert.Empty(t, saved.PasswordHash)
	assert.NotEmpty(t, saved.FriendCode)
	assert.Equal(t, "https://lh3.googleusercontent.com/a/foto", saved.PhotoURL)
	assert.Equal(t, "2026-10", saved.Privacy.TermsVersion)
	assert.Equal(t, usuarios.WelcomePoints, saved.Points)

	var session auth.Session
	require.NoError(t, env.Mongo.DB().Collection(auth.SessionsCollection).FindOne(context.Background(), bson.D{}).Decode(&session))
	assert.Equal(t, auth.MethodGoogle, session.Method)
}

func TestAuthGoogle_NewAccountWithoutDataIs422AndSavesNothing(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	token := env.Google.Token(t, googletest.Identidad{Sub: "g-incompleta", Correo: "incompleta@wherever.test"})

	resp := sendJSON(t, app, http.MethodPost, googlePath, googleBody(token, nil), "")

	require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	body := decode(t, resp)
	assert.Equal(t, "registro_incompleto", body["mensaje"])
	fields := []string{}
	for _, f := range list(t, body["campos"]) {
		fields = append(fields, str(t, obj(t, f)["campo"]))
	}
	assert.ElementsMatch(t, []string{"fecha_nacimiento", "acepta_terminos", "version_terminos"}, fields)
	assert.Zero(t, googleUsers(t, env))
}

func TestAuthGoogle_MinorIs422(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	token := env.Google.Token(t, googletest.Identidad{Sub: "g-menor", Correo: "menor@wherever.test"})
	data := signupData()
	data["fecha_nacimiento"] = time.Now().UTC().AddDate(-17, 0, 0).Format("2006-01-02")

	resp := sendJSON(t, app, http.MethodPost, googlePath, googleBody(token, data), "")

	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	assert.Zero(t, googleUsers(t, env))
}

func TestAuthGoogle_ExistingByGoogleID200(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	token := env.Google.Token(t, googletest.Identidad{Sub: "g-repite", Correo: "repite@wherever.test"})
	require.Equal(t, http.StatusCreated, sendJSON(t, app, http.MethodPost, googlePath, googleBody(token, signupData()), "").StatusCode)

	resp := sendJSON(t, app, http.MethodPost, googlePath, googleBody(token, nil), "")

	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int64(1), googleUsers(t, env))
}

func TestAuthGoogle_LinksVerifiedEmailKeepingPassword(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	user := env.CreateUser(t, ana)
	require.NoError(t, env.Users.MarkEmailVerified(context.Background(), user.ID))
	access, _ := loginHTTP(t, app, ana)
	token := env.Google.Token(t, googletest.Identidad{Sub: "g-ana", Correo: ana})

	resp := sendJSON(t, app, http.MethodPost, googlePath, googleBody(token, nil), "")

	require.Equal(t, http.StatusOK, resp.StatusCode)
	linked, err := env.Users.FindByID(context.Background(), user.ID)
	require.NoError(t, err)
	assert.Equal(t, "g-ana", linked.GoogleID)
	assert.NotEmpty(t, linked.PasswordHash)
	assert.Equal(t, http.StatusOK, sendJSON(t, app, http.MethodGet, "/api/v1/auth/sesiones", nil, access).StatusCode)
}

func TestAuthGoogle_LinkUnverifiedEmailDropsPasswordAndSessions(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	user := env.CreateUser(t, ana) // correo sin verificar: lo pudo registrar otra persona
	access, refresh := loginHTTP(t, app, ana)
	token := env.Google.Token(t, googletest.Identidad{Sub: "g-ana", Correo: ana})

	resp := sendJSON(t, app, http.MethodPost, googlePath, googleBody(token, nil), "")

	require.Equal(t, http.StatusOK, resp.StatusCode)
	linked, err := env.Users.FindByID(context.Background(), user.ID)
	require.NoError(t, err)
	assert.Empty(t, linked.PasswordHash)
	assert.True(t, linked.EmailVerified)
	assert.Equal(t, http.StatusUnauthorized, sendJSON(t, app, http.MethodGet, "/api/v1/auth/sesiones", nil, access).StatusCode)
	assert.Equal(t, http.StatusUnauthorized, sendJSON(t, app, http.MethodPost, "/api/v1/auth/refresh", map[string]string{"refresh_token": refresh}, "").StatusCode)
	env.ClearRateLimits(t)
	assert.Equal(t, http.StatusUnauthorized, sendJSON(t, app, http.MethodPost, "/api/v1/auth/login", loginBody(ana, authtest.Password), "").StatusCode)
}

func TestAuthGoogle_EmailLinkedToOtherGoogleIDIs409(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	first := env.Google.Token(t, googletest.Identidad{Sub: "g-primera", Correo: "doble@wherever.test"})
	require.Equal(t, http.StatusCreated, sendJSON(t, app, http.MethodPost, googlePath, googleBody(first, signupData()), "").StatusCode)
	other := env.Google.Token(t, googletest.Identidad{Sub: "g-otra", Correo: "doble@wherever.test"})

	resp := sendJSON(t, app, http.MethodPost, googlePath, googleBody(other, nil), "")

	require.Equal(t, http.StatusConflict, resp.StatusCode)
	assert.Equal(t, "cuenta_vinculada", decode(t, resp)["mensaje"])
	saved, err := env.Users.FindByEmail(context.Background(), "doble@wherever.test")
	require.NoError(t, err)
	assert.Equal(t, "g-primera", saved.GoogleID)
}

func TestAuthGoogle_BlockedAccountIs401(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	user := env.CreateUser(t, ana)
	_, err := env.Users.Update(context.Background(), user.ID, bson.D{{Key: "estado", Value: usuarios.StatusBlocked}})
	require.NoError(t, err)
	token := env.Google.Token(t, googletest.Identidad{Sub: "g-ana", Correo: ana})

	resp := sendJSON(t, app, http.MethodPost, googlePath, googleBody(token, nil), "")

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	saved, err := env.Users.FindByID(context.Background(), user.ID)
	require.NoError(t, err)
	assert.Empty(t, saved.GoogleID, "una cuenta bloqueada no se vincula")
}

func TestAuthGoogle_GoogleDownIs503(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	token := env.Google.Token(t, googletest.Identidad{Sub: "g-caida", Correo: "caida@wherever.test"})
	env.Google.Caer(true)

	resp := sendJSON(t, app, http.MethodPost, googlePath, googleBody(token, signupData()), "")

	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

func TestAuthGoogle_RateLimit11thPerMinuteIs429(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	body := googleBody("no-es-un-token", nil)
	for i := 0; i < 10; i++ {
		require.Equal(t, http.StatusUnauthorized, sendJSON(t, app, http.MethodPost, googlePath, body, "").StatusCode, "petición %d", i+1)
	}

	assert.Equal(t, http.StatusTooManyRequests, sendJSON(t, app, http.MethodPost, googlePath, body, "").StatusCode)
}

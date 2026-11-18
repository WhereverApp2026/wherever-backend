package security

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/testutil/googletest"
)

// Seguridad de POST /auth/google (spec nv-72): cualquier id_token malo da el mismo 401, nada
// del token llega a los logs ni a la respuesta.

const (
	googlePath     = "/api/v1/auth/google"
	googleSub      = "109876543210987654321"
	googleCorreo   = "google.seguridad@wherever.test"
	googleRegistro = "1990-01-02"
)

func googleSignup(token string) map[string]any {
	return map[string]any{
		"id_token": token, "dispositivo": deviceJSON,
		"fecha_nacimiento": googleRegistro, "acepta_terminos": true, "version_terminos": "2026-10",
	}
}

func TestAuthGoogleSecurity_BadTokensGiveTheSame401(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	base := googletest.Identidad{Sub: googleSub, Correo: googleCorreo}
	with := func(change func(*googletest.Identidad)) googletest.Identidad {
		id := base
		change(&id)
		return id
	}
	tokens := map[string]string{
		"firma ajena":          env.Google.TokenAjeno(t, base),
		"aud ajeno":            env.Google.Token(t, with(func(id *googletest.Identidad) { id.Aud = "otra-app.apps.googleusercontent.com" })),
		"iss ajeno":            env.Google.Token(t, with(func(id *googletest.Identidad) { id.Iss = "https://evil.example.com" })),
		"vencido":              env.Google.Token(t, with(func(id *googletest.Identidad) { id.Vence = time.Now().Add(-time.Minute) })),
		"correo sin verificar": env.Google.Token(t, with(func(id *googletest.Identidad) { id.SinVerif = true })),
		"sin sub":              env.Google.Token(t, with(func(id *googletest.Identidad) { id.Sub = "" })),
		"basura":               "a.b.c",
	}
	var first map[string]any
	for name, token := range tokens {
		r := send(t, app, http.MethodPost, googlePath, googleSignup(token), "")
		require.Equal(t, http.StatusUnauthorized, r.status, name)
		body := withoutTrace(t, r.body)
		if first == nil {
			first = body
		}
		assert.Equal(t, first, body, "%s: el 401 no dice qué falló", name)
	}
	n, err := env.Users.EmailExists(t.Context(), googleCorreo)
	require.NoError(t, err)
	assert.False(t, n, "ningún token malo crea la cuenta")
}

func TestAuthGoogleSecurity_NothingFromTheTokenInLogsOrResponse(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	token := env.Google.Token(t, googletest.Identidad{Sub: googleSub, Correo: googleCorreo, Nombre: "Persona Segura"})

	created := send(t, app, http.MethodPost, googlePath, googleSignup(token), "")
	again := send(t, app, http.MethodPost, googlePath, googleSignup(token), "")
	bad := send(t, app, http.MethodPost, googlePath, googleSignup(env.Google.TokenAjeno(t, googletest.Identidad{Sub: googleSub, Correo: googleCorreo})), "")

	require.Equal(t, http.StatusCreated, created.status, created.body)
	require.Equal(t, http.StatusOK, again.status, again.body)
	require.Equal(t, http.StatusUnauthorized, bad.status)
	for _, r := range []reply{created, again} {
		assert.NotContains(t, r.body, "google_id")
		assert.NotContains(t, r.body, googleSub)
	}
	logs := env.Logs.String()
	assert.NotContains(t, logs, token)
	assert.NotContains(t, logs, googleSub)
	assert.NotContains(t, logs, googleCorreo)
}

func TestAuthGoogleSecurity_UnknownFieldIs400(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	body := googleSignup(env.Google.Token(t, googletest.Identidad{Sub: googleSub, Correo: googleCorreo}))
	body["roles"] = []string{"administrador"}

	r := send(t, app, http.MethodPost, googlePath, body, "")

	assert.Equal(t, http.StatusBadRequest, r.status)
}

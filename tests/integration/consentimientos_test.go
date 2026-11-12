package integration

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios/account"
)

// Consentimientos (spec nv-19, US-003) con MongoDB y Redis reales: aceptar, cambiar la versión
// y ver `consentimiento_vigente` falso, aceptar de nuevo y conservar el historial.

const consentPath = "/api/v1/usuarios/yo/consentimientos"

func consentBody(terms, notice string) map[string]any {
	return map[string]any{"version_terminos": terms, "version_aviso_privacidad": notice, "promociones_correo": true}
}

func TestConsentimientos_AcceptChangeVersionAndKeepHistory(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	access := registerHTTP(t, app, lu)
	terms, notice := env.Cfg.VersionApp.Terminos, env.Cfg.VersionApp.Aviso
	vigente := func() bool {
		t.Helper()
		resp := sendJSON(t, app, http.MethodGet, "/api/v1/usuarios/yo", nil, access)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		return boolean(t, obj(t, decode(t, resp)["privacidad"])["consentimiento_vigente"])
	}

	assert.False(t, vigente(), "el registro no acepta el aviso: hay que pedir el consentimiento")

	// 1. Aceptar las versiones vigentes.
	resp := sendJSON(t, app, http.MethodPost, consentPath, consentBody(terms, notice), access)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	first := decode(t, resp)
	assert.Equal(t, true, first["consentimiento_vigente"])
	assert.Equal(t, notice, first["version_aviso_privacidad"])
	assert.True(t, vigente())

	// 2. Sube la versión de los términos: el consentimiento deja de estar vigente.
	newTerms := terms + "-b"
	env.Account.WithConsents(usuarios.NewConsentRepository(env.Mongo.DB()), account.Versions{Terms: newTerms, Notice: notice})
	assert.False(t, vigente())
	stale := sendJSON(t, app, http.MethodPost, consentPath, consentBody(terms, notice), access)
	assert.Equal(t, http.StatusUnprocessableEntity, stale.StatusCode, "aceptar una versión que ya no es la vigente")

	// 3. Aceptar de nuevo.
	again := sendJSON(t, app, http.MethodPost, consentPath, consentBody(newTerms, notice), access)
	require.Equal(t, http.StatusCreated, again.StatusCode)
	assert.True(t, vigente())

	// 4. El historial conserva las dos aceptaciones; la primera no cambió.
	claims := parseClaims(t, env, access)
	id, err := bson.ObjectIDFromHex(claims.Subject)
	require.NoError(t, err)
	history, err := usuarios.NewConsentRepository(env.Mongo.DB()).History(t.Context(), id)
	require.NoError(t, err)
	require.Len(t, history, 2)
	assert.Equal(t, newTerms, history[0].TermsVersion)
	assert.Equal(t, terms, history[1].TermsVersion)
	assert.Equal(t, first["fecha_aceptacion"], history[1].AcceptedAt.UTC().Format(time.RFC3339Nano))
}

func TestConsentimientos_RequiresTokenAndValidBody(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	body := consentBody(env.Cfg.VersionApp.Terminos, env.Cfg.VersionApp.Aviso)

	assert.Equal(t, http.StatusUnauthorized, sendJSON(t, app, http.MethodPost, consentPath, body, "").StatusCode)
	access := registerHTTP(t, app, lu)
	assert.Equal(t, http.StatusUnprocessableEntity, sendJSON(t, app, http.MethodPost, consentPath, map[string]any{}, access).StatusCode)
}

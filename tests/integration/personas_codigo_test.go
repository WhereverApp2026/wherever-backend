package integration

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

const codigoPath = "/api/v1/personas/codigo/"

func porCodigo(t *testing.T, app *fiber.App, who amigo, code string, want int) map[string]any {
	t.Helper()
	return amistadCall(t, app, http.MethodGet, codigoPath+code, nil, who.token, want)
}

func TestPersonasCodigo_EncontradoConRelacion(t *testing.T) {
	env, app := amistadesEnv(t)
	ana, beto := seedAmigo(t, env, "ana_qr"), seedAmigo(t, env, "beto_qr")
	_, err := env.Users.Update(context.Background(), beto.user.ID, bson.D{{Key: "privacidad.buscable_por_usuario", Value: false}})
	require.NoError(t, err)

	got := porCodigo(t, app, ana, strings.ToLower(beto.user.FriendCode), http.StatusOK)
	assert.Equal(t, beto.id(), got["id"])
	assert.Equal(t, "beto_qr", got["nombre_usuario"])
	assert.Equal(t, "ninguna", got["relacion"], "el QR ignora buscable_por_usuario")
	assert.ElementsMatch(t, []string{"id", "nombre", "nombre_usuario", "foto_url", "relacion"}, keysOf(got))

	solicitar(t, app, ana, beto, http.StatusCreated)
	assert.Equal(t, "solicitud_enviada", porCodigo(t, app, ana, beto.user.FriendCode, http.StatusOK)["relacion"])
	assert.Equal(t, "solicitud_recibida", porCodigo(t, app, beto, ana.user.FriendCode, http.StatusOK)["relacion"])
	porCodigo(t, app, ana, "malo", http.StatusUnprocessableEntity)
}

// 404 igual para inexistente, propio, cuenta no activa y bloqueo en los dos sentidos.
func TestPersonasCodigo_404Uniforme(t *testing.T) {
	env, app := amistadesEnv(t)
	ana, beto, caro := seedAmigo(t, env, "ana_qr4"), seedAmigo(t, env, "beto_qr4"), seedAmigo(t, env, "caro_qr4")
	inactive := seedPersona(t, env, "inactiva@qr.mx", "inactiva_qr", personaOpts{status: usuarios.StatusPendingDeletion})
	blocked := seedPersona(t, env, "bloqueada@qr.mx", "bloqueada_qr", personaOpts{status: usuarios.StatusBlocked})

	id := str(t, solicitar(t, app, ana, beto, http.StatusCreated)["id"])
	amistadCall(t, app, http.MethodPost, amistadesPath+"/"+id+"/bloquear", nil, beto.token, http.StatusNoContent)

	want := porCodigo(t, app, caro, "ZZZZZZZZ", http.StatusNotFound)
	delete(want, "traza_id")
	for name, c := range map[string]struct {
		who  amigo
		code string
	}{
		"propio":              {caro, caro.user.FriendCode},
		"en eliminación":      {caro, inactive.FriendCode},
		"cuenta bloqueada":    {caro, blocked.FriendCode},
		"quien bloqueó":       {beto, ana.user.FriendCode},
		"quien fue bloqueado": {ana, beto.user.FriendCode},
	} {
		got := porCodigo(t, app, c.who, c.code, http.StatusNotFound)
		delete(got, "traza_id")
		assert.Equal(t, want, got, name)
	}
}

func TestPersonasCodigo_LimitePorHoraYSinToken(t *testing.T) {
	env, app := amistadesEnv(t)
	ana := seedAmigo(t, env, "ana_qrl")
	amistadCall(t, app, http.MethodGet, codigoPath+"ZZZZZZZZ", nil, "", http.StatusUnauthorized)
	for i := 0; i < 30; i++ {
		porCodigo(t, app, ana, "ZZZZZZZZ", http.StatusNotFound)
	}
	resp := sendJSON(t, app, http.MethodGet, codigoPath+"ZZZZZZZZ", nil, ana.token)
	require.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.NotEmpty(t, resp.Header.Get(fiber.HeaderRetryAfter))
	buscar(t, app, ana.token, "ana_qrl", http.StatusOK)
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

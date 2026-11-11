package security

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
)

// TestAsistenteSecurity_AuthYCuentaActiva: sin token o con token inválido es 401, un `sub`
// que no existe es 401 y una cuenta con eliminación pendiente es 403. Nada llega al modelo.
func TestAsistenteSecurity_AuthYCuentaActiva(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	path := "/api/v1/planes/generar"
	body := map[string]any{"frase": "tacos", "lat": 19.4326, "lng": -99.1332}

	assert.Equal(t, http.StatusUnauthorized, send(t, app, http.MethodPost, path, body, "").status)
	assert.Equal(t, http.StatusUnauthorized, send(t, app, http.MethodPost, path, body, testutil.ExpiredToken(t)).status)
	ghost := testutil.ValidTokenFor(t, "64b000000000000000000001", "jti-asistente-fantasma", middleware.RoleConsumer)
	assert.Equal(t, http.StatusUnauthorized, send(t, app, http.MethodPost, path, body, ghost).status)

	user := env.CreateUser(t, "asistente-borrado@wherever.test")
	_, err := env.Users.MarkForDeletion(context.Background(), user.ID, time.Now().UTC().Add(24*time.Hour))
	require.NoError(t, err)
	token := testutil.ValidTokenFor(t, user.ID.Hex(), "jti-asistente-borrado", middleware.RoleConsumer)
	assert.Equal(t, http.StatusForbidden, send(t, app, http.MethodPost, path, body, token).status)

	assert.Empty(t, env.IA.Peticiones())
}

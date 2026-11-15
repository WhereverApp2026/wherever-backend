package contract

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
)

func statusWith(t *testing.T, app *fiber.App, method, path, ifMatch string, body any, token string) (int, http.Header) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer "+token)
	if ifMatch != "" {
		req.Header.Set(fiber.HeaderIfMatch, ifMatch)
	}
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode, resp.Header
}

// TestContract_DispositivosYReglasPorFiltro fija las formas de nv-71: el teléfono por id y las
// reglas con `filtros`; los 204 sin cuerpo.
func TestContract_DispositivosYReglasPorFiltro(t *testing.T) {
	env := authtest.New(t, mongoClient, redisClient, testutil.Config(config.EnvProd))
	app := env.App()
	user := env.CreateUser(t, "telefono-contrato@wherever.test")
	token := testutil.ValidTokenFor(t, user.ID.Hex(), "jti-telefono-contrato", middleware.RoleConsumer)
	status, headers := statusWith(t, app, http.MethodPut, "/api/v1/dispositivos", "", map[string]string{"token": strings.Repeat("c", 150)}, token)
	require.Equal(t, http.StatusNoContent, status)
	loc := headers.Get("Location")
	status, body := call(t, app, http.MethodPut, loc, map[string]any{"app_version": "1.4.0", "canales_activos": []string{"grupos"}}, token)
	check(t, "dispositivos/por_id_200", status, body)
	status, _ = statusWith(t, app, http.MethodDelete, loc, "", nil, token)
	require.Equal(t, http.StatusNoContent, status)

	admin := env.CreateAdmin(t, "admin-filtros-contrato@wherever.test")
	adminToken := testutil.ValidTokenFor(t, admin.ID.Hex(), "jti-admin-filtros-contrato", middleware.RoleAdmin)
	status, _ = call(t, app, http.MethodPost, "/api/v1/admin/moderacion/reglas", map[string]any{"cuarentena": 40, "rechazo": 80, "palabras": []string{}}, adminToken)
	require.Equal(t, http.StatusCreated, status)
	status, body = callHeaders(t, app, http.MethodPatch, "/api/v1/admin/moderacion/reglas/spam", map[string]any{"cuarentena": 60}, adminToken, map[string]string{fiber.HeaderIfMatch: `"1"`})
	check(t, "moderacion/reglas_filtro_200", status, body)
	status, _ = statusWith(t, app, http.MethodDelete, "/api/v1/admin/moderacion/reglas/spam", `"2"`, nil, adminToken)
	require.Equal(t, http.StatusNoContent, status)
}

package integration

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
)

// Una IP real detrás de dos saltos, con una entrada distinta a la izquierda en cada intento.
func TestIPCliente_FalsificarLaCabeceraNoEvadeElLimiteDeLogin(t *testing.T) {
	cfg := testutil.Config("staging")
	cfg.TrustedProxies = []string{"0.0.0.0/0"}
	cfg.TrustedProxyHops = 2
	app := authEnvWith(t, cfg).App()
	body := `{"correo":"nadie@example.com","password":"Clave-Incorrecta-1!","dispositivo":{"modelo":"m","sistema":"s","app_version":"1"}}`

	statuses := make([]int, 0, 6)
	for i := range 6 {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("198.51.100.%d, 203.0.113.9, 130.176.0.1", i))
		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		_ = resp.Body.Close()
		statuses = append(statuses, resp.StatusCode)
	}

	assert.Equal(t, http.StatusTooManyRequests, statuses[5], "el sexto intento se limita: %v", statuses)
	exists, err := redisClient.Exists(context.Background(), "rate:login:203.0.113.9").Result()
	require.NoError(t, err)
	assert.Equal(t, int64(1), exists, "el contador es de la IP real")
	falsa, err := redisClient.Exists(context.Background(), "rate:login:198.51.100.0").Result()
	require.NoError(t, err)
	assert.Equal(t, int64(0), falsa, "la entrada falsificada no crea contador")
}

package middleware_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
)

// En app.Test la dirección de la conexión siempre es 0.0.0.0.
const socketIP = "0.0.0.0"

func clientIPApp(trusted []string, hops int) *fiber.App {
	app := fiber.New(fiber.Config{
		ProxyHeader:             fiber.HeaderXForwardedFor,
		EnableTrustedProxyCheck: true,
		TrustedProxies:          trusted,
	})
	app.Use(middleware.ProxyHeaders(hops))
	app.Get("/ip", func(c *fiber.Ctx) error { return c.SendString(middleware.ClientIP(c)) })
	return app
}

func resolvedIP(t *testing.T, app *fiber.App, headers ...string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/ip", nil)
	for _, h := range headers {
		req.Header.Add(fiber.HeaderXForwardedFor, h)
	}
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(body)
}

func TestClientIP_TomaLaEntradaALaDistanciaDeLosSaltos(t *testing.T) {
	trusted := []string{"0.0.0.0/0"}
	cases := []struct {
		name   string
		hops   int
		header string
		want   string
	}{
		{"Caddy: una entrada", 1, "203.0.113.9", "203.0.113.9"},
		{"Caddy: la izquierda es falsa", 1, "198.51.100.1, 203.0.113.9", "203.0.113.9"},
		{"CloudFront y balanceador: dos entradas", 2, "203.0.113.9, 130.176.0.1", "203.0.113.9"},
		{"CloudFront y balanceador: tres, la izquierda es falsa", 2, "198.51.100.1, 203.0.113.9, 130.176.0.1", "203.0.113.9"},
		{"IPv6", 2, "2001:db8::1, 130.176.0.1", "2001:db8::1"},
		{"sin espacios", 2, "203.0.113.9,130.176.0.1", "203.0.113.9"},
		{"menos entradas que saltos", 2, "203.0.113.9", socketIP},
		{"sin cabecera", 1, "", socketIP},
		{"entrada que no es IP", 1, "no-es-ip", socketIP},
		{"entrada vacía", 1, "203.0.113.9, ", socketIP},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, resolvedIP(t, clientIPApp(trusted, tc.hops), tc.header))
		})
	}
}

func TestClientIP_SinProxyDeConfianzaIgnoraLaCabecera(t *testing.T) {
	assert.Equal(t, socketIP, resolvedIP(t, clientIPApp(nil, 1), "203.0.113.9"))
	assert.Equal(t, socketIP, resolvedIP(t, clientIPApp([]string{"10.0.0.0/8"}, 1), "203.0.113.9"))
}

func TestClientIP_VariasCabecerasCuentanComoUnaLista(t *testing.T) {
	app := clientIPApp([]string{"0.0.0.0/0"}, 1)

	assert.Equal(t, "203.0.113.9", resolvedIP(t, app, "198.51.100.1", "203.0.113.9"),
		"la última cabecera es la del proxy; la primera la pudo poner el cliente")
}

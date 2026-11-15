package contract

import (
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
)

// TestContract_Eventos fija las formas de nv-65: 201, lista, detalle y 204.
func TestContract_Eventos(t *testing.T) {
	env := authtest.New(t, mongoClient, redisClient, testutil.Config(config.EnvProd))
	app := env.App()
	duena := env.CreateUser(t, "duena-eventos-contrato@wherever.test")
	b := env.AltaAprobada(t, duena, "Contrato Eventos")
	token := testutil.ValidTokenFor(t, duena.ID.Hex(), "jti-eventos-contrato", middleware.RoleConsumer, middleware.RoleMerchant)
	inicio := time.Now().Add(24 * time.Hour)
	body := map[string]any{
		"titulo": "Tianguis", "tipo": "tianguis", "descripcion": "Artesanías", "direccion": "Centro", "negocio_id": b.ID,
		"ubicacion": map[string]float64{"lat": 19.4326, "lng": -99.1332},
		"inicio":    inicio.Format(time.RFC3339), "fin": inicio.Add(2 * time.Hour).Format(time.RFC3339), "precio": 5000,
	}
	status, created := call(t, app, http.MethodPost, "/api/v1/eventos", body, token)
	check(t, "eventos/crear_201", status, created)
	id, _ := created.(map[string]any)["id"].(string)
	require.NotEmpty(t, id)
	status, got := call(t, app, http.MethodGet, "/api/v1/eventos?lat=19.4326&lng=-99.1332&limite=1", nil, "")
	check(t, "eventos/lista_200", status, got)
	status, got = call(t, app, http.MethodGet, "/api/v1/eventos/"+id, nil, "")
	check(t, "eventos/detalle_200", status, got)
	status, _ = statusWith(t, app, http.MethodDelete, "/api/v1/eventos/"+id, `"1"`, nil, token)
	require.Equal(t, fiber.StatusNoContent, status)
}

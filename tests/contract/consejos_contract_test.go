package contract

import (
	"net/http"
	"testing"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/reservastest"
)

// TestContract_Consejos fija las formas de nv-75: el consejo del día y la descripción con IA.
func TestContract_Consejos(t *testing.T) {
	env := authtest.New(t, mongoClient, redisClient, testutil.Config(config.EnvProd))
	app := env.App()
	duena := env.CreateUser(t, "duena-consejo-contrato@wherever.test")
	b := reservastest.Negocio(t, env.Mongo.DB(), duena.ID, reservastest.Config{})
	token := testutil.ValidTokenFor(t, duena.ID.Hex(), "jti-duena-consejo-contrato", middleware.RoleMerchant)
	env.IA.Responder(`{"texto":"Completa tu perfil con fotos y una buena descripción para atraer más visitas.","accion":"completar_perfil"}`)

	status, body := call(t, app, http.MethodGet, "/api/v1/negocios/"+b.ID.Hex()+"/consejo", nil, token)
	check(t, "consejos/consejo_200", status, body)
	env.IA.Responder(`{"descripcion":"Restaurante familiar con tacos al pastor, salsas de la casa y cerveza fría para compartir."}`)
	status, body = call(t, app, http.MethodPost, "/api/v1/negocios/"+b.ID.Hex()+"/ia/descripcion", map[string]any{"palabras": []string{"tacos", "salsas", "cerveza"}}, token)
	check(t, "consejos/descripcion_200", status, body)
}

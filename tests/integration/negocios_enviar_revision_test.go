package integration

import (
	"context"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios/alta"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
)

// Integration/Alta: POST /negocios/{id}/enviar-revision por HTTP (nv-55). Caso feliz: 200 con
// el negocio en revisión; error principal: 422 con la lista de lo que falta.
func TestNegociosAlta_EnviarRevisionPorHTTP(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	u := env.CreateUser(t, "duena-envio-http@wherever.test")
	token := testutil.ValidTokenFor(t, u.ID.Hex(), "jti-envio-http", middleware.RoleConsumer, middleware.RoleMerchant)

	borrador, err := env.Alta.Crear(context.Background(), authtest.Comerciante(u), alta.CrearRequest{Nombre: "Taquería Incompleta", Categoria: "restaurante"})
	require.NoError(t, err)
	resp := sendJSON(t, app, http.MethodPost, altaPath+"/"+borrador.ID+"/enviar-revision", nil, token)
	requireStatus(t, resp, http.StatusUnprocessableEntity)
	campos := map[string]bool{}
	for _, f := range list(t, decode(t, resp)["campos"]) {
		campos[str(t, obj(t, f)["campo"])] = true
	}
	for _, falta := range []string{"ubicacion", "horarios", "fotos", "precios", "consentimiento"} {
		assert.True(t, campos[falta], "el 422 dice que falta %s", falta)
	}

	completo := env.AltaCompleta(t, u, "Taquería Completa")
	resp = sendJSON(t, app, http.MethodPost, altaPath+"/"+completo.ID+"/enviar-revision", nil, token)
	requireStatus(t, resp, http.StatusOK)
	assert.Equal(t, "no-store", resp.Header.Get(fiber.HeaderCacheControl))
	body := decode(t, resp)
	assertSnakeCase(t, body)
	assert.Equal(t, negocios.StatusReview, body["estado"])
	assert.NotEmpty(t, obj(t, body["revision"])["enviado_en"])
}

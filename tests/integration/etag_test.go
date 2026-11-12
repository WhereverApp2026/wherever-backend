package integration

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const autocompletarGuada = "/api/v1/ciudades/autocompletar?q=guada"

// nv-17: el catálogo público responde ETag débil y frescura de 60 s.
func TestETag_AutocompletarConETag(t *testing.T) {
	env := authEnv(t)
	importCities(t, env)
	app := env.App()

	resp := sendJSON(t, app, http.MethodGet, autocompletarGuada, nil, "")

	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.True(t, strings.HasPrefix(resp.Header.Get(fiber.HeaderETag), `W/"`))
	assert.Equal(t, "public, max-age=60", resp.Header.Get(fiber.HeaderCacheControl))
}

func TestETag_AutocompletarSinCambios304(t *testing.T) {
	env := authEnv(t)
	importCities(t, env)
	app := env.App()
	tag := sendJSON(t, app, http.MethodGet, autocompletarGuada, nil, "").Header.Get(fiber.HeaderETag)
	require.NotEmpty(t, tag)

	resp := sendWithHeaders(t, app, http.MethodGet, autocompletarGuada, nil, "", map[string]string{fiber.HeaderIfNoneMatch: tag})

	require.Equal(t, http.StatusNotModified, resp.StatusCode)
	assert.Empty(t, readAll(t, resp), "el 304 no lleva cuerpo")
	assert.Equal(t, tag, resp.Header.Get(fiber.HeaderETag))
	assert.Equal(t, "public, max-age=60", resp.Header.Get(fiber.HeaderCacheControl))
}

func TestETag_AutocompletarCambiaConElCatalogo(t *testing.T) {
	env := authEnv(t)
	importCities(t, env)
	app := env.App()
	tag := sendJSON(t, app, http.MethodGet, autocompletarGuada, nil, "").Header.Get(fiber.HeaderETag)
	_, err := env.Cities.Import(context.Background(), strings.NewReader("nombre,estado,latitud,longitud,con_cobertura\n"+
		"Guadalupe,Nuevo León,25.6775,-100.2597,true\n"), "csv")
	require.NoError(t, err)

	resp := sendWithHeaders(t, app, http.MethodGet, autocompletarGuada, nil, "", map[string]string{fiber.HeaderIfNoneMatch: tag})

	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NotEqual(t, tag, resp.Header.Get(fiber.HeaderETag))
}

// Los datos del usuario nunca se cachean ni responden 304.
func TestETag_DatosDelUsuarioNoStore(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	access := registerHTTP(t, app, "etag@wherever.test")

	profile := sendJSON(t, app, http.MethodGet, "/api/v1/usuarios/yo", nil, access)
	require.Equal(t, http.StatusOK, profile.StatusCode)
	assert.Equal(t, "no-store", profile.Header.Get(fiber.HeaderCacheControl))
	again := sendWithHeaders(t, app, http.MethodGet, "/api/v1/usuarios/yo", nil, access,
		map[string]string{fiber.HeaderIfNoneMatch: profile.Header.Get(fiber.HeaderETag)})
	assert.Equal(t, http.StatusOK, again.StatusCode, "el perfil no responde 304")

	sessions := sendWithHeaders(t, app, http.MethodGet, "/api/v1/auth/sesiones", nil, access, map[string]string{fiber.HeaderIfNoneMatch: "*"})
	require.Equal(t, http.StatusOK, sessions.StatusCode)
	assert.Equal(t, "no-store", sessions.Header.Get(fiber.HeaderCacheControl))
}

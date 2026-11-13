package security

import (
	"bytes"
	"encoding/json"
	"image"
	"image/jpeg"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
)

// Las tres rutas de nv-45 piden sesión; una imagen ajena no se confirma ni se borra (404, sin
// revelar que existe) y no se gasta moderación en ella.
func TestImagenesSecurity_SesionYPropiedad(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	id := "0123456789abcdef0123456789abcdef"
	for _, r := range []struct{ method, path string }{
		{http.MethodPost, "/api/v1/imagenes/subida"},
		{http.MethodPost, "/api/v1/imagenes/" + id + "/confirmar"},
		{http.MethodDelete, "/api/v1/imagenes/" + id},
	} {
		assert.Equal(t, http.StatusUnauthorized, send(t, app, r.method, r.path, nil, "").status, r.method+" "+r.path)
	}

	dueno := env.CreateUser(t, "dueno-foto@wherever.test")
	ajeno := env.CreateUser(t, "ajeno-foto@wherever.test")
	tokDueno := testutil.ValidTokenFor(t, dueno.ID.Hex(), "jti-dueno-foto", middleware.RoleConsumer)
	tokAjeno := testutil.ValidTokenFor(t, ajeno.ID.Hex(), "jti-ajeno-foto", middleware.RoleConsumer)

	r := send(t, app, http.MethodPost, "/api/v1/imagenes/subida", map[string]string{"tipo": "image/jpeg"}, tokDueno)
	require.Equal(t, http.StatusCreated, r.status, r.body)
	var sub struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal([]byte(r.body), &sub))
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 8, 8)), nil))
	env.Imagenes.Almacen.Subir(sub.ID, "image/jpeg", buf.Bytes())

	assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodPost, "/api/v1/imagenes/"+sub.ID+"/confirmar", nil, tokAjeno).status)
	assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodDelete, "/api/v1/imagenes/"+sub.ID, nil, tokAjeno).status)
	assert.Empty(t, env.Imagenes.Rekognition.Peticiones(), "lo ajeno no llega a moderación")
	_, sigue := env.Imagenes.Almacen.Objeto("cuarentena/" + sub.ID)
	assert.True(t, sigue, "el ajeno no borró el original")

	assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodPost, "/api/v1/imagenes/no-es-un-id/confirmar", nil, tokDueno).status, "id mal formado")
	assert.Equal(t, http.StatusOK, send(t, app, http.MethodPost, "/api/v1/imagenes/"+sub.ID+"/confirmar", nil, tokDueno).status)
}

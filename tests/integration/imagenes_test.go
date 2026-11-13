package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	httpapi "github.com/WhereverApp2026/wherever-backend/internal/http"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/imagenes"
	"github.com/WhereverApp2026/wherever-backend/internal/moderacion"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/imagenestest"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/rekognitiontest"
)

const imagenesPath = "/api/v1/imagenes"

func imgCall(t *testing.T, app *fiber.App, method, path string, body any, token string, want int) map[string]any {
	t.Helper()
	resp := sendJSON(t, app, method, path, body, token)
	raw := readAll(t, resp)
	require.Equal(t, want, resp.StatusCode, raw)
	if raw == "" || want == http.StatusNoContent {
		return nil
	}
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &out))
	assertSnakeCase(t, out)
	return out
}

func fotoJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 120, A: 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, nil))
	return buf.Bytes()
}

// subirFoto pide el formulario y hace lo que haría la app: dejar el archivo en cuarentena.
func subirFoto(t *testing.T, env *authtest.Env, app *fiber.App, token, tipo string, datos []byte) string {
	t.Helper()
	body := imgCall(t, app, http.MethodPost, imagenesPath+"/subida", map[string]string{"tipo": tipo}, token, http.StatusCreated)
	id := str(t, body["id"])
	env.Imagenes.Almacen.Subir(id, tipo, datos)
	return id
}

func imagenesEnv(t *testing.T) (*authtest.Env, *fiber.App, amigo) {
	t.Helper()
	env, app := amistadesEnv(t)
	return env, app, seedAmigo(t, env, "fotografa")
}

func TestImagenes_SubidaModeracionYPublicacion(t *testing.T) {
	env, app, yo := imagenesEnv(t)

	body := imgCall(t, app, http.MethodPost, imagenesPath+"/subida", map[string]string{"tipo": "image/jpeg"}, yo.token, http.StatusCreated)
	id := str(t, body["id"])
	assert.Len(t, id, 32)
	assert.Equal(t, float64(5<<20), body["max_bytes"])
	firmas := env.Imagenes.Almacen.Firmas()
	require.Len(t, firmas, 1)
	assert.Equal(t, imagenestest.Firma{Clave: "cuarentena/" + id, Tipo: "image/jpeg", MaxBytes: 5 << 20, Vence: 5 * time.Minute}, firmas[0])

	env.Imagenes.Almacen.Subir(id, "image/jpeg", fotoJPEG(t, 2000, 1500))
	out := imgCall(t, app, http.MethodPost, imagenesPath+"/"+id+"/confirmar", nil, yo.token, http.StatusOK)
	assert.Equal(t, imagenes.EstadoPublicada, out["estado"])
	assert.Equal(t, imagenestest.URLPublica+"/i/"+id+"/1080.jpg", out["url_1080"])
	assert.Equal(t, imagenestest.URLPublica+"/i/"+id+"/320.jpg", out["url_320"])
	assert.Equal(t, []string{"i/" + id + "/1080.jpg", "i/" + id + "/320.jpg"}, env.Imagenes.Almacen.Claves(), "el original sale de cuarentena")
	grande, _ := env.Imagenes.Almacen.Objeto("i/" + id + "/1080.jpg")
	assert.Equal(t, imagenes.CacheInmutable, grande.Cache)
	assert.Equal(t, "image/jpeg", grande.Tipo)

	// Se modera una vez por original: confirmar de nuevo es 409 y no llama a Rekognition.
	imgCall(t, app, http.MethodPost, imagenesPath+"/"+id+"/confirmar", nil, yo.token, http.StatusConflict)
	peticiones := env.Imagenes.Rekognition.Peticiones()
	require.Len(t, peticiones, 1)
	assert.True(t, peticiones[0].Firmada)
	cfg, _, err := image.DecodeConfig(bytes.NewReader(peticiones[0].Bytes))
	require.NoError(t, err)
	assert.Equal(t, 1080, cfg.Width, "a Rekognition va el JPEG ya recodificado, sin EXIF")

	imgCall(t, app, http.MethodDelete, imagenesPath+"/"+id, nil, yo.token, http.StatusNoContent)
	assert.Empty(t, env.Imagenes.Almacen.Claves())
	imgCall(t, app, http.MethodDelete, imagenesPath+"/"+id, nil, yo.token, http.StatusNotFound)
}

func TestImagenes_ConfirmarSinSubirEs409(t *testing.T) {
	env, app, yo := imagenesEnv(t)
	body := imgCall(t, app, http.MethodPost, imagenesPath+"/subida", map[string]string{"tipo": "image/png"}, yo.token, http.StatusCreated)
	id := str(t, body["id"])
	imgCall(t, app, http.MethodPost, imagenesPath+"/"+id+"/confirmar", nil, yo.token, http.StatusConflict)

	// Sigue pendiente: al subirla ya se confirma.
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 30, 30))))
	env.Imagenes.Almacen.Subir(id, "image/png", buf.Bytes())
	out := imgCall(t, app, http.MethodPost, imagenesPath+"/"+id+"/confirmar", nil, yo.token, http.StatusOK)
	assert.Equal(t, imagenes.EstadoPublicada, out["estado"])
	imgCall(t, app, http.MethodPost, imagenesPath+"/0123456789abcdef0123456789abcdef/confirmar", nil, yo.token, http.StatusNotFound)
}

func TestImagenes_TipoEquivocadoOArchivoInvalidoEs422(t *testing.T) {
	env, app, yo := imagenesEnv(t)
	imgCall(t, app, http.MethodPost, imagenesPath+"/subida", map[string]string{"tipo": "image/gif"}, yo.token, http.StatusUnprocessableEntity)

	id := subirFoto(t, env, app, yo.token, "image/png", fotoJPEG(t, 50, 50)) // dice PNG y es JPEG
	imgCall(t, app, http.MethodPost, imagenesPath+"/"+id+"/confirmar", nil, yo.token, http.StatusUnprocessableEntity)
	assert.Empty(t, env.Imagenes.Almacen.Claves(), "el original inválido se borra")
	assert.Empty(t, env.Imagenes.Rekognition.Peticiones(), "no se gasta moderación")
	imgCall(t, app, http.MethodPost, imagenesPath+"/"+id+"/confirmar", nil, yo.token, http.StatusConflict)
}

func TestImagenes_ModeracionRechazaODudosa(t *testing.T) {
	env, app, yo := imagenesEnv(t)

	env.Imagenes.Rekognition.Responder(rekognitiontest.Etiqueta{Name: "Explicit Nudity", Confidence: 97, TaxonomyLevel: 1})
	id := subirFoto(t, env, app, yo.token, "image/jpeg", fotoJPEG(t, 64, 64))
	out := imgCall(t, app, http.MethodPost, imagenesPath+"/"+id+"/confirmar", nil, yo.token, http.StatusOK)
	assert.Equal(t, imagenes.EstadoRechazada, out["estado"])
	assert.Equal(t, imagenes.MotivoModeracion, out["motivo"])
	assert.Nil(t, out["url_1080"])
	assert.Empty(t, env.Imagenes.Almacen.Claves(), "nada rechazado se publica ni se queda")

	env.Imagenes.Rekognition.Responder(rekognitiontest.Etiqueta{Name: "Violence", Confidence: 55, TaxonomyLevel: 1})
	id = subirFoto(t, env, app, yo.token, "image/jpeg", fotoJPEG(t, 64, 64))
	out = imgCall(t, app, http.MethodPost, imagenesPath+"/"+id+"/confirmar", nil, yo.token, http.StatusOK)
	assert.Equal(t, imagenes.EstadoEnRevision, out["estado"])
	assert.Equal(t, []string{"cuarentena/" + id}, env.Imagenes.Almacen.Claves(), "la dudosa sigue en cuarentena para la cola de nv-43")
	imgCall(t, app, http.MethodPost, imagenesPath+"/"+id+"/confirmar", nil, yo.token, http.StatusConflict)
}

func TestImagenes_ModeracionCaidaQuedaEnRevisionYSeReintenta(t *testing.T) {
	env, app, yo := imagenesEnv(t)
	env.Imagenes.Rekognition.Fallar(http.StatusInternalServerError, "InternalServerError")
	id := subirFoto(t, env, app, yo.token, "image/jpeg", fotoJPEG(t, 64, 64))
	out := imgCall(t, app, http.MethodPost, imagenesPath+"/"+id+"/confirmar", nil, yo.token, http.StatusOK)
	assert.Equal(t, imagenes.EstadoEnRevision, out["estado"])
	assert.Equal(t, []string{"cuarentena/" + id}, env.Imagenes.Almacen.Claves(), "sin veredicto no se publica")

	env.Imagenes.Rekognition.Responder()
	out = imgCall(t, app, http.MethodPost, imagenesPath+"/"+id+"/confirmar", nil, yo.token, http.StatusOK)
	assert.Equal(t, imagenes.EstadoPublicada, out["estado"])
}

func TestImagenes_LimitePorHora(t *testing.T) {
	_, app, yo := imagenesEnv(t)
	for range 20 {
		imgCall(t, app, http.MethodPost, imagenesPath+"/subida", map[string]string{"tipo": "image/jpeg"}, yo.token, http.StatusCreated)
	}
	resp := sendJSON(t, app, http.MethodPost, imagenesPath+"/subida", map[string]string{"tipo": "image/jpeg"}, yo.token)
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.NotEmpty(t, resp.Header.Get(fiber.HeaderRetryAfter))
}

func TestImagenes_BorrarLaCuentaBorraSusImagenes(t *testing.T) {
	env, app, yo := imagenesEnv(t)
	id := subirFoto(t, env, app, yo.token, "image/jpeg", fotoJPEG(t, 64, 64))
	imgCall(t, app, http.MethodPost, imagenesPath+"/"+id+"/confirmar", nil, yo.token, http.StatusOK)
	subirFoto(t, env, app, yo.token, "image/jpeg", fotoJPEG(t, 64, 64)) // pendiente

	exportadas, err := env.Imagenes.Privacy.Export(context.Background(), yo.user.ID)
	require.NoError(t, err)
	raw, err := json.Marshal(exportadas)
	require.NoError(t, err)
	assert.Contains(t, string(raw), imagenestest.URLPublica+"/i/"+id+"/1080.jpg")

	require.NoError(t, env.Imagenes.Privacy.PurgeAccount(context.Background(), yo.user.ID))
	assert.Empty(t, env.Imagenes.Almacen.Claves())
	n, err := env.Mongo.DB().Collection(imagenes.Collection).CountDocuments(context.Background(), bson.M{"dueno_id": yo.user.ID})
	require.NoError(t, err)
	assert.Zero(t, n)
}

// Sin IMAGENES_BUCKET e IMAGENES_URL la API arranca y las rutas responden 503 tras la sesión.
func TestImagenes_SinConfiguracionResponde503(t *testing.T) {
	v, err := validation.New()
	require.NoError(t, err)
	deps := realDeps()
	deps.Modules = []httpapi.Module{imagenes.NewHandler(nil, v)}
	app := httpapi.NewApp(testutil.Config(config.EnvStaging), deps)
	token := testutil.ValidTokenFor(t, bson.NewObjectID().Hex(), "jti-sin-imagenes", middleware.RoleConsumer)

	imgCall(t, app, http.MethodPost, imagenesPath+"/subida", map[string]string{"tipo": "image/jpeg"}, "", http.StatusUnauthorized)
	imgCall(t, app, http.MethodPost, imagenesPath+"/subida", map[string]string{"tipo": "image/jpeg"}, token, http.StatusServiceUnavailable)
	imgCall(t, app, http.MethodDelete, imagenesPath+"/0123456789abcdef0123456789abcdef", nil, token, http.StatusServiceUnavailable)
}

// La dudosa entra a la cola de nv-43 y la decisión del administrador la publica o la descarta.
func TestImagenes_ColaDeRevisionPublicaODescarta(t *testing.T) {
	env, app, yo := imagenesEnv(t)
	ctx := context.Background()
	admin := bson.NewObjectID().Hex()
	decidir := func(id, decision string) {
		t.Helper()
		var it moderacion.Item
		require.NoError(t, env.Mongo.DB().Collection(moderacion.ColeccionCola).FindOne(ctx, bson.M{"referencia": id}).Decode(&it))
		assert.Equal(t, imagenes.TipoCola, it.Tipo)
		assert.Equal(t, "cuarentena/"+id, it.ImagenClave, "la cola guarda la llave, nunca los bytes")
		_, err := env.Moderacion.Decidir(ctx, admin, it.ID.Hex(), it.Version, moderacion.DecisionRequest{Decision: decision, Motivo: "revisada"})
		require.NoError(t, err)
	}
	estado := func(id string) string {
		t.Helper()
		var img imagenes.Imagen
		require.NoError(t, env.Mongo.DB().Collection(imagenes.Collection).FindOne(ctx, bson.M{"_id": id}).Decode(&img))
		return img.Estado
	}

	env.Imagenes.Rekognition.Responder(rekognitiontest.Etiqueta{Name: "Violence", Confidence: 55, TaxonomyLevel: 1})
	aprobada := subirFoto(t, env, app, yo.token, "image/jpeg", fotoJPEG(t, 64, 64))
	imgCall(t, app, http.MethodPost, imagenesPath+"/"+aprobada+"/confirmar", nil, yo.token, http.StatusOK)
	decidir(aprobada, moderacion.EstadoAprobado)
	assert.Equal(t, imagenes.EstadoPublicada, estado(aprobada))
	assert.Equal(t, []string{"i/" + aprobada + "/1080.jpg", "i/" + aprobada + "/320.jpg"}, env.Imagenes.Almacen.Claves())

	rechazada := subirFoto(t, env, app, yo.token, "image/jpeg", fotoJPEG(t, 64, 64))
	imgCall(t, app, http.MethodPost, imagenesPath+"/"+rechazada+"/confirmar", nil, yo.token, http.StatusOK)
	decidir(rechazada, moderacion.EstadoRechazado)
	assert.Equal(t, imagenes.EstadoRechazada, estado(rechazada))
	_, quedo := env.Imagenes.Almacen.Objeto("cuarentena/" + rechazada)
	assert.False(t, quedo, "el original rechazado se borra")
}

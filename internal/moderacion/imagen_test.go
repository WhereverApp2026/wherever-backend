package moderacion_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/moderacion"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/iatest"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/rekognitiontest"
)

var imagenPrueba = []byte("\x89PNG\r\n\x1a\nimagen-de-prueba")

func adaptadorLocal(t *testing.T, srv *rekognitiontest.Server) *moderacion.Rekognition {
	t.Helper()
	r, err := moderacion.NewRekognition(context.Background(), moderacion.RekognitionConfig{
		Region: "us-east-2", Endpoint: srv.URL, Credenciales: rekognitiontest.Credenciales(), Timeout: 3 * time.Second,
	})
	require.NoError(t, err)
	return r
}

func evaluarImagen(t *testing.T, srv *rekognitiontest.Server, img []byte) moderacion.Resultado {
	t.Helper()
	m := moderacion.New(adaptadorLocal(t, srv), nil, moderacion.Fijas(moderacion.ReglasPorDefecto()), silencio)
	return m.Evaluar(context.Background(), moderacion.Contenido{Imagen: img})
}

func TestImagen_Limpia(t *testing.T) {
	srv := rekognitiontest.New(t)
	res := evaluarImagen(t, srv, imagenPrueba)
	require.Equal(t, moderacion.VeredictoAprobado, res.Veredicto)
	require.Len(t, res.Puntajes, 4)
	pet := srv.Peticiones()
	require.Len(t, pet, 1)
	require.Equal(t, imagenPrueba, pet[0].Bytes)
	require.InDelta(t, 40, pet[0].MinConfidence, 0)
	require.True(t, pet[0].Firmada, "la petición va firmada con SigV4")
}

func TestImagen_ExplicitaRechaza(t *testing.T) {
	srv := rekognitiontest.New(t).Responder(
		rekognitiontest.Etiqueta{Name: "Explicit", Confidence: 97.5, TaxonomyLevel: 1},
		rekognitiontest.Etiqueta{Name: "Explicit Nudity", ParentName: "Explicit", Confidence: 97.5, TaxonomyLevel: 2},
	)
	res := evaluarImagen(t, srv, imagenPrueba)
	require.Equal(t, moderacion.VeredictoRechazado, res.Veredicto)
	require.InDelta(t, 97.5, res.Puntajes[moderacion.FiltroDesnudez], 0.01)
}

func TestImagen_DudosaVaACuarentenaPorSuRaiz(t *testing.T) {
	// Solo llega la etiqueta de segundo nivel: se sube a su raíz por ParentName.
	srv := rekognitiontest.New(t).Responder(
		rekognitiontest.Etiqueta{Name: "Weapon Violence", ParentName: "Violence", Confidence: 55, TaxonomyLevel: 2},
	)
	res := evaluarImagen(t, srv, imagenPrueba)
	require.Equal(t, moderacion.VeredictoCuarentena, res.Veredicto)
	require.InDelta(t, 55, res.Puntajes[moderacion.FiltroViolencia], 0.01)
}

func TestImagen_AlcoholYTrajeDeBanoNoCuentan(t *testing.T) {
	srv := rekognitiontest.New(t).Responder(
		rekognitiontest.Etiqueta{Name: "Alcohol", Confidence: 98, TaxonomyLevel: 1},
		rekognitiontest.Etiqueta{Name: "Swimwear or Underwear", Confidence: 95, TaxonomyLevel: 1},
	)
	require.Equal(t, moderacion.VeredictoAprobado, evaluarImagen(t, srv, imagenPrueba).Veredicto)
}

func TestImagen_ServicioCaidoDejaEnCuarentena(t *testing.T) {
	for _, f := range []struct {
		estado int
		tipo   string
	}{
		{http.StatusInternalServerError, "InternalServerError"},
		{http.StatusBadRequest, "ThrottlingException"},
		{http.StatusBadRequest, "AccessDeniedException"},
	} {
		srv := rekognitiontest.New(t).Fallar(f.estado, f.tipo)
		res := evaluarImagen(t, srv, imagenPrueba)
		require.Equal(t, moderacion.VeredictoCuarentena, res.Veredicto, f.tipo)
		require.Equal(t, moderacion.FallaImagen, res.Falla, f.tipo)
	}
}

func TestImagen_GrandeNoSaleYQuedaEnCuarentena(t *testing.T) {
	srv := rekognitiontest.New(t)
	res := evaluarImagen(t, srv, make([]byte, moderacion.MaxBytesImagen+1))
	require.Equal(t, moderacion.VeredictoCuarentena, res.Veredicto)
	require.Empty(t, srv.Peticiones())
}

func TestImagen_RechazoGanaAUnaFallaDelTexto(t *testing.T) {
	srv := rekognitiontest.New(t).Responder(
		rekognitiontest.Etiqueta{Name: "Hate Symbols", Confidence: 90, TaxonomyLevel: 1},
	)
	m := moderacion.New(adaptadorLocal(t, srv), moderacion.NewTextoIA(iatest.Nuevo()), moderacion.Fijas(moderacion.ReglasPorDefecto()), silencio)
	res := m.Evaluar(context.Background(), moderacion.Contenido{Imagen: imagenPrueba, Texto: "pie de foto"})
	require.Equal(t, moderacion.VeredictoRechazado, res.Veredicto)
	require.Equal(t, moderacion.FallaTexto, res.Falla)
}

func TestImagen_SinRegion(t *testing.T) {
	_, err := moderacion.NewRekognition(context.Background(), moderacion.RekognitionConfig{})
	require.Error(t, err)
}

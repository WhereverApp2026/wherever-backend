package expediente

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/platform/cifrado"
)

func TestRFCValido_FisicaMoralYFormas(t *testing.T) {
	for _, rfc := range []string{"GODE561231GR8", "ABC680524P76", "ÑAÑA800101AB1", "&AB991231XY0"} {
		assert.True(t, RFCValido(rfc), rfc)
	}
	for _, rfc := range []string{"", "GODE561331GR8", "GODE5612310R", "GO1E561231GR8", "GODE561231GR8X", "AB680524P76", "GODE561231GRB"} {
		assert.False(t, RFCValido(rfc), rfc)
	}
	assert.Equal(t, "GODE561231GR8", NormalizarRFC(" gode-561231 gr8 "))
}

func TestBancoCLABE_DigitoYBanco(t *testing.T) {
	banco, ok := BancoCLABE("002180000118359710")
	require.True(t, ok)
	assert.Equal(t, "BANAMEX", banco)
	banco, ok = BancoCLABE("012180000118359713")
	require.True(t, ok)
	assert.Equal(t, "BBVA MEXICO", banco)

	_, ok = BancoCLABE("012180000118359714")
	assert.False(t, ok, "dígito verificador malo")
	_, ok = BancoCLABE("032180000118359719")
	assert.False(t, ok, "dígito correcto pero banco fuera del catálogo")
	_, ok = BancoCLABE("01218000011835971")
	assert.False(t, ok, "17 dígitos")
	_, ok = BancoCLABE("01218000011835971A")
	assert.False(t, ok, "con letra")
	assert.Equal(t, "9713", Enmascarar("012180000118359713"))
	assert.Empty(t, Enmascarar("12"))
}

func TestCatalogosSAT(t *testing.T) {
	assert.True(t, RegimenValido("612"))
	assert.False(t, RegimenValido("999"))
	assert.True(t, UsoCFDIValido("G03"))
	assert.False(t, UsoCFDIValido("Z99"))
	assert.True(t, CPValido("06600"))
	assert.False(t, CPValido("0660"))
}

func imagen() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for x := 0; x < 8; x++ {
		img.Set(x, x, color.RGBA{R: 200, A: 255})
	}
	return img
}

// jpegConExif inserta un segmento APP1 Exif con texto de GPS después del SOI.
func jpegConExif(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, imagen(), nil))
	raw := buf.Bytes()
	payload := append([]byte("Exif\x00\x00"), []byte("GPSLatitude=19.4326")...)
	require.Less(t, len(payload)+2, 256)
	seg := []byte{0xFF, 0xE1, 0x00, uint8(len(payload) + 2)} // #nosec G115 -- acotado arriba
	out := append([]byte{}, raw[:2]...)
	out = append(out, seg...)
	out = append(out, payload...)
	return append(out, raw[2:]...)
}

func TestPreparar_QuitaExifYDetectaTipoReal(t *testing.T) {
	sucio := jpegConExif(t)
	require.Contains(t, string(sucio), "GPSLatitude")
	tipo, limpio, err := Preparar(sucio)
	require.NoError(t, err)
	assert.Equal(t, TipoJPEG, tipo)
	assert.NotContains(t, string(limpio), "GPSLatitude")
	assert.NotContains(t, string(limpio), "Exif")

	var png1 bytes.Buffer
	require.NoError(t, png.Encode(&png1, imagen()))
	tipo, _, err = Preparar(png1.Bytes())
	require.NoError(t, err)
	assert.Equal(t, TipoPNG, tipo)

	pdf := []byte("%PDF-1.4\n1 0 obj<<>>endobj\ntrailer<<>>\n%%EOF")
	tipo, same, err := Preparar(pdf)
	require.NoError(t, err)
	assert.Equal(t, TipoPDF, tipo)
	assert.Equal(t, pdf, same, "el PDF se guarda tal cual")
}

func TestPreparar_RechazaOtrosTiposYGrandes(t *testing.T) {
	for name, datos := range map[string][]byte{
		"html con extensión jpg": []byte("<html><script>alert(1)</script></html>"),
		"jpeg truncado":          {0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F'},
		"vacío":                  {},
		"zip":                    []byte("PK\x03\x04aaaaaaaaaaaaaaaa"),
	} {
		_, _, err := Preparar(datos)
		assert.ErrorIs(t, err, ErrTipoArchivo, name)
	}
	_, _, err := Preparar(make([]byte, MaxBytes+1))
	assert.ErrorIs(t, err, ErrGrande)
	assert.Equal(t, "pdf", Extension(TipoPDF))
	assert.Equal(t, "png", Extension(TipoPNG))
	assert.Equal(t, "jpg", Extension(TipoJPEG))
}

// El archivo binario cifrado con la capa de nv-48 vuelve igual y no se abre en otro negocio.
func TestCifradoDeArchivo_AtadoAlNegocioYTipo(t *testing.T) {
	l, err := cifrado.CargarLlavero("pruebas:c29sb19wYXJhX3BydWViYXNfY2lmcmFkb18wMDAwMDA=")
	require.NoError(t, err)
	datos := jpegConExif(t)
	n := bson.NewObjectID()
	sellado, err := l.Cifrar(lugar(n, TipoINEFrente), string(datos))
	require.NoError(t, err)
	assert.NotContains(t, sellado, "GPSLatitude")
	plano, err := l.Descifrar(lugar(n, TipoINEFrente), sellado)
	require.NoError(t, err)
	assert.Equal(t, datos, []byte(plano))
	_, err = l.Descifrar(lugar(bson.NewObjectID(), TipoINEFrente), sellado)
	assert.Error(t, err, "copiado a otro negocio no abre")
	_, err = l.Descifrar(lugar(n, TipoINEReverso), sellado)
	assert.Error(t, err, "copiado a otro tipo no abre")
}

func TestService_SinLlaveroResponde503(t *testing.T) {
	s := NewService(nil, nil, nil, nil)
	_, err := s.SubirArchivo(t.Context(), bson.NewObjectID(), bson.NewObjectID(), TipoINEFrente, []byte("x"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no_disponible")
	_, err = s.GuardarFiscales(t.Context(), bson.NewObjectID(), bson.NewObjectID(), DatosFiscales{})
	assert.Contains(t, err.Error(), "no_disponible")
	_, err = s.GuardarCuenta(t.Context(), bson.NewObjectID(), bson.NewObjectID(), "002180000118359710", "Ana")
	assert.Contains(t, err.Error(), "no_disponible")
	_, err = s.LeerAdmin(t.Context(), Acceso{Tipo: TipoFiscales})
	assert.Contains(t, err.Error(), "no_disponible")
}

func TestGuardarFiscales_ValidaAntesDeGuardar(t *testing.T) {
	l, err := cifrado.CargarLlavero("pruebas:c29sb19wYXJhX3BydWViYXNfY2lmcmFkb18wMDAwMDA=")
	require.NoError(t, err)
	s := NewService(nil, l, nil, nil) // sin repositorio: si validara mal, entraría en pánico
	_, err = s.GuardarFiscales(t.Context(), bson.NewObjectID(), bson.NewObjectID(),
		DatosFiscales{RFC: "XXXX", RegimenFiscal: "999", UsoCFDI: "Z", CPFiscal: "1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validacion")
	_, err = s.GuardarCuenta(t.Context(), bson.NewObjectID(), bson.NewObjectID(), "012180000118359714", "Ana")
	assert.Contains(t, err.Error(), "validacion")
}

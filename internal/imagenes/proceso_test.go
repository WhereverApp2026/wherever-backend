package imagenes_test

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/imagenes"
)

// WebP de 1x1 (con pérdida y sin pérdida): Go no los codifica, así que van fijos.
const (
	webpConPerdida = "UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA"
	webpSinPerdida = "UklGRhoAAABXRUJQVlA4TA0AAAAvAAAAEAcQERGIiP4HAA=="
)

func foto(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 90, A: 255})
		}
	}
	return img
}

func jpegDe(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, nil))
	return buf.Bytes()
}

func pngDe(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func dims(t *testing.T, datos []byte) (int, int) {
	t.Helper()
	cfg, formato, err := image.DecodeConfig(bytes.NewReader(datos))
	require.NoError(t, err)
	require.Equal(t, "jpeg", formato, "la salida siempre es JPEG")
	return cfg.Width, cfg.Height
}

// conEXIF mete un APP1 Exif con Orientation=6 (girar 90°) y una etiqueta GPS.
func conEXIF(t *testing.T, original []byte) []byte {
	t.Helper()
	le := binary.LittleEndian
	tiff := []byte{'I', 'I', 0x2A, 0}
	tiff = le.AppendUint32(tiff, 8)
	tiff = le.AppendUint16(tiff, 2) // IFD0 con dos entradas
	tiff = le.AppendUint16(tiff, 0x0112)
	tiff = le.AppendUint16(tiff, 3)
	tiff = le.AppendUint32(tiff, 1)
	tiff = le.AppendUint32(tiff, 6)
	tiff = le.AppendUint16(tiff, 0x8825) // puntero al IFD de GPS
	tiff = le.AppendUint16(tiff, 4)
	tiff = le.AppendUint32(tiff, 1)
	tiff = le.AppendUint32(tiff, 38)
	tiff = le.AppendUint32(tiff, 0)
	tiff = le.AppendUint16(tiff, 1) // IFD de GPS: GPSLatitudeRef = "N"
	tiff = le.AppendUint16(tiff, 1)
	tiff = le.AppendUint16(tiff, 2)
	tiff = le.AppendUint32(tiff, 2)
	tiff = append(tiff, 'N', 0, 0, 0)
	tiff = le.AppendUint32(tiff, 0)
	tiff = append(tiff, []byte("GPS-19.4326N-99.1332W")...)
	seg := append([]byte("Exif\x00\x00"), tiff...)
	app1 := []byte{0xFF, 0xE1}
	app1 = binary.BigEndian.AppendUint16(app1, uint16(len(seg)+2)) //nolint:gosec // segmento de prueba de pocos bytes
	app1 = append(app1, seg...)
	out := append([]byte{0xFF, 0xD8}, app1...)
	return append(out, original[2:]...)
}

// pngCabecera es solo firma + IHDR: basta para DecodeConfig (una bomba no necesita más).
func pngCabecera(w, h uint32, profundidad, tipoColor, entrelazado byte) []byte {
	ihdr := binary.BigEndian.AppendUint32(nil, w)
	ihdr = binary.BigEndian.AppendUint32(ihdr, h)
	ihdr = append(ihdr, profundidad, tipoColor, 0, 0, entrelazado)
	chunk := binary.BigEndian.AppendUint32(nil, uint32(len(ihdr))) //nolint:gosec // IHDR mide 13 bytes
	cuerpo := append([]byte("IHDR"), ihdr...)
	chunk = append(chunk, cuerpo...)
	chunk = binary.BigEndian.AppendUint32(chunk, crc32.ChecksumIEEE(cuerpo))
	return append([]byte("\x89PNG\r\n\x1a\n"), chunk...)
}

func TestProcesar_JPEGReduceADosTamanos(t *testing.T) {
	out, err := imagenes.Procesar(jpegDe(t, foto(2000, 1000)), "image/jpeg")
	require.NoError(t, err)
	w, h := dims(t, out.Grande)
	assert.Equal(t, []int{1080, 540}, []int{w, h})
	w, h = dims(t, out.Chica)
	assert.Equal(t, []int{320, 160}, []int{w, h})
}

func TestProcesar_NoAmplia(t *testing.T) {
	out, err := imagenes.Procesar(jpegDe(t, foto(200, 100)), "image/jpeg")
	require.NoError(t, err)
	w, h := dims(t, out.Grande)
	assert.Equal(t, []int{200, 100}, []int{w, h})
}

func TestProcesar_PNGConTransparenciaVaSobreBlanco(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 40, 40)) // todo transparente
	out, err := imagenes.Procesar(pngDe(t, img), "image/png")
	require.NoError(t, err)
	dec, err := jpeg.Decode(bytes.NewReader(out.Grande))
	require.NoError(t, err)
	r, g, b, _ := dec.At(20, 20).RGBA()
	assert.Greater(t, r>>8, uint32(240))
	assert.Greater(t, g>>8, uint32(240))
	assert.Greater(t, b>>8, uint32(240))
}

func TestProcesar_WebPConYSinPerdida(t *testing.T) {
	for nombre, b64 := range map[string]string{"con pérdida": webpConPerdida, "sin pérdida": webpSinPerdida} {
		datos, err := base64.StdEncoding.DecodeString(b64)
		require.NoError(t, err)
		out, err := imagenes.Procesar(datos, "image/webp")
		require.NoError(t, err, nombre)
		w, h := dims(t, out.Grande)
		assert.Equal(t, []int{1, 1}, []int{w, h}, nombre)
	}
}

func TestProcesar_ExtensionMentirosa(t *testing.T) {
	_, err := imagenes.Procesar(pngDe(t, foto(10, 10)), "image/jpeg")
	assert.ErrorIs(t, err, imagenes.ErrTipoDistinto)
	_, err = imagenes.Procesar(jpegDe(t, foto(10, 10)), "image/webp")
	assert.ErrorIs(t, err, imagenes.ErrTipoDistinto)
}

func TestProcesar_NoEsImagen(t *testing.T) {
	_, err := imagenes.Procesar([]byte("%PDF-1.7 esto no es una foto"), "image/jpeg")
	assert.ErrorIs(t, err, imagenes.ErrFormato)
	gif := []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;")
	_, err = imagenes.Procesar(gif, "image/png")
	assert.ErrorIs(t, err, imagenes.ErrFormato, "GIF no está permitido")
}

func TestProcesar_BombaSeRechazaConLaCabecera(t *testing.T) {
	casos := map[string][]byte{
		"lado mayor a 6000":       pngCabecera(6001, 10, 8, 2, 0),
		"20000x20000":             pngCabecera(20000, 20000, 8, 6, 0),
		"5000x5000 RGBA (100 MB)": pngCabecera(5000, 5000, 8, 6, 0),
		"4000x4000 de 16 bits":    pngCabecera(4000, 4000, 16, 6, 0),
		"3000x3000 entrelazado":   pngCabecera(3000, 3000, 8, 6, 1),
	}
	for nombre, datos := range casos {
		_, err := imagenes.Procesar(datos, "image/png")
		assert.ErrorIs(t, err, imagenes.ErrDimensiones, nombre)
	}
}

func TestProcesar_QuitaEXIFyGPSYRespetaLaOrientacion(t *testing.T) {
	original := conEXIF(t, jpegDe(t, foto(40, 20)))
	require.Contains(t, string(original), "GPS-19.4326N")
	out, err := imagenes.Procesar(original, "image/jpeg")
	require.NoError(t, err)
	for _, datos := range [][]byte{out.Grande, out.Chica} {
		assert.NotContains(t, string(datos), "Exif")
		assert.NotContains(t, string(datos), "GPS-19.4326N")
		w, h := dims(t, datos)
		assert.Equal(t, []int{20, 40}, []int{w, h}, "Orientation=6 gira la foto antes de perder el EXIF")
	}
}

func TestProcesar_JPEGTruncado(t *testing.T) {
	datos := jpegDe(t, foto(100, 100))
	_, err := imagenes.Procesar(datos[:len(datos)/2], "image/jpeg")
	assert.ErrorIs(t, err, imagenes.ErrFormato)
}

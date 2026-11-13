package imagenes

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/png" // registra el decodificador PNG para image.Decode
	"math"

	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // registra el decodificador WebP (solo lectura; excepción de stack nv-45)
)

// Errores del procesamiento: todos terminan en 422.
var (
	ErrFormato      = errors.New("imagenes: el archivo no es una foto JPEG, PNG o WebP válida")
	ErrTipoDistinto = errors.New("imagenes: el contenido no coincide con el tipo declarado")
	ErrDimensiones  = errors.New("imagenes: la foto pasa del tamaño en píxeles permitido")
)

// Límites contra bombas de descompresión (la API tiene 192 MB). Se revisan con la cabecera,
// antes de decodificar: lado máximo y memoria estimada del decodificador.
const (
	MaxLado               = 6000
	MaxBytesDecodificados = 64 << 20
	calidadJPEG           = 82
	// bytesJPEGProgresivo: el decodificador de Go guarda los coeficientes de un JPEG
	// progresivo (int32 por muestra, hasta 3 componentes) además de la imagen.
	bytesJPEGProgresivo = 15
)

// TiposPermitidos son los Content-Type que acepta la subida y el formato que debe tener.
var TiposPermitidos = map[string]string{"image/jpeg": "jpeg", "image/png": "png", "image/webp": "webp"}

// Procesada son los dos tamaños en JPEG, sin metadatos y ya orientados.
type Procesada struct {
	Grande []byte
	Chica  []byte
}

// Procesar valida el original contra el tipo declarado, lo decodifica dentro del presupuesto
// de memoria y lo recodifica en JPEG. Decodificar y volver a codificar deja fuera EXIF, GPS y
// cualquier otro metadato; la orientación EXIF se aplica a los píxeles antes de perderla.
func Procesar(datos []byte, tipo string) (Procesada, error) {
	cfg, formato, err := image.DecodeConfig(bytes.NewReader(datos))
	if err != nil {
		return Procesada{}, ErrFormato
	}
	if TiposPermitidos[tipo] != formato {
		return Procesada{}, ErrTipoDistinto
	}
	if err := revisarDimensiones(cfg, formato, datos); err != nil {
		return Procesada{}, err
	}
	img, _, err := image.Decode(bytes.NewReader(datos))
	if err != nil {
		return Procesada{}, ErrFormato
	}
	orientacion := 1
	if formato == "jpeg" {
		orientacion = orientacionEXIF(datos)
	}
	grande := reducir(img, LadoGrande) // desde aquí el original ya no se usa y el GC lo suelta
	chica := reducir(grande, LadoChico)
	var out Procesada
	if out.Grande, err = codificar(orientar(grande, orientacion)); err != nil {
		return Procesada{}, err
	}
	if out.Chica, err = codificar(orientar(chica, orientacion)); err != nil {
		return Procesada{}, err
	}
	return out, nil
}

func revisarDimensiones(cfg image.Config, formato string, datos []byte) error {
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > MaxLado || cfg.Height > MaxLado {
		return ErrDimensiones
	}
	if float64(cfg.Width)*float64(cfg.Height)*bytesAlDecodificar(cfg.ColorModel, formato, datos) > MaxBytesDecodificados {
		return ErrDimensiones
	}
	return nil
}

// bytesAlDecodificar es lo que ocupa cada píxel al decodificar, con el costo extra de un JPEG
// progresivo o un PNG entrelazado.
func bytesAlDecodificar(m color.Model, formato string, datos []byte) float64 {
	bpp := bytesPorPixel(m)
	switch {
	case formato == "jpeg" && jpegProgresivo(datos):
		return bytesJPEGProgresivo
	case formato == "png" && len(datos) > 28 && datos[28] == 1:
		return bpp * 2 // entrelazado: cada pasada se decodifica aparte antes de copiarse
	}
	return bpp
}

// bytesPorPixel es lo que ocupa la imagen decodificada; YCbCr cuenta 3 (sin submuestreo, el
// peor caso) y un modelo desconocido cuenta 8.
func bytesPorPixel(m color.Model) float64 {
	if _, ok := m.(color.Palette); ok {
		return 1
	}
	switch m {
	case color.GrayModel, color.AlphaModel:
		return 1
	case color.Gray16Model:
		return 2
	case color.YCbCrModel:
		return 3
	case color.RGBAModel, color.NRGBAModel, color.CMYKModel, color.NYCbCrAModel:
		return 4
	default:
		return 8
	}
}

// reducir escala para que el lado mayor no pase de lado (nunca amplía) sobre fondo blanco:
// JPEG no tiene transparencia.
func reducir(src image.Image, lado int) *image.RGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if mayor := max(w, h); mayor > lado {
		escala := float64(lado) / float64(mayor)
		w = max(1, int(math.Round(float64(w)*escala)))
		h = max(1, int(math.Round(float64(h)*escala)))
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.Draw(dst, dst.Bounds(), image.White, image.Point{}, xdraw.Src)
	xdraw.BiLinear.Scale(dst, dst.Bounds(), src, b, xdraw.Over, nil)
	return dst
}

func codificar(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: calidadJPEG}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// orientar aplica la orientación EXIF (1 a 8) a los píxeles.
func orientar(src *image.RGBA, o int) *image.RGBA {
	if o < 2 || o > 8 {
		return src
	}
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	dw, dh := w, h
	if o >= 5 {
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx, dy := destino(o, x, y, w, h)
			si := src.PixOffset(x, y)
			copy(dst.Pix[dst.PixOffset(dx, dy):], src.Pix[si:si+4])
		}
	}
	return dst
}

func destino(o, x, y, w, h int) (int, int) {
	switch o {
	case 2:
		return w - 1 - x, y
	case 3:
		return w - 1 - x, h - 1 - y
	case 4:
		return x, h - 1 - y
	case 5:
		return y, x
	case 6:
		return h - 1 - y, x
	case 7:
		return h - 1 - y, w - 1 - x
	default: // 8
		return y, w - 1 - x
	}
}

// recorrerJPEG visita los segmentos anteriores a la imagen (hasta SOS) y se detiene si visitar
// devuelve false. Ignora un archivo mal formado: la validación real es image.Decode.
func recorrerJPEG(datos []byte, visitar func(marcador byte, segmento []byte) bool) {
	if len(datos) < 4 || datos[0] != 0xFF || datos[1] != 0xD8 {
		return
	}
	for i := 2; i+4 <= len(datos) && datos[i] == 0xFF; {
		marcador := datos[i+1]
		salto, conSegmento, fin := saltoJPEG(marcador)
		if fin {
			return
		}
		if conSegmento {
			seg, ok := segmentoJPEG(datos, i)
			if !ok || !visitar(marcador, seg) {
				return
			}
			salto = 4 + len(seg)
		}
		i += salto
	}
}

// segmentoJPEG devuelve el cuerpo del segmento que empieza en i; false si el largo no cabe.
func segmentoJPEG(datos []byte, i int) ([]byte, bool) {
	largo := int(binary.BigEndian.Uint16(datos[i+2:]))
	if largo < 2 || i+2+largo > len(datos) {
		return nil, false
	}
	return datos[i+4 : i+2+largo], true
}

// saltoJPEG clasifica un marcador: relleno (avanza 1), sin segmento (avanza 2), fin de la
// cabecera (SOS o EOI) o con segmento de largo propio.
func saltoJPEG(marcador byte) (salto int, conSegmento, fin bool) {
	switch {
	case marcador == 0xFF:
		return 1, false, false
	case marcador == 0xDA || marcador == 0xD9:
		return 0, false, true
	case marcador >= 0xD0 && marcador <= 0xD7, marcador == 0x01:
		return 2, false, false
	}
	return 0, true, false
}

// jpegProgresivo dice si el primer SOF es progresivo (SOF2).
func jpegProgresivo(datos []byte) bool {
	progresivo := false
	recorrerJPEG(datos, func(m byte, _ []byte) bool {
		if m >= 0xC0 && m <= 0xCF && m != 0xC4 && m != 0xC8 && m != 0xCC {
			progresivo = m == 0xC2 || m == 0xC6 || m == 0xCA || m == 0xCE
			return false
		}
		return true
	})
	return progresivo
}

// orientacionEXIF lee la etiqueta Orientation (0x0112) del segmento APP1 Exif; 1 si no hay.
func orientacionEXIF(datos []byte) int {
	orientacion := 1
	recorrerJPEG(datos, func(m byte, seg []byte) bool {
		if m == 0xE1 && len(seg) >= 6 && string(seg[:6]) == "Exif\x00\x00" {
			orientacion = orientacionTIFF(seg[6:])
			return false
		}
		return true
	})
	return orientacion
}

// ordenTIFF lee el orden de bytes de la cabecera TIFF; nil si es corta o no es TIFF.
func ordenTIFF(t []byte) binary.ByteOrder {
	if len(t) < 8 {
		return nil
	}
	switch string(t[:2]) {
	case "II":
		return binary.LittleEndian
	case "MM":
		return binary.BigEndian
	}
	return nil
}

func orientacionTIFF(t []byte) int {
	orden := ordenTIFF(t)
	if orden == nil {
		return 1
	}
	inicio := int(orden.Uint32(t[4:8]))
	if inicio < 8 || inicio+2 > len(t) {
		return 1
	}
	n := int(orden.Uint16(t[inicio:]))
	for k := 0; k < n; k++ {
		e := inicio + 2 + 12*k
		if e+12 > len(t) {
			return 1
		}
		if orden.Uint16(t[e:]) == 0x0112 {
			if v := int(orden.Uint16(t[e+8:])); v >= 1 && v <= 8 {
				return v
			}
			return 1
		}
	}
	return 1
}

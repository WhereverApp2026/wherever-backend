package expediente

import (
	"bytes"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
)

// Límites del archivo (spec nv-55).
const (
	MaxBytes   = 5 << 20
	maxPixeles = 40_000_000 // evita bombas de descompresión: 40 MP alcanza para una foto de la INE
)

// Tipos de contenido aceptados, por los primeros bytes y no por la extensión.
const (
	TipoJPEG = "image/jpeg"
	TipoPNG  = "image/png"
	TipoPDF  = "application/pdf"
)

// Errores del archivo.
var (
	ErrTipoArchivo = errors.New("expediente: el archivo debe ser JPEG, PNG o PDF")
	ErrGrande      = errors.New("expediente: el archivo pasa de 5 MB")
)

// Preparar detecta el tipo real del archivo y, si es imagen, la vuelve a codificar para quitar
// EXIF y GPS (los metadatos no sobreviven a decodificar y codificar). Los PDF quedan igual.
func Preparar(datos []byte) (contentType string, limpio []byte, err error) {
	if len(datos) > MaxBytes {
		return "", nil, ErrGrande
	}
	if len(datos) == 0 {
		return "", nil, ErrTipoArchivo
	}
	switch http.DetectContentType(datos) {
	case TipoPDF:
		return TipoPDF, datos, nil
	case TipoJPEG:
		img, err := decodificar(datos, jpeg.Decode)
		if err != nil {
			return "", nil, err
		}
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
			return "", nil, ErrTipoArchivo
		}
		return TipoJPEG, buf.Bytes(), nil
	case TipoPNG:
		img, err := decodificar(datos, png.Decode)
		if err != nil {
			return "", nil, err
		}
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			return "", nil, ErrTipoArchivo
		}
		return TipoPNG, buf.Bytes(), nil
	default:
		return "", nil, ErrTipoArchivo
	}
}

func decodificar(datos []byte, decode func(io.Reader) (image.Image, error)) (image.Image, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(datos))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > maxPixeles {
		return nil, ErrTipoArchivo
	}
	img, err := decode(bytes.NewReader(datos))
	if err != nil {
		return nil, ErrTipoArchivo
	}
	return img, nil
}

// Extension es la extensión del archivo que se entrega al leer.
func Extension(contentType string) string {
	switch contentType {
	case TipoPDF:
		return "pdf"
	case TipoPNG:
		return "png"
	default:
		return "jpg"
	}
}

package imagenes

import "time"

// SubidaRequest pide el formulario firmado. Tipo es el Content-Type del archivo.
type SubidaRequest struct {
	Tipo string `json:"tipo" validate:"required,oneof=image/jpeg image/png image/webp"`
}

// SubidaResponse es el formulario: la app manda un POST multipart a `url` con cada campo de
// `campos` y, al final, el archivo en `file`. Vence en 5 minutos.
type SubidaResponse struct {
	ID       string            `json:"id"`
	URL      string            `json:"url"`
	Campos   map[string]string `json:"campos"`
	VenceEn  time.Time         `json:"vence_en"`
	MaxBytes int64             `json:"max_bytes"`
}

// ImagenResponse es el estado de una imagen; las direcciones solo existen si está publicada.
type ImagenResponse struct {
	ID        string `json:"id"`
	Estado    string `json:"estado"`
	Motivo    string `json:"motivo,omitempty"`
	URLGrande string `json:"url_1080,omitempty"`
	URLChica  string `json:"url_320,omitempty"`
}

func nuevaRespuesta(img *Imagen, base string) *ImagenResponse {
	out := &ImagenResponse{ID: img.ID, Estado: img.Estado, Motivo: img.Motivo}
	if img.Estado == EstadoPublicada && base != "" {
		out.URLGrande = base + "/" + clavePublicada(img.ID, LadoGrande)
		out.URLChica = base + "/" + clavePublicada(img.ID, LadoChico)
	}
	return out
}

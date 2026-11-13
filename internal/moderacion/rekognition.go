package moderacion

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/rekognition"
	"github.com/aws/aws-sdk-go-v2/service/rekognition/types"
)

// MaxBytesImagen es el límite de DetectModerationLabels para imágenes enviadas en bytes.
const MaxBytesImagen = 5 << 20

// confianzaMinima: Rekognition no devuelve etiquetas por debajo; coincide con el umbral de
// cuarentena por defecto. Bajar el umbral en nv-43 por debajo de 40 no ve etiquetas menores.
const confianzaMinima = 40

// raices traduce las categorías de primer nivel de Rekognition (taxonomía v7 y nombres
// anteriores) a los filtros de la spec. Las demás (traje de baño, alcohol, tabaco, apuestas,
// gestos groseros) no cuentan: un bar o un balneario no deben caer en cuarentena por eso.
var raices = map[string]string{
	"Explicit":        FiltroDesnudez,
	"Explicit Nudity": FiltroDesnudez,
	"Non-Explicit Nudity of Intimate parts and Kissing": FiltroDesnudez,
	"Violence":            FiltroViolencia,
	"Visually Disturbing": FiltroViolencia,
	"Drugs & Tobacco":     FiltroDrogas,
	"Drugs":               FiltroDrogas,
	"Hate Symbols":        FiltroOdio,
}

// detector es la parte del cliente de Rekognition que se usa.
type detector interface {
	DetectModerationLabels(ctx context.Context, in *rekognition.DetectModerationLabelsInput, opts ...func(*rekognition.Options)) (*rekognition.DetectModerationLabelsOutput, error)
}

// RekognitionConfig configura el adaptador. Endpoint y Credenciales solo se usan en pruebas
// con un servidor local; en AWS las credenciales salen del rol de la instancia.
type RekognitionConfig struct {
	Region       string
	Timeout      time.Duration
	Endpoint     string
	Credenciales aws.CredentialsProvider
}

// Rekognition evalúa imágenes con Amazon Rekognition DetectModerationLabels.
type Rekognition struct {
	cliente detector
	timeout time.Duration
}

// NewRekognition arma el cliente con la cadena de credenciales de AWS.
func NewRekognition(ctx context.Context, cfg RekognitionConfig) (*Rekognition, error) {
	if cfg.Region == "" {
		return nil, errors.New("moderacion: falta la región de Rekognition")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Second
	}
	opts := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(cfg.Region), awsconfig.WithRetryMaxAttempts(2)}
	if cfg.Credenciales != nil {
		opts = append(opts, awsconfig.WithCredentialsProvider(cfg.Credenciales))
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("moderacion: configuración de AWS: %w", err)
	}
	cliente := rekognition.NewFromConfig(awsCfg, func(o *rekognition.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
	})
	return &Rekognition{cliente: cliente, timeout: cfg.Timeout}, nil
}

// EvaluarImagen devuelve el puntaje más alto de cada filtro; un filtro sin etiquetas vale 0.
// La imagen sale hacia AWS en la misma región y nunca se registra.
func (r *Rekognition) EvaluarImagen(ctx context.Context, imagen []byte) (map[string]float64, error) {
	if len(imagen) > MaxBytesImagen {
		return nil, ErrImagenGrande
	}
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	out, err := r.cliente.DetectModerationLabels(ctx, &rekognition.DetectModerationLabelsInput{
		Image:         &types.Image{Bytes: imagen},
		MinConfidence: aws.Float32(confianzaMinima),
	})
	if err != nil {
		return nil, fmt.Errorf("%w: rekognition: %w", ErrProveedor, err)
	}
	return puntajesDeEtiquetas(out.ModerationLabels)
}

// puntajesDeEtiquetas sube cada etiqueta hasta su categoría de primer nivel por ParentName.
func puntajesDeEtiquetas(etiquetas []types.ModerationLabel) (map[string]float64, error) {
	padre := make(map[string]string, len(etiquetas))
	for _, e := range etiquetas {
		padre[aws.ToString(e.Name)] = aws.ToString(e.ParentName)
	}
	out := map[string]float64{FiltroDesnudez: 0, FiltroViolencia: 0, FiltroDrogas: 0, FiltroOdio: 0}
	for _, e := range etiquetas {
		filtro, ok := raices[raiz(aws.ToString(e.Name), padre)]
		if !ok {
			continue
		}
		p := float64(aws.ToFloat32(e.Confidence))
		if err := validarPuntaje(filtro, p); err != nil {
			return nil, err
		}
		out[filtro] = max(out[filtro], p)
	}
	return out, nil
}

// raiz sigue ParentName hasta una etiqueta sin padre o una conocida. La taxonomía tiene tres
// niveles; el tope evita ciclos en una respuesta mal formada.
func raiz(nombre string, padre map[string]string) string {
	for range 4 {
		if _, ok := raices[nombre]; ok {
			return nombre
		}
		p := padre[nombre]
		if p == "" {
			return nombre
		}
		nombre = p
	}
	return nombre
}

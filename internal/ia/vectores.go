package ia

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Vectorizador calcula vectores de significado (embeddings, spec nv-50). Es la misma idea que
// Modelo: los llamadores dependen de la interfaz y el adaptador vigente es Gemini.
type Vectorizador interface {
	// Documentos vectoriza textos públicos (negocios) en el orden recibido.
	Documentos(ctx context.Context, docs []Documento) ([][]float32, error)
	// Consulta vectoriza una frase de búsqueda; el adaptador la anonimiza antes de enviarla.
	Consulta(ctx context.Context, frase string) ([]float32, error)
	// Nombre identifica proveedor y modelo, nunca la llave.
	Nombre() string
}

// Documento es un texto público con su título.
type Documento struct {
	Titulo string
	Texto  string
}

// Dimensiones de los vectores: 768, que gemini-embedding-2 entrega ya normalizados.
const Dimensiones = 768

// MaxLote es el máximo de documentos por llamada: con 768 números por vector la respuesta de
// 50 cabe de sobra en el límite de lectura de 1 MiB.
const MaxLote = 50

// ErrVectorInvalido: el proveedor devolvió otra cantidad de vectores o de dimensiones.
var ErrVectorInvalido = errors.New("ia: vector inválido")

// GeminiVectoresConfig configura el adaptador. La llave es la misma de nv-26 (GEMINI_API_KEY).
type GeminiVectoresConfig struct {
	APIKey   string
	ModeloID string
	Timeout  time.Duration
	// URLBase vacía usa GeminiURLBase.
	URLBase string
	Cliente *http.Client
}

// GeminiVectores habla con batchEmbedContents. gemini-embedding-2 no acepta task_type: la
// tarea va como prefijo del texto, según su documentación.
type GeminiVectores struct {
	endpoint string
	apiKey   string
	modelo   string
	timeout  time.Duration
	cliente  *http.Client
}

// NewGeminiVectores valida la configuración y arma el adaptador.
func NewGeminiVectores(cfg GeminiVectoresConfig) (*GeminiVectores, error) {
	if cfg.APIKey == "" || cfg.ModeloID == "" {
		return nil, errors.New("ia: los embeddings de Gemini necesitan llave y modelo")
	}
	base := cfg.URLBase
	if base == "" {
		base = GeminiURLBase
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = geminiTimeoutDefault
	}
	cliente := cfg.Cliente
	if cliente == nil {
		cliente = &http.Client{}
	}
	return &GeminiVectores{
		endpoint: strings.TrimRight(base, "/") + "/models/" + url.PathEscape(cfg.ModeloID) + ":batchEmbedContents",
		apiKey:   cfg.APIKey,
		modelo:   cfg.ModeloID,
		timeout:  cfg.Timeout,
		cliente:  cliente,
	}, nil
}

// Nombre identifica proveedor y modelo.
func (g *GeminiVectores) Nombre() string { return "gemini/" + g.modelo }

type embedPeticion struct {
	Model                string          `json:"model"`
	Content              geminiContenido `json:"content"`
	OutputDimensionality int             `json:"output_dimensionality"`
}

type embedLote struct {
	Requests []embedPeticion `json:"requests"`
}

type embedRespuesta struct {
	Embeddings []struct {
		Values []float32 `json:"values"`
	} `json:"embeddings"`
}

// TextoDocumento es el formato de documento de gemini-embedding-2 para recuperación.
func TextoDocumento(d Documento) string {
	titulo := strings.TrimSpace(d.Titulo)
	if titulo == "" {
		titulo = "none"
	}
	return "title: " + titulo + " | text: " + strings.TrimSpace(d.Texto)
}

// TextoConsulta es el formato de consulta de gemini-embedding-2, con la frase anonimizada.
func TextoConsulta(frase string) string {
	return "task: search result | query: " + Anonimizar(strings.TrimSpace(frase))
}

// Documentos vectoriza en una sola llamada; el llamador parte en lotes de MaxLote.
func (g *GeminiVectores) Documentos(ctx context.Context, docs []Documento) ([][]float32, error) {
	if len(docs) == 0 {
		return nil, nil
	}
	if len(docs) > MaxLote {
		return nil, fmt.Errorf("%w: lote de %d, máximo %d", ErrProveedor, len(docs), MaxLote)
	}
	textos := make([]string, len(docs))
	for i, d := range docs {
		textos[i] = TextoDocumento(d)
	}
	return g.lote(ctx, textos)
}

// Consulta vectoriza una frase de búsqueda.
func (g *GeminiVectores) Consulta(ctx context.Context, frase string) ([]float32, error) {
	vs, err := g.lote(ctx, []string{TextoConsulta(frase)})
	if err != nil {
		return nil, err
	}
	return vs[0], nil
}

func (g *GeminiVectores) lote(ctx context.Context, textos []string) ([][]float32, error) {
	pet := embedLote{Requests: make([]embedPeticion, len(textos))}
	for i, t := range textos {
		pet.Requests[i] = embedPeticion{
			Model:                "models/" + g.modelo,
			Content:              geminiContenido{Parts: []geminiParte{{Text: t}}},
			OutputDimensionality: Dimensiones,
		}
	}
	cuerpo, err := json.Marshal(pet)
	if err != nil {
		return nil, fmt.Errorf("%w: petición no serializable", ErrProveedor)
	}
	datos, _, err := postGemini(ctx, g.cliente, g.endpoint, g.apiKey, cuerpo, g.timeout)
	if err != nil {
		return nil, err
	}
	var r embedRespuesta
	if err := json.Unmarshal(datos, &r); err != nil {
		return nil, fmt.Errorf("%w: sobre ilegible", ErrVectorInvalido)
	}
	if len(r.Embeddings) != len(textos) {
		return nil, fmt.Errorf("%w: %d vectores para %d textos", ErrVectorInvalido, len(r.Embeddings), len(textos))
	}
	out := make([][]float32, len(textos))
	for i, e := range r.Embeddings {
		v, err := NormalizarVector(e.Values)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

// NormalizarVector exige 768 números finitos y devuelve el vector con norma 1. El modelo ya los
// normaliza; hacerlo aquí deja el producto punto igual a la similitud coseno aunque cambie.
func NormalizarVector(v []float32) ([]float32, error) {
	if len(v) != Dimensiones {
		return nil, fmt.Errorf("%w: %d dimensiones", ErrVectorInvalido, len(v))
	}
	var suma float64
	for _, x := range v {
		f := float64(x)
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return nil, fmt.Errorf("%w: valor no finito", ErrVectorInvalido)
		}
		suma += f * f
	}
	norma := math.Sqrt(suma)
	if norma == 0 {
		return nil, fmt.Errorf("%w: vector nulo", ErrVectorInvalido)
	}
	out := make([]float32, len(v))
	for i, x := range v {
		out[i] = float32(float64(x) / norma)
	}
	return out, nil
}

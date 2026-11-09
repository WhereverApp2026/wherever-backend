package ia

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// GeminiURLBase es la API REST pública de Gemini. Las pruebas la cambian por un servidor local.
const GeminiURLBase = "https://generativelanguage.googleapis.com/v1beta"

const (
	geminiMaxTokens      = 1024
	geminiTemperatura    = 0.2
	geminiMaxRespuesta   = 1 << 20
	geminiTimeoutDefault = 25 * time.Second
)

// GeminiConfig configura el adaptador. La llave sale de GEMINI_API_KEY (SSM en AWS).
type GeminiConfig struct {
	APIKey   string
	ModeloID string
	Timeout  time.Duration
	// URLBase vacía usa GeminiURLBase.
	URLBase string
	// Cliente vacío usa uno propio; el plazo lo pone el contexto.
	Cliente *http.Client
}

// Gemini habla con generateContent y pide salida JSON con esquema.
type Gemini struct {
	endpoint string
	apiKey   string
	modelo   string
	timeout  time.Duration
	cliente  *http.Client
}

// NewGemini valida la configuración y arma el adaptador.
func NewGemini(cfg GeminiConfig) (*Gemini, error) {
	if cfg.APIKey == "" || cfg.ModeloID == "" {
		return nil, errors.New("ia: Gemini necesita llave y modelo")
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
	return &Gemini{
		endpoint: strings.TrimRight(base, "/") + "/models/" + url.PathEscape(cfg.ModeloID) + ":generateContent",
		apiKey:   cfg.APIKey,
		modelo:   cfg.ModeloID,
		timeout:  cfg.Timeout,
		cliente:  cliente,
	}, nil
}

// Nombre identifica proveedor y modelo.
func (g *Gemini) Nombre() string { return "gemini/" + g.modelo }

type geminiParte struct {
	Text    string `json:"text,omitempty"`
	Thought bool   `json:"thought,omitempty"`
}

type geminiContenido struct {
	Role  string        `json:"role,omitempty"`
	Parts []geminiParte `json:"parts"`
}

type geminiPeticion struct {
	SystemInstruction geminiContenido   `json:"systemInstruction"`
	Contents          []geminiContenido `json:"contents"`
	GenerationConfig  map[string]any    `json:"generationConfig"`
}

type geminiRespuesta struct {
	Candidates []struct {
		Content      geminiContenido `json:"content"`
		FinishReason string          `json:"finishReason"`
	} `json:"candidates"`
	PromptFeedback struct {
		BlockReason string `json:"blockReason"`
	} `json:"promptFeedback"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		ThoughtsTokenCount   int `json:"thoughtsTokenCount"`
	} `json:"usageMetadata"`
}

type geminiError struct {
	Error struct {
		Status  string `json:"status"`
		Details []struct {
			Type       string `json:"@type"`
			RetryDelay string `json:"retryDelay"`
		} `json:"details"`
	} `json:"error"`
}

// Generar hace una llamada. El texto del usuario va anonimizado y en su propia parte del
// turno de usuario; las instrucciones van solo en systemInstruction.
func (g *Gemini) Generar(ctx context.Context, pet Peticion) (Respuesta, error) {
	cuerpo, err := json.Marshal(g.armar(pet))
	if err != nil {
		return Respuesta{}, fmt.Errorf("%w: esquema no serializable", ErrProveedor)
	}
	ctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.endpoint, bytes.NewReader(cuerpo))
	if err != nil {
		return Respuesta{}, fmt.Errorf("%w: petición inválida", ErrProveedor)
	}
	req.Header.Set("Content-Type", "application/json")
	// La llave va en un encabezado, nunca en la URL, para que no aparezca en errores ni logs.
	req.Header.Set("x-goog-api-key", g.apiKey)

	inicio := time.Now()
	resp, err := g.cliente.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return Respuesta{Duracion: time.Since(inicio)}, ErrTiempo
		}
		return Respuesta{Duracion: time.Since(inicio)}, fmt.Errorf("%w: sin conexión", ErrProveedor)
	}
	defer func() { _ = resp.Body.Close() }()
	datos, err := io.ReadAll(io.LimitReader(resp.Body, geminiMaxRespuesta))
	duracion := time.Since(inicio)
	if err != nil {
		if ctx.Err() != nil {
			return Respuesta{Duracion: duracion}, ErrTiempo
		}
		return Respuesta{Duracion: duracion}, fmt.Errorf("%w: respuesta cortada", ErrProveedor)
	}
	if resp.StatusCode != http.StatusOK {
		return Respuesta{Duracion: duracion}, errorDeEstado(resp, datos)
	}
	r, err := leerRespuesta(datos)
	r.Duracion = duracion
	return r, err
}

func (g *Gemini) armar(pet Peticion) geminiPeticion {
	partes := make([]geminiParte, 0, 2)
	if pet.Contexto != "" {
		partes = append(partes, geminiParte{Text: pet.Contexto})
	}
	partes = append(partes, geminiParte{Text: Anonimizar(pet.TextoUsuario)})
	maxTokens := pet.MaxTokens
	if maxTokens <= 0 {
		maxTokens = geminiMaxTokens
	}
	return geminiPeticion{
		SystemInstruction: geminiContenido{Parts: []geminiParte{{Text: pet.Instrucciones}}},
		Contents:          []geminiContenido{{Role: "user", Parts: partes}},
		GenerationConfig: map[string]any{
			"responseMimeType":   "application/json",
			"responseJsonSchema": pet.Esquema,
			"maxOutputTokens":    maxTokens,
			"temperature":        geminiTemperatura,
		},
	}
}

func leerRespuesta(datos []byte) (Respuesta, error) {
	var gr geminiRespuesta
	if err := json.Unmarshal(datos, &gr); err != nil {
		return Respuesta{}, fmt.Errorf("%w: sobre ilegible", ErrRespuestaInvalida)
	}
	r := Respuesta{
		TokensEntrada: gr.UsageMetadata.PromptTokenCount,
		TokensSalida:  gr.UsageMetadata.CandidatesTokenCount + gr.UsageMetadata.ThoughtsTokenCount,
	}
	if gr.PromptFeedback.BlockReason != "" {
		return r, fmt.Errorf("%w: %s", ErrBloqueada, gr.PromptFeedback.BlockReason)
	}
	if len(gr.Candidates) == 0 {
		return r, fmt.Errorf("%w: sin candidatos", ErrRespuestaInvalida)
	}
	c := gr.Candidates[0]
	switch c.FinishReason {
	case "STOP", "":
	case "MAX_TOKENS":
		return r, fmt.Errorf("%w: salida truncada", ErrRespuestaInvalida)
	default:
		return r, fmt.Errorf("%w: %s", ErrBloqueada, c.FinishReason)
	}
	var texto strings.Builder
	for _, p := range c.Content.Parts {
		if !p.Thought {
			texto.WriteString(p.Text)
		}
	}
	salida := bytes.TrimSpace([]byte(texto.String()))
	if !json.Valid(salida) {
		return r, fmt.Errorf("%w: no es JSON", ErrRespuestaInvalida)
	}
	r.JSON = salida
	return r, nil
}

// errorDeEstado traduce el código HTTP. Solo se conserva el estado del proveedor, no el cuerpo.
func errorDeEstado(resp *http.Response, datos []byte) error {
	var ge geminiError
	_ = json.Unmarshal(datos, &ge)
	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		return &LimiteError{Espera: espera(resp.Header.Get("Retry-After"), ge)}
	case resp.StatusCode == http.StatusRequestTimeout || resp.StatusCode == http.StatusGatewayTimeout:
		return ErrTiempo
	default:
		return fmt.Errorf("%w: HTTP %d %s", ErrProveedor, resp.StatusCode, ge.Error.Status)
	}
}

func espera(cabecera string, ge geminiError) time.Duration {
	if s, err := strconv.Atoi(strings.TrimSpace(cabecera)); err == nil && s > 0 {
		return time.Duration(s) * time.Second
	}
	for _, d := range ge.Error.Details {
		if d.RetryDelay == "" {
			continue
		}
		if dur, err := time.ParseDuration(d.RetryDelay); err == nil && dur > 0 {
			return dur
		}
	}
	return 0
}

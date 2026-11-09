package ia_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/ia"
)

// llaveFalsa no es una llave real; solo comprueba por dónde viaja.
const llaveFalsa = "llave-de-prueba"

type salidaPrueba struct {
	Animo string `json:"animo"`
}

func respuestaGemini(texto, fin string) string {
	b, _ := json.Marshal(map[string]any{
		"candidates": []map[string]any{{
			"content":      map[string]any{"role": "model", "parts": []map[string]any{{"text": texto}}},
			"finishReason": fin,
		}},
		"usageMetadata": map[string]any{"promptTokenCount": 40, "candidatesTokenCount": 8, "thoughtsTokenCount": 2},
	})
	return string(b)
}

func servidor(t *testing.T, h http.HandlerFunc) *ia.Gemini {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	g, err := ia.NewGemini(ia.GeminiConfig{APIKey: llaveFalsa, ModeloID: "modelo-prueba", Timeout: 300 * time.Millisecond, URLBase: srv.URL})
	require.NoError(t, err)
	return g
}

func peticion() ia.Peticion {
	return ia.Peticion{
		Instrucciones: "Devuelve el ánimo de la frase.",
		Contexto:      "Candidatos: a1, b2",
		TextoUsuario:  "algo tranquilo, escríbeme a ana@example.com o al 771 123 4567",
		Esquema:       map[string]any{"type": "object", "properties": map[string]any{"animo": map[string]any{"type": "string"}}},
	}
}

func TestGemini_ExitoConPeticionSeparadaYSinDatosPersonales(t *testing.T) {
	var cuerpo map[string]any
	var ruta, llave string
	g := servidor(t, func(w http.ResponseWriter, r *http.Request) {
		ruta, llave = r.URL.String(), r.Header.Get("x-goog-api-key")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &cuerpo)
		_, _ = io.WriteString(w, respuestaGemini(`{"animo":"relax"}`, "STOP"))
	})

	var out salidaPrueba
	r, err := ia.GenerarValidado(context.Background(), g, peticion(), &out)

	require.NoError(t, err)
	assert.Equal(t, "relax", out.Animo)
	assert.Equal(t, 40, r.TokensEntrada)
	assert.Equal(t, 10, r.TokensSalida, "los tokens de razonamiento también se cobran")
	assert.Equal(t, "/models/modelo-prueba:generateContent", ruta, "la llave nunca va en la URL")
	assert.Equal(t, llaveFalsa, llave)

	sistema, _ := json.Marshal(cuerpo["systemInstruction"])
	assert.NotContains(t, string(sistema), "tranquilo", "el texto del usuario nunca va en las instrucciones")
	usuario, _ := json.Marshal(cuerpo["contents"])
	assert.Contains(t, string(usuario), "tranquilo")
	assert.NotContains(t, string(usuario), "ana@example.com")
	assert.NotContains(t, string(usuario), "123 4567")
	config, ok := cuerpo["generationConfig"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "application/json", config["responseMimeType"])
	assert.NotNil(t, config["responseJsonSchema"])
}

func TestGemini_LentoDevuelveErrTiempo(t *testing.T) {
	g := servidor(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
		}
	})

	_, err := g.Generar(context.Background(), peticion())

	assert.ErrorIs(t, err, ia.ErrTiempo)
}

func TestGemini_429EsLimiteDelProveedorYNoSeReintenta(t *testing.T) {
	var llamadas atomic.Int32
	g := servidor(t, func(w http.ResponseWriter, _ *http.Request) {
		llamadas.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"error":{"code":429,"status":"RESOURCE_EXHAUSTED","details":[{"@type":"type.googleapis.com/google.rpc.RetryInfo","retryDelay":"13s"}]}}`)
	})

	var out salidaPrueba
	_, err := ia.GenerarValidado(context.Background(), g, peticion(), &out)

	require.ErrorIs(t, err, ia.ErrLimiteProveedor)
	var limite *ia.LimiteError
	require.True(t, errors.As(err, &limite))
	assert.Equal(t, 13*time.Second, limite.Espera)
	assert.Equal(t, int32(1), llamadas.Load())
}

func TestGemini_5xxEsErrorDelProveedorSinCuerpo(t *testing.T) {
	g := servidor(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, `{"error":{"status":"UNAVAILABLE","message":"detalle interno"}}`)
	})

	_, err := g.Generar(context.Background(), peticion())

	require.ErrorIs(t, err, ia.ErrProveedor)
	assert.Contains(t, err.Error(), "503 UNAVAILABLE")
	assert.NotContains(t, err.Error(), "detalle interno")
	assert.NotContains(t, err.Error(), llaveFalsa)
}

func TestGemini_RespuestaInvalidaSeReintentaUnaVez(t *testing.T) {
	var llamadas atomic.Int32
	g := servidor(t, func(w http.ResponseWriter, _ *http.Request) {
		if llamadas.Add(1) == 1 {
			_, _ = io.WriteString(w, respuestaGemini(`{"animo":"rel`, "MAX_TOKENS"))
			return
		}
		_, _ = io.WriteString(w, respuestaGemini(`{"animo":"fiesta"}`, "STOP"))
	})

	var out salidaPrueba
	r, err := ia.GenerarValidado(context.Background(), g, peticion(), &out)

	require.NoError(t, err)
	assert.Equal(t, "fiesta", out.Animo)
	assert.Equal(t, int32(2), llamadas.Load())
	assert.Equal(t, 80, r.TokensEntrada, "el gasto cuenta los dos intentos")
}

func TestGemini_DosRespuestasInvalidasFallan(t *testing.T) {
	g := servidor(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, respuestaGemini(`{"animo":"relax","extra":1}`, "STOP"))
	})

	var out salidaPrueba
	_, err := ia.GenerarValidado(context.Background(), g, peticion(), &out)

	assert.ErrorIs(t, err, ia.ErrRespuestaInvalida)
}

func TestGemini_BloqueoDelProveedor(t *testing.T) {
	g := servidor(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"promptFeedback":{"blockReason":"SAFETY"}}`)
	})

	_, err := g.Generar(context.Background(), peticion())

	assert.ErrorIs(t, err, ia.ErrBloqueada)
}

func TestGemini_SinLlaveNoSeArma(t *testing.T) {
	_, err := ia.NewGemini(ia.GeminiConfig{ModeloID: "m"})
	assert.Error(t, err)
	_, err = ia.NewGemini(ia.GeminiConfig{APIKey: llaveFalsa})
	assert.Error(t, err)
}

func TestGemini_NombreNoIncluyeLaLlave(t *testing.T) {
	g := servidor(t, func(http.ResponseWriter, *http.Request) {})
	assert.Equal(t, "gemini/modelo-prueba", g.Nombre())
	assert.False(t, strings.Contains(g.Nombre(), llaveFalsa))
}

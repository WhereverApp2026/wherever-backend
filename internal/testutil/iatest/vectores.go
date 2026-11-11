package iatest

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/WhereverApp2026/wherever-backend/internal/ia"
)

// LlaveFalsa no es una llave real; solo comprueba por dónde viaja.
const LlaveFalsa = "llave-de-prueba"

// ServidorVectores imita batchEmbedContents de Gemini en un servidor local (nv-50): devuelve
// vectores de 768 dimensiones deterministas por texto y guarda lo que recibió. Con Estado
// distinto de 200 responde ese error. Es seguro entre goroutines.
type ServidorVectores struct {
	URL string

	mu      sync.Mutex
	estado  int
	llamada int
	textos  []string
	rutas   []string
	llaves  []string
}

// NuevoServidorVectores arranca el servidor y lo cierra al terminar la prueba.
func NuevoServidorVectores(t testing.TB) *ServidorVectores {
	t.Helper()
	s := &ServidorVectores{estado: http.StatusOK}
	srv := httptest.NewServer(http.HandlerFunc(s.atender))
	t.Cleanup(srv.Close)
	s.URL = srv.URL
	return s
}

// Responder fija el estado HTTP de las siguientes llamadas.
func (s *ServidorVectores) Responder(estado int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.estado = estado
}

// Llamadas es cuántas peticiones recibió.
func (s *ServidorVectores) Llamadas() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.llamada
}

// Textos son todos los textos recibidos, en orden.
func (s *ServidorVectores) Textos() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.textos...)
}

// Rutas son la ruta y la consulta de cada petición; la llave nunca debe estar ahí.
func (s *ServidorVectores) Rutas() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.rutas...)
}

// Llaves son los valores de x-goog-api-key recibidos.
func (s *ServidorVectores) Llaves() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.llaves...)
}

type peticionLote struct {
	Requests []struct {
		Model   string `json:"model"`
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
		OutputDimensionality int `json:"output_dimensionality"`
	} `json:"requests"`
}

func (s *ServidorVectores) atender(w http.ResponseWriter, r *http.Request) {
	var pet peticionLote
	_ = json.NewDecoder(r.Body).Decode(&pet)
	s.mu.Lock()
	s.llamada++
	s.rutas = append(s.rutas, r.URL.RequestURI())
	s.llaves = append(s.llaves, r.Header.Get("x-goog-api-key"))
	estado := s.estado
	textos := make([]string, 0, len(pet.Requests))
	for _, req := range pet.Requests {
		var b strings.Builder
		for _, p := range req.Content.Parts {
			b.WriteString(p.Text)
		}
		textos = append(textos, b.String())
	}
	s.textos = append(s.textos, textos...)
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	if estado != http.StatusOK {
		w.WriteHeader(estado)
		_, _ = w.Write([]byte(`{"error":{"status":"PRUEBA"}}`))
		return
	}
	type valores struct {
		Values []float32 `json:"values"`
	}
	out := struct {
		Embeddings []valores `json:"embeddings"`
	}{Embeddings: make([]valores, len(textos))}
	for i, t := range textos {
		out.Embeddings[i] = valores{Values: VectorDe(t)}
	}
	_ = json.NewEncoder(w).Encode(out)
}

// VectorDe es un vector unitario determinista de 768 dimensiones para un texto.
func VectorDe(texto string) []float32 {
	v := make([]float32, ia.Dimensiones)
	var suma float64
	for i := range v {
		bloque := sha256.Sum256([]byte(texto + "|" + strconv.Itoa(i)))
		x := float64(binary.BigEndian.Uint32(bloque[:4]))/math.MaxUint32 - 0.5
		v[i] = float32(x)
		suma += x * x
	}
	norma := math.Sqrt(suma)
	for i := range v {
		v[i] = float32(float64(v[i]) / norma)
	}
	return v
}

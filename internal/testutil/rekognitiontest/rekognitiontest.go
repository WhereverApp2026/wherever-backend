// Package rekognitiontest es un servidor local que habla como Amazon Rekognition
// DetectModerationLabels (JSON 1.1 de AWS) para probar el adaptador real de nv-42 sin
// credenciales ni llamadas a AWS.
package rekognitiontest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
)

// Etiqueta es una etiqueta de moderación como la devuelve Rekognition.
type Etiqueta struct {
	Name          string
	ParentName    string
	Confidence    float64
	TaxonomyLevel int
}

// Peticion es lo que llegó al servidor.
type Peticion struct {
	Bytes         []byte
	MinConfidence float64
	Firmada       bool
}

// Server responde las etiquetas configuradas o un error.
type Server struct {
	URL string

	mu         sync.Mutex
	etiquetas  []Etiqueta
	estado     int
	tipoError  string
	peticiones []Peticion
}

// New levanta el servidor y lo cierra al terminar la prueba. Sin configurar responde una
// imagen limpia.
func New(t testing.TB) *Server {
	t.Helper()
	s := &Server{}
	srv := httptest.NewServer(http.HandlerFunc(s.handle))
	t.Cleanup(srv.Close)
	s.URL = srv.URL
	return s
}

// Credenciales son fijas y falsas; el servidor solo revisa que la petición venga firmada.
func Credenciales() aws.CredentialsProvider {
	return credentials.NewStaticCredentialsProvider("AKIDREKOGNITIONTEST", "secreto-de-prueba", "")
}

// Responder fija las etiquetas de las siguientes respuestas.
func (s *Server) Responder(etiquetas ...Etiqueta) *Server {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.etiquetas, s.estado, s.tipoError = etiquetas, 0, ""
	return s
}

// Fallar hace que las siguientes respuestas sean un error de AWS con ese estado y tipo
// (por ejemplo 400 ThrottlingException o 500 InternalServerError).
func (s *Server) Fallar(estado int, tipo string) *Server {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.estado, s.tipoError = estado, tipo
	return s
}

// Peticiones devuelve lo que llegó, en orden.
func (s *Server) Peticiones() []Peticion {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Peticion(nil), s.peticiones...)
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Amz-Target") != "RekognitionService.DetectModerationLabels" {
		escribirError(w, http.StatusBadRequest, "UnknownOperationException")
		return
	}
	var in struct {
		Image struct {
			Bytes []byte
		}
		MinConfidence float64
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		escribirError(w, http.StatusBadRequest, "SerializationException")
		return
	}
	s.mu.Lock()
	s.peticiones = append(s.peticiones, Peticion{
		Bytes: in.Image.Bytes, MinConfidence: in.MinConfidence,
		Firmada: r.Header.Get("Authorization") != "",
	})
	estado, tipo, etiquetas := s.estado, s.tipoError, append([]Etiqueta(nil), s.etiquetas...)
	s.mu.Unlock()
	if estado != 0 {
		escribirError(w, estado, tipo)
		return
	}
	if etiquetas == nil {
		etiquetas = []Etiqueta{}
	}
	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	_ = json.NewEncoder(w).Encode(map[string]any{"ModerationLabels": etiquetas, "ModerationModelVersion": "7.0"})
}

func escribirError(w http.ResponseWriter, estado int, tipo string) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	w.WriteHeader(estado)
	_ = json.NewEncoder(w).Encode(map[string]string{"__type": tipo, "message": "error de prueba"})
}

// Package fcmtest es un servidor local que habla como FCM HTTP v1 y el intercambio de tokens
// de Google (nv-41). Genera su propia llave: las pruebas nunca usan credenciales reales.
package fcmtest

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// Project es el proyecto falso.
const Project = "wherever-prueba"

// Sent es un mensaje que llegó.
type Sent struct {
	Token string
	Title string
	Body  string
	Data  map[string]string
}

// Server registra lo que recibe y responde según el token.
type Server struct {
	URL         string
	Credentials string // JSON de la cuenta de servicio en base64

	mu           sync.Mutex
	sent         []Sent
	unregistered map[string]bool
	failing      map[string]int // token → cuántas veces más responde 500
	tokenCalls   int
}

// New levanta el servidor y lo cierra al terminar la prueba.
func New(t testing.TB) *Server {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	pemKey := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	s := &Server{unregistered: map[string]bool{}, failing: map[string]int{}}
	srv := httptest.NewServer(http.HandlerFunc(s.handle))
	t.Cleanup(srv.Close)
	s.URL = srv.URL
	creds, err := json.Marshal(map[string]string{
		"client_email": "push@fcmtest.invalid",
		"private_key":  string(pemKey), "token_uri": srv.URL + "/token",
	})
	require.NoError(t, err)
	s.Credentials = base64.StdEncoding.EncodeToString(creds)
	return s
}

// Unregister hace que FCM responda UNREGISTERED para ese token.
func (s *Server) Unregister(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.unregistered[token] = true
}

// FailTimes hace que FCM responda 500 las siguientes n veces para ese token.
func (s *Server) FailTimes(token string, n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failing[token] = n
}

// Sent devuelve una copia de lo enviado.
func (s *Server) Sent() []Sent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Sent{}, s.sent...)
}

// TokenCalls cuenta los intercambios de token de acceso.
func (s *Server) TokenCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tokenCalls
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/token":
		s.mu.Lock()
		s.tokenCalls++
		s.mu.Unlock()
		if r.FormValue("grant_type") != "urn:ietf:params:oauth:grant-type:jwt-bearer" || r.FormValue("assertion") == "" {
			http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"acceso-de-prueba","expires_in":3600,"token_type":"Bearer"}`))
	case "/v1/projects/" + Project + "/messages:send":
		s.send(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) send(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer acceso-de-prueba" {
		http.Error(w, `{"error":{"status":"UNAUTHENTICATED"}}`, http.StatusUnauthorized)
		return
	}
	var body struct {
		Message struct {
			Token        string            `json:"token"`
			Notification map[string]string `json:"notification"`
			Data         map[string]string `json:"data"`
		} `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":{"status":"INVALID_ARGUMENT"}}`, http.StatusBadRequest)
		return
	}
	m := body.Message
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.unregistered[m.Token] {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"status":"NOT_FOUND","details":[{"errorCode":"UNREGISTERED"}]}}`))
		return
	}
	if s.failing[m.Token] > 0 {
		s.failing[m.Token]--
		http.Error(w, `{"error":{"status":"UNAVAILABLE"}}`, http.StatusServiceUnavailable)
		return
	}
	s.sent = append(s.sent, Sent{Token: m.Token, Title: m.Notification["title"], Body: m.Notification["body"], Data: m.Data})
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"name":"projects/` + Project + `/messages/` + strings.Repeat("1", 8) + `"}`))
}

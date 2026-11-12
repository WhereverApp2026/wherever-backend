// Package climatest es un servidor local que imita a Open-Meteo (interfaz externa, nv-25).
// Las pruebas usan el adaptador real contra él y cambian su respuesta, su demora o su falla.
package climatest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Actual es el bloque `current` que devuelve el servidor.
type Actual struct {
	TemperaturaC    float64
	Codigo          int
	EsDeDia         bool
	PrecipitacionMM float64
}

// Servidor imita GET /v1/forecast.
type Servidor struct {
	URL string

	mu      sync.Mutex
	actual  Actual
	estado  int
	tardar  time.Duration
	llamado atomic.Int64
}

// Nuevo arranca el servidor con 22 °C y despejado; se cierra al terminar la prueba.
func Nuevo(t testing.TB) *Servidor {
	t.Helper()
	s := &Servidor{actual: Actual{TemperaturaC: 22, EsDeDia: true}, estado: http.StatusOK}
	srv := httptest.NewServer(http.HandlerFunc(s.responder))
	t.Cleanup(srv.Close)
	s.URL = srv.URL
	return s
}

// Responder fija la lectura y vuelve a 200 sin demora.
func (s *Servidor) Responder(a Actual) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.actual, s.estado, s.tardar = a, http.StatusOK, 0
}

// Fallar responde con ese estado.
func (s *Servidor) Fallar(estado int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.estado = estado
}

// Tardar demora cada respuesta.
func (s *Servidor) Tardar(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tardar = d
}

// Llamadas cuenta las peticiones recibidas.
func (s *Servidor) Llamadas() int { return int(s.llamado.Load()) }

func (s *Servidor) responder(w http.ResponseWriter, r *http.Request) {
	s.llamado.Add(1)
	s.mu.Lock()
	a, estado, tardar := s.actual, s.estado, s.tardar
	s.mu.Unlock()
	if tardar > 0 {
		select {
		case <-time.After(tardar):
		case <-r.Context().Done():
			return
		}
	}
	if r.URL.Path != "/v1/forecast" || r.URL.Query().Get("latitude") == "" {
		estado = http.StatusBadRequest
	}
	if estado != http.StatusOK {
		w.WriteHeader(estado)
		return
	}
	dia := 0
	if a.EsDeDia {
		dia = 1
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"current": map[string]any{
			"time": time.Now().UTC().Format("2006-01-02T15:04"), "interval": 900,
			"temperature_2m": a.TemperaturaC, "weather_code": a.Codigo, "is_day": dia,
			"precipitation": a.PrecipitacionMM,
		},
	})
}

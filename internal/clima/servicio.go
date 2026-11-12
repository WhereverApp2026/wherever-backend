package clima

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"
)

// Vigencias de la caché. Un fallo también se recuerda un minuto: con el proveedor caído,
// cada Inicio esperaría el plazo de 3 s.
const (
	Vigencia      = 30 * time.Minute
	VigenciaFallo = time.Minute
	maxEntradas   = 2048
)

type entrada struct {
	clima  Clima
	expira time.Time
}

// Servicio es el clima con caché en memoria por instancia: el clima no depende del usuario,
// así que no hace falta compartirla entre instancias ni gastar comandos de Redis. Con dos
// instancias, cada celda se pide a lo más dos veces cada 30 min.
type Servicio struct {
	prov  Proveedor
	plazo time.Duration
	log   *slog.Logger
	now   func() time.Time

	mu    sync.Mutex
	cache map[string]entrada
}

// NewServicio arma el servicio con el plazo de 3 s.
func NewServicio(p Proveedor, log *slog.Logger) *Servicio {
	return &Servicio{prov: p, plazo: Plazo, log: log, now: time.Now, cache: map[string]entrada{}}
}

// WithClock fija el reloj (pruebas).
func (s *Servicio) WithClock(now func() time.Time) *Servicio {
	s.now = now
	return s
}

// WithPlazo cambia el plazo (pruebas del servicio lento).
func (s *Servicio) WithPlazo(d time.Duration) *Servicio {
	s.plazo = d
	return s
}

// Reset vacía la caché.
func (s *Servicio) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache = map[string]entrada{}
}

// Redondear lleva la coordenada a una celda de 0,01° (~1 km): es la llave de la caché y
// lo único que sale hacia Open-Meteo, nunca la ubicación exacta.
func Redondear(v float64) float64 { return math.Round(v*100) / 100 }

func llave(lat, lng float64) string { return fmt.Sprintf("%.2f,%.2f", lat, lng) }

// Actual devuelve el clima de la coordenada; nunca falla: sin servicio, NoDisponible.
func (s *Servicio) Actual(ctx context.Context, lat, lng float64) Clima {
	lat, lng = Redondear(lat), Redondear(lng)
	k := llave(lat, lng)
	if c, ok := s.leer(k); ok {
		return c
	}
	ctx, cancel := context.WithTimeout(ctx, s.plazo)
	defer cancel()
	l, err := s.prov.Actual(ctx, lat, lng)
	if err != nil {
		// Sin coordenadas en el log: son la ubicación de alguien.
		s.log.WarnContext(ctx, "clima: proveedor sin respuesta", slog.String("parte", "clima"),
			slog.Bool("plazo_vencido", errors.Is(err, context.DeadlineExceeded)))
		s.guardar(k, NoDisponible(), VigenciaFallo)
		return NoDisponible()
	}
	c := De(l)
	s.guardar(k, c, Vigencia)
	return c
}

func (s *Servicio) leer(k string) (Clima, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.cache[k]
	if !ok || !s.now().Before(e.expira) {
		return Clima{}, false
	}
	return e.clima, true
}

func (s *Servicio) guardar(k string, c Clima, vigencia time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	if len(s.cache) >= maxEntradas {
		for key, e := range s.cache {
			if !now.Before(e.expira) {
				delete(s.cache, key)
			}
		}
		if len(s.cache) >= maxEntradas {
			s.cache = map[string]entrada{}
		}
	}
	s.cache[k] = entrada{clima: c, expira: now.Add(vigencia)}
}

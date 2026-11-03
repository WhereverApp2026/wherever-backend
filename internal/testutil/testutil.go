// Package testutil reúne helpers de prueba: configuración, tokens, logger capturable y
// bases caídas (skill 02-test-data). Solo lo importan tests.
package testutil

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	platformredis "github.com/WhereverApp2026/wherever-backend/internal/platform/redis"
)

// Valores fijos de prueba; no son secretos de ningún entorno.
const (
	SigningKey = "solo_para_pruebas_no_usar_en_ningun_entorno_0123"
	PhoneKey   = "solo_para_pruebas_hmac_de_telefonos_no_usar_0456"
	Issuer     = "wherever-api"
	Audience   = "wherever-app"
	// DeadAddr no tiene nada escuchando: simula una base caída sin reiniciar contenedores.
	DeadAddr = "127.0.0.1:1"
)

// Config arma una configuración válida para el entorno dado.
func Config(env string) config.Config {
	return config.Config{
		Env:           env,
		Port:          8080,
		JWTKey:        []byte(SigningKey),
		JWTIssuer:     Issuer,
		JWTAudience:   Audience,
		AccessTTL:     config.MaxAccessTTL,
		RefreshTTL:    config.MaxRefreshTTL,
		PhoneHMACKey:  []byte(PhoneKey),
		ArcoGraceDays: config.MaxArcoGraceDays,
		Mail: config.Mail{
			SMTPTLS: config.SMTPTLSStartTLS, SMTPPort: 587, Timeout: 5 * time.Second, Workers: 4, QueueSize: 100,
			LinkBase: "http://localhost:8080",
		},
		BodyLimit:       config.MaxBodyLimit,
		RequestTimeout:  config.MaxRequestTimeout,
		RateLimitGlobal: config.DefaultRateGlobal,
		RateLimitWrites: config.DefaultRateWrites,
	}
}

// Env devuelve un getenv con las claves obligatorias válidas más los cambios dados.
func Env(overrides map[string]string) func(string) string {
	values := map[string]string{
		config.KeyAppEnv:       config.EnvDev,
		config.KeyMongoURI:     "mongodb://localhost:27017/wherever",
		config.KeyRedisAddr:    "localhost:6379",
		config.KeyNeo4jURI:     "neo4j://localhost:7687",
		config.KeyJWTKey:       SigningKey,
		config.KeyJWTIssuer:    Issuer,
		config.KeyJWTAudience:  Audience,
		config.KeyPhoneHMACKey: PhoneKey,
	}
	for k, v := range overrides {
		values[k] = v
	}
	return func(key string) string { return values[key] }
}

// DeadRedis crea un cliente real de Redis apuntando a un puerto sin servidor.
func DeadRedis(t *testing.T) *goredis.Client {
	t.Helper()
	client, err := platformredis.New(DeadAddr)
	if err != nil {
		t.Fatalf("redis: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

// DownPinger representa una base que no responde.
type DownPinger struct{}

// Ping siempre falla.
func (DownPinger) Ping(context.Context) error { return errors.New("sin conexión") }

// LogBuffer captura los logs para revisarlos en las pruebas; seguro entre goroutines.
type LogBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *LogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

// String devuelve lo registrado hasta ahora.
func (b *LogBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// Logger devuelve un logger JSON que escribe en un buffer capturable.
func Logger() (*slog.Logger, *LogBuffer) {
	buf := &LogBuffer{}
	return slog.New(slog.NewJSONHandler(buf, nil)), buf
}

// ShortTimeout es un plazo para pruebas que esperan una dependencia caída.
const ShortTimeout = 3 * time.Second

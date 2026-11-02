// Package config resuelve la configuración desde variables de entorno (spec nv-01 R4).
// Se resuelve por constructor: nada se lee al importar el paquete.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Entornos admitidos en APP_ENV.
const (
	EnvDev     = "dev"
	EnvStaging = "staging"
	EnvProd    = "prod"
)

// Límites fijados por la constitución (§3, §6). Se pueden bajar, nunca subir.
const (
	MaxBodyLimit      = 10 * 1024 * 1024
	MaxRequestTimeout = 60 * time.Second
	DefaultRateGlobal = 1000
	DefaultRateWrites = 10
	MinJWTKeyBytes    = 32
	defaultPort       = 8080
	jwtKeyPlaceholder = "PLACEHOLDER"
)

// Claves de entorno que el código usa. Son la fuente para configs/.env.example.
const (
	KeyAppEnv          = "APP_ENV"
	KeyPort            = "PORT"
	KeyMongoURI        = "MONGO_URI"
	KeyRedisAddr       = "REDIS_ADDR"
	KeyNeo4jURI        = "NEO4J_URI"
	KeyJWTKey          = "JWT_KEY"
	KeyJWTIssuer       = "JWT_ISSUER"
	KeyJWTAudience     = "JWT_AUDIENCE"
	KeyCORSOrigin      = "CORS_ALLOWED_ORIGIN"
	KeyTrustedProxies  = "TRUSTED_PROXIES"
	KeyBodyLimit       = "BODY_LIMIT"
	KeyRequestTimeout  = "REQUEST_TIMEOUT"
	KeyRateLimitGlobal = "RATE_LIMIT_GLOBAL"
	KeyRateLimitWrites = "RATE_LIMIT_WRITES"
)

var requiredKeys = []string{
	KeyAppEnv, KeyMongoURI, KeyRedisAddr, KeyNeo4jURI, KeyJWTKey, KeyJWTIssuer, KeyJWTAudience,
}

var optionalKeys = []string{
	KeyPort, KeyCORSOrigin, KeyTrustedProxies, KeyBodyLimit, KeyRequestTimeout, KeyRateLimitGlobal, KeyRateLimitWrites,
}

// Keys devuelve todas las claves que la configuración lee.
func Keys() []string {
	return append(append([]string{}, requiredKeys...), optionalKeys...)
}

// Config es la configuración ya validada.
type Config struct {
	Env               string
	Port              int
	MongoURI          string
	RedisAddr         string
	Neo4jURI          string
	JWTKey            []byte
	JWTIssuer         string
	JWTAudience       string
	CORSAllowedOrigin string
	TrustedProxies    []string
	BodyLimit         int
	RequestTimeout    time.Duration
	RateLimitGlobal   int
	RateLimitWrites   int
}

// IsProd indica si corre en producción.
func (c Config) IsProd() bool { return c.Env == EnvProd }

// IsDev indica si corre en desarrollo.
func (c Config) IsDev() bool { return c.Env == EnvDev }

// Load lee y valida la configuración con la función de lectura dada (os.Getenv en main).
func Load(getenv func(string) string) (Config, error) {
	var errs []error
	for _, key := range requiredKeys {
		if strings.TrimSpace(getenv(key)) == "" {
			errs = append(errs, fmt.Errorf("falta la variable obligatoria %s", key))
		}
	}
	if len(errs) > 0 {
		return Config{}, errors.Join(errs...)
	}

	cfg := Config{
		Env:               getenv(KeyAppEnv),
		MongoURI:          getenv(KeyMongoURI),
		RedisAddr:         getenv(KeyRedisAddr),
		Neo4jURI:          getenv(KeyNeo4jURI),
		JWTKey:            []byte(getenv(KeyJWTKey)),
		JWTIssuer:         getenv(KeyJWTIssuer),
		JWTAudience:       getenv(KeyJWTAudience),
		CORSAllowedOrigin: strings.TrimSpace(getenv(KeyCORSOrigin)),
		TrustedProxies:    splitList(getenv(KeyTrustedProxies)),
	}
	errs = append(errs, cfg.parseNumbers(getenv)...)
	errs = append(errs, cfg.validate()...)
	if len(errs) > 0 {
		return Config{}, errors.Join(errs...)
	}
	return cfg, nil
}

func (c *Config) parseNumbers(getenv func(string) string) []error {
	var errs []error
	ints := []struct {
		key      string
		target   *int
		fallback int
	}{
		{KeyPort, &c.Port, defaultPort},
		{KeyBodyLimit, &c.BodyLimit, MaxBodyLimit},
		{KeyRateLimitGlobal, &c.RateLimitGlobal, DefaultRateGlobal},
		{KeyRateLimitWrites, &c.RateLimitWrites, DefaultRateWrites},
	}
	for _, item := range ints {
		value, err := intOr(getenv(item.key), item.fallback)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: no es un entero", item.key))
		}
		*item.target = value
	}
	timeout, err := durationOr(getenv(KeyRequestTimeout), MaxRequestTimeout)
	if err != nil {
		errs = append(errs, fmt.Errorf("%s: no es una duración (ej. 60s)", KeyRequestTimeout))
	}
	c.RequestTimeout = timeout
	return errs
}

func (c Config) validate() []error {
	var errs []error
	switch c.Env {
	case EnvDev, EnvStaging, EnvProd:
	default:
		errs = append(errs, fmt.Errorf("%s admite solo dev, staging o prod", KeyAppEnv))
	}
	if len(c.JWTKey) < MinJWTKeyBytes || strings.HasPrefix(string(c.JWTKey), jwtKeyPlaceholder) {
		errs = append(errs, fmt.Errorf("%s debe tener al menos %d bytes y no ser el marcador de ejemplo", KeyJWTKey, MinJWTKeyBytes))
	}
	if err := validateOrigin(c.CORSAllowedOrigin); err != nil {
		errs = append(errs, fmt.Errorf("%s: %w", KeyCORSOrigin, err))
	}
	return append(errs, c.validateLimits()...)
}

func (c Config) validateLimits() []error {
	var errs []error
	if c.Port < 1 || c.Port > 65535 {
		errs = append(errs, fmt.Errorf("%s fuera de rango", KeyPort))
	}
	if c.BodyLimit < 1 || c.BodyLimit > MaxBodyLimit {
		errs = append(errs, fmt.Errorf("%s debe estar entre 1 y %d bytes", KeyBodyLimit, MaxBodyLimit))
	}
	if c.RequestTimeout <= 0 || c.RequestTimeout > MaxRequestTimeout {
		errs = append(errs, fmt.Errorf("%s debe ser positivo y no mayor a %s", KeyRequestTimeout, MaxRequestTimeout))
	}
	return append(errs, c.validateRates()...)
}

func (c Config) validateRates() []error {
	var errs []error
	if c.RateLimitGlobal < 1 || c.RateLimitWrites < 1 {
		errs = append(errs, errors.New("los límites de peticiones deben ser positivos"))
	}
	if c.IsProd() && (c.RateLimitGlobal > DefaultRateGlobal || c.RateLimitWrites > DefaultRateWrites) {
		errs = append(errs, errors.New("en prod los límites de peticiones no se pueden relajar"))
	}
	return errs
}

// validateOrigin admite vacío (sin CORS) o un único origen http(s) sin ruta.
func validateOrigin(origin string) error {
	if origin == "" {
		return nil
	}
	if strings.ContainsAny(origin, "*, ") {
		return errors.New("debe ser un solo origen, sin comodines")
	}
	u, err := url.Parse(origin)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || (u.Path != "" && u.Path != "/") {
		return errors.New("debe tener la forma http(s)://host[:puerto]")
	}
	return nil
}

func intOr(raw string, fallback int) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	return strconv.Atoi(strings.TrimSpace(raw))
}

func durationOr(raw string, fallback time.Duration) (time.Duration, error) {
	if strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	return time.ParseDuration(strings.TrimSpace(raw))
}

func splitList(raw string) []string {
	var out []string
	for _, item := range strings.Split(raw, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

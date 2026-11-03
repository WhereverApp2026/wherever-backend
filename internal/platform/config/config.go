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
	MaxAccessTTL      = 15 * time.Minute
	MaxRefreshTTL     = 30 * 24 * time.Hour
	MinPhoneKeyBytes  = 32
	MaxArcoGraceDays  = 30
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
	KeyJWTAccessTTL    = "JWT_ACCESS_TTL"
	KeyJWTRefreshTTL   = "JWT_REFRESH_TTL"
	KeyPhoneHMACKey    = "PHONE_HMAC_KEY"
	KeyArcoGraceDays   = "ARCO_DIAS_GRACIA"
)

var requiredKeys = []string{
	KeyAppEnv, KeyMongoURI, KeyRedisAddr, KeyNeo4jURI, KeyJWTKey, KeyJWTIssuer, KeyJWTAudience, KeyPhoneHMACKey,
}

var optionalKeys = []string{
	KeyPort, KeyCORSOrigin, KeyTrustedProxies, KeyBodyLimit, KeyRequestTimeout, KeyRateLimitGlobal, KeyRateLimitWrites,
	KeyJWTAccessTTL, KeyJWTRefreshTTL, KeyArcoGraceDays,
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
	AccessTTL         time.Duration
	RefreshTTL        time.Duration
	PhoneHMACKey      []byte
	ArcoGraceDays     int
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
		PhoneHMACKey:      []byte(getenv(KeyPhoneHMACKey)),
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
		{KeyArcoGraceDays, &c.ArcoGraceDays, MaxArcoGraceDays},
	}
	for _, item := range ints {
		value, err := intOr(getenv(item.key), item.fallback)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: no es un entero", item.key))
		}
		*item.target = value
	}
	durations := []struct {
		key      string
		target   *time.Duration
		fallback time.Duration
	}{
		{KeyRequestTimeout, &c.RequestTimeout, MaxRequestTimeout},
		{KeyJWTAccessTTL, &c.AccessTTL, MaxAccessTTL},
		{KeyJWTRefreshTTL, &c.RefreshTTL, MaxRefreshTTL},
	}
	for _, item := range durations {
		value, err := durationOr(getenv(item.key), item.fallback)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: no es una duración (ej. 60s)", item.key))
		}
		*item.target = value
	}
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
	if len(c.PhoneHMACKey) < MinPhoneKeyBytes || strings.HasPrefix(string(c.PhoneHMACKey), jwtKeyPlaceholder) {
		errs = append(errs, fmt.Errorf("%s debe tener al menos %d bytes y no ser el marcador de ejemplo", KeyPhoneHMACKey, MinPhoneKeyBytes))
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
	errs = append(errs, c.validateTTLs()...)
	return append(errs, c.validateRates()...)
}

// validateTTLs aplica la constitución §6: access 15 min y refresh 30 días como máximo.
func (c Config) validateTTLs() []error {
	var errs []error
	if c.AccessTTL <= 0 || c.AccessTTL > MaxAccessTTL {
		errs = append(errs, fmt.Errorf("%s debe ser positivo y no mayor a %s", KeyJWTAccessTTL, MaxAccessTTL))
	}
	if c.RefreshTTL <= 0 || c.RefreshTTL > MaxRefreshTTL {
		errs = append(errs, fmt.Errorf("%s debe ser positivo y no mayor a %s", KeyJWTRefreshTTL, MaxRefreshTTL))
	}
	// Hoja A1: la cuenta se borra 30 días después de pedirlo; se puede acortar, no alargar.
	if c.ArcoGraceDays < 1 || c.ArcoGraceDays > MaxArcoGraceDays {
		errs = append(errs, fmt.Errorf("%s debe estar entre 1 y %d", KeyArcoGraceDays, MaxArcoGraceDays))
	}
	return errs
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

// Package config resuelve la configuración desde variables de entorno (spec nv-01 R4).
// Se resuelve por constructor: nada se lee al importar el paquete.
package config

import (
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/WhereverApp2026/wherever-backend/internal/platform/cifrado"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/push"
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
	// Saltos de confianza en X-Forwarded-For: 2 con CloudFront y balanceador (nv-12).
	DefaultTrustedProxyHops = 1
	MaxTrustedProxyHops     = 3
	DefaultRateGlobal       = 1000
	DefaultRateWrites       = 10
	MinJWTKeyBytes          = 32
	MaxAccessTTL            = 15 * time.Minute
	MaxRefreshTTL           = 30 * 24 * time.Hour
	MinPhoneKeyBytes        = 32
	MinCursorKeyBytes       = 32
	MaxArcoGraceDays        = 30
	// DefaultJobHour es la madrugada de America/Mexico_City; un servidor que se apaga de noche la cambia.
	DefaultJobHour     = 3
	maxJobHour         = 23
	defaultSMTPPort    = 587
	defaultMailTimeout = 5 * time.Second
	maxMailTimeout     = 30 * time.Second
	defaultMailWorkers = 4
	maxMailWorkers     = 32
	defaultMailQueue   = 100
	maxMailQueue       = 10000
	defaultPort        = 8080
	// IA generativa (nv-26): plazo por llamada al proveedor.
	defaultIATimeout = 25 * time.Second
	maxIATimeout     = 55 * time.Second
	// Tope de gasto de la IA (nv-27), en USD al mes; el máximo es el presupuesto de la cuenta.
	defaultIATopeUSDMes = 5.0
	maxIATopeUSDMes     = 50.0
	maxIAPrecioUSDMTok  = 100.0
	// Límites de la IA (nv-28): por usuario y hora, y globales al 80 % de la capa gratuita de
	// gemini-3.5-flash-lite (15 por minuto, 500 por día). El máximo es el límite del proveedor.
	defaultIALimiteHora = 10
	maxIALimiteHora     = 50
	defaultIALimiteMin  = 12
	maxIALimiteMin      = 15
	defaultIALimiteDia  = 400
	maxIALimiteDia      = 500
	// Embeddings (nv-50): textos por minuto y por día. AI Studio no publica la cuota gratuita de
	// gemini-embedding-2; los máximos son los de gemini-embedding-001 (100 y 1000) y el valor por
	// defecto su 80 %. Se ajustan al leer la cuota del proyecto en AI Studio.
	defaultIAEmbLimiteMin = 80
	maxIAEmbLimiteMin     = 100
	defaultIAEmbLimiteDia = 800
	maxIAEmbLimiteDia     = 1000
	jwtKeyPlaceholder     = "PLACEHOLDER"
)

// Claves de entorno que el código usa. Son la fuente para configs/.env.example.
const (
	KeyAppEnv           = "APP_ENV"
	KeyPort             = "PORT"
	KeyMongoURI         = "MONGO_URI"
	KeyRedisAddr        = "REDIS_ADDR"
	KeyNeo4jURI         = "NEO4J_URI"
	KeyJWTKey           = "JWT_KEY"
	KeyJWTIssuer        = "JWT_ISSUER"
	KeyJWTAudience      = "JWT_AUDIENCE"
	KeyCORSOrigin       = "CORS_ALLOWED_ORIGIN"
	KeyTrustedProxies   = "TRUSTED_PROXIES"
	KeyTrustedProxyHops = "TRUSTED_PROXY_HOPS"
	KeyBodyLimit        = "BODY_LIMIT"
	KeyRequestTimeout   = "REQUEST_TIMEOUT"
	KeyRateLimitGlobal  = "RATE_LIMIT_GLOBAL"
	KeyRateLimitWrites  = "RATE_LIMIT_WRITES"
	KeyJWTAccessTTL     = "JWT_ACCESS_TTL"
	KeyJWTRefreshTTL    = "JWT_REFRESH_TTL"
	KeyPhoneHMACKey     = "PHONE_HMAC_KEY"
	KeyCursorHMACKey    = "CURSOR_HMAC_KEY"
	KeyArcoGraceDays    = "ARCO_DIAS_GRACIA"
	KeySMTPHost         = "SMTP_HOST"
	KeySMTPPort         = "SMTP_PORT"
	KeySMTPUser         = "SMTP_USER"
	KeySMTPPassword     = "SMTP_PASSWORD"
	KeySMTPTLS          = "SMTP_TLS"
	KeyMailFrom         = "MAIL_FROM"
	KeyMailTimeout      = "MAIL_TIMEOUT"
	KeyMailWorkers      = "MAIL_WORKERS"
	KeyMailQueueSize    = "MAIL_QUEUE_SIZE"
	KeyAppLinkBase      = "APP_LINK_BASE"
	KeyJobHour          = "JOB_HOUR"
	KeyHideInternals    = "HIDE_INTERNALS"
	KeyGeminiAPIKey     = "GEMINI_API_KEY" // #nosec G101 -- nombre de la variable, no su valor
	KeyIAModeloID       = "IA_MODELO_ID"
	KeyIATimeout        = "IA_TIMEOUT"
	KeyIATopeUSDMes     = "IA_TOPE_USD_MES"
	KeyIAPrecioEntrada  = "IA_PRECIO_ENTRADA_USD_MTOK"
	KeyIAPrecioSalida   = "IA_PRECIO_SALIDA_USD_MTOK"
	KeyIALimiteHora     = "IA_LIMITE_HORA"
	KeyIALimiteMin      = "IA_LIMITE_MIN"
	KeyIALimiteDia      = "IA_LIMITE_DIA"
	KeyIAEmbeddingsID   = "IA_EMBEDDINGS_ID"
	KeyIAEmbLimiteMin   = "IA_EMB_LIMITE_MIN"
	KeyIAEmbLimiteDia   = "IA_EMB_LIMITE_DIA"
	KeyAppVersionMinima = "APP_VERSION_MINIMA"
	KeyAppVersionActual = "APP_VERSION_ACTUAL"
	KeyTerminosVersion  = "TERMINOS_VERSION"
	KeyAvisoVersion     = "AVISO_VERSION"
	KeyMantenimiento    = "MODO_MANTENIMIENTO"
	KeyFCMProjectID     = "FCM_PROJECT_ID"
	KeyFCMCredenciales  = "FCM_CREDENCIALES_B64" // #nosec G101 -- nombre de la variable, no su valor
	KeyFCMURL           = "FCM_URL"
	KeyDataKeys         = "DATA_KEYS" // #nosec G101 -- nombre de la variable, no su valor
	KeyImagenesBucket   = "IMAGENES_BUCKET"
	KeyImagenesURL      = "IMAGENES_URL"
	KeyImagenesRegion   = "IMAGENES_REGION"
)

// DefaultImagenesRegion es la región del bucket y de Rekognition (nv-44, nv-45).
const DefaultImagenesRegion = "us-east-2"

// Modos de SMTP_TLS (spec nv-06).
const (
	SMTPTLSStartTLS = "starttls"
	SMTPTLSImplicit = "tls"
	SMTPTLSNone     = "none"
)

var requiredKeys = []string{
	KeyAppEnv, KeyMongoURI, KeyRedisAddr, KeyNeo4jURI, KeyJWTKey, KeyJWTIssuer, KeyJWTAudience, KeyPhoneHMACKey,
	KeyCursorHMACKey,
}

var optionalKeys = []string{
	KeyPort, KeyCORSOrigin, KeyTrustedProxies, KeyTrustedProxyHops, KeyBodyLimit, KeyRequestTimeout, KeyRateLimitGlobal, KeyRateLimitWrites,
	KeyJWTAccessTTL, KeyJWTRefreshTTL, KeyArcoGraceDays,
	KeySMTPHost, KeySMTPPort, KeySMTPUser, KeySMTPPassword, KeySMTPTLS, KeyMailFrom,
	KeyMailTimeout, KeyMailWorkers, KeyMailQueueSize, KeyAppLinkBase,
	KeyJobHour, KeyHideInternals,
	KeyGeminiAPIKey, KeyIAModeloID, KeyIATimeout, KeyIATopeUSDMes, KeyIAPrecioEntrada, KeyIAPrecioSalida,
	KeyIALimiteHora, KeyIALimiteMin, KeyIALimiteDia, KeyIAEmbeddingsID, KeyIAEmbLimiteMin, KeyIAEmbLimiteDia,
	KeyAppVersionMinima, KeyAppVersionActual, KeyTerminosVersion, KeyAvisoVersion, KeyMantenimiento,
	KeyFCMProjectID, KeyFCMCredenciales, KeyFCMURL, KeyDataKeys,
	KeyImagenesBucket, KeyImagenesURL, KeyImagenesRegion,
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
	CursorHMACKey     []byte
	ArcoGraceDays     int
	Mail              Mail
	CORSAllowedOrigin string
	TrustedProxies    []string
	TrustedProxyHops  int
	BodyLimit         int
	RequestTimeout    time.Duration
	RateLimitGlobal   int
	RateLimitWrites   int
	JobHour           int
	HideInternals     bool
	IA                IA
	// DataKeys son las llaves de cifrado de campos (nv-48): `id:base64,…`, la primera cifra.
	DataKeys string
	// FCM es el push por Firebase (nv-41); sin credenciales solo queda el centro de avisos.
	FCM        FCM
	VersionApp VersionApp
	// Imagenes es el bucket y el CloudFront de imágenes (nv-44, nv-45); sin ellos las rutas
	// de imágenes responden 503.
	Imagenes Imagenes
}

// IA es la configuración de la IA generativa (nv-26). Sin llave o sin modelo, la IA queda
// apagada y los llamadores usan el motor de reglas.
type IA struct {
	GeminiAPIKey string
	ModeloID     string
	Timeout      time.Duration
	// TopeUSDMes es el gasto estimado máximo del mes para toda la app (nv-27).
	TopeUSDMes float64
	// Precios en USD por millón de tokens; 0 en la capa gratuita.
	PrecioEntradaUSDMTok float64
	PrecioSalidaUSDMTok  float64
	// LimiteHora son las generaciones por usuario y hora (nv-28).
	LimiteHora int
	// LimiteMin y LimiteDia son las llamadas al proveedor de toda la app por minuto y por día
	// (hora del Pacífico).
	LimiteMin int
	LimiteDia int
	// EmbeddingsID es el modelo de embeddings (nv-50); usa la misma llave.
	EmbeddingsID string
	// EmbLimiteMin y EmbLimiteDia son los textos vectorizados de toda la app por minuto y por día.
	EmbLimiteMin int
	EmbLimiteDia int
}

// Enabled indica si hay proveedor de IA configurado.
func (i IA) Enabled() bool { return i.GeminiAPIKey != "" && i.ModeloID != "" }

// EmbeddingsEnabled indica si hay modelo de embeddings configurado (nv-50).
func (i IA) EmbeddingsEnabled() bool { return i.GeminiAPIKey != "" && i.EmbeddingsID != "" }

// Mail es la configuración del correo saliente (nv-06). Sin SMTPHost no hay servidor: dev y
// staging usan el LogMailer; producción no arranca.
type Mail struct {
	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPassword string
	SMTPTLS      string
	From         string
	Timeout      time.Duration
	Workers      int
	QueueSize    int
	LinkBase     string
}

// Enabled indica si hay servidor SMTP configurado.
func (m Mail) Enabled() bool { return m.SMTPHost != "" }

// IsProd indica si corre en producción.
func (c Config) IsProd() bool { return c.Env == EnvProd }

// IsDev indica si corre en desarrollo.
func (c Config) IsDev() bool { return c.Env == EnvDev }

// ExposeInternals indica si se muestran `detalle` y las sondas. Nunca en prod, y tampoco en un
// servidor público de otro entorno que pida ocultarlos con HIDE_INTERNALS (spec nv-15).
func (c Config) ExposeInternals() bool { return !c.IsProd() && !c.HideInternals }

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
		Env:           getenv(KeyAppEnv),
		MongoURI:      getenv(KeyMongoURI),
		RedisAddr:     getenv(KeyRedisAddr),
		Neo4jURI:      getenv(KeyNeo4jURI),
		JWTKey:        []byte(getenv(KeyJWTKey)),
		JWTIssuer:     getenv(KeyJWTIssuer),
		JWTAudience:   getenv(KeyJWTAudience),
		PhoneHMACKey:  []byte(getenv(KeyPhoneHMACKey)),
		CursorHMACKey: []byte(getenv(KeyCursorHMACKey)),
		Mail: Mail{
			SMTPHost:     strings.TrimSpace(getenv(KeySMTPHost)),
			SMTPUser:     getenv(KeySMTPUser),
			SMTPPassword: getenv(KeySMTPPassword),
			SMTPTLS:      orDefault(strings.ToLower(strings.TrimSpace(getenv(KeySMTPTLS))), SMTPTLSStartTLS),
			From:         strings.TrimSpace(getenv(KeyMailFrom)),
			LinkBase:     strings.TrimSpace(getenv(KeyAppLinkBase)),
		},
		CORSAllowedOrigin: strings.TrimSpace(getenv(KeyCORSOrigin)),
		TrustedProxies:    splitList(getenv(KeyTrustedProxies)),
		HideInternals:     strings.EqualFold(strings.TrimSpace(getenv(KeyHideInternals)), "true"),
		DataKeys:          strings.TrimSpace(getenv(KeyDataKeys)),
		FCM: FCM{
			ProjectID:      strings.TrimSpace(getenv(KeyFCMProjectID)),
			CredentialsB64: strings.TrimSpace(getenv(KeyFCMCredenciales)),
			BaseURL:        strings.TrimSpace(getenv(KeyFCMURL)),
		},
		Imagenes: Imagenes{
			Bucket: strings.TrimSpace(getenv(KeyImagenesBucket)),
			URL:    strings.TrimRight(strings.TrimSpace(getenv(KeyImagenesURL)), "/"),
			Region: orDefault(strings.TrimSpace(getenv(KeyImagenesRegion)), DefaultImagenesRegion),
		},
		IA: IA{
			GeminiAPIKey: strings.TrimSpace(getenv(KeyGeminiAPIKey)),
			ModeloID:     strings.TrimSpace(getenv(KeyIAModeloID)),
			EmbeddingsID: strings.TrimSpace(getenv(KeyIAEmbeddingsID)),
		},
	}
	errs = append(errs, cfg.parseNumbers(getenv)...)
	errs = append(errs, cfg.VersionApp.parse(getenv)...)
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
		{KeySMTPPort, &c.Mail.SMTPPort, defaultSMTPPort},
		{KeyMailWorkers, &c.Mail.Workers, defaultMailWorkers},
		{KeyMailQueueSize, &c.Mail.QueueSize, defaultMailQueue},
		{KeyJobHour, &c.JobHour, DefaultJobHour},
		{KeyTrustedProxyHops, &c.TrustedProxyHops, DefaultTrustedProxyHops},
		{KeyIALimiteHora, &c.IA.LimiteHora, defaultIALimiteHora},
		{KeyIALimiteMin, &c.IA.LimiteMin, defaultIALimiteMin},
		{KeyIALimiteDia, &c.IA.LimiteDia, defaultIALimiteDia},
		{KeyIAEmbLimiteMin, &c.IA.EmbLimiteMin, defaultIAEmbLimiteMin},
		{KeyIAEmbLimiteDia, &c.IA.EmbLimiteDia, defaultIAEmbLimiteDia},
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
		{KeyMailTimeout, &c.Mail.Timeout, defaultMailTimeout},
		{KeyIATimeout, &c.IA.Timeout, defaultIATimeout},
	}
	for _, item := range durations {
		value, err := durationOr(getenv(item.key), item.fallback)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: no es una duración (ej. 60s)", item.key))
		}
		*item.target = value
	}
	return append(errs, c.parseFloats(getenv)...)
}

func (c *Config) parseFloats(getenv func(string) string) []error {
	var errs []error
	floats := []struct {
		key      string
		target   *float64
		fallback float64
	}{
		{KeyIATopeUSDMes, &c.IA.TopeUSDMes, defaultIATopeUSDMes},
		{KeyIAPrecioEntrada, &c.IA.PrecioEntradaUSDMTok, 0},
		{KeyIAPrecioSalida, &c.IA.PrecioSalidaUSDMTok, 0},
	}
	for _, item := range floats {
		value := item.fallback
		if raw := strings.TrimSpace(getenv(item.key)); raw != "" {
			parsed, err := strconv.ParseFloat(raw, 64)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: no es un número", item.key))
			}
			value = parsed
		}
		*item.target = value
	}
	return errs
}

// validateKeys exige el tamaño mínimo de cada llave y que no sea el marcador de ejemplo.
func (c Config) validateKeys() []error {
	var errs []error
	for _, k := range []struct {
		name string
		key  []byte
		min  int
	}{
		{KeyJWTKey, c.JWTKey, MinJWTKeyBytes},
		{KeyPhoneHMACKey, c.PhoneHMACKey, MinPhoneKeyBytes},
		{KeyCursorHMACKey, c.CursorHMACKey, MinCursorKeyBytes},
	} {
		if len(k.key) < k.min || strings.HasPrefix(string(k.key), jwtKeyPlaceholder) {
			errs = append(errs, fmt.Errorf("%s debe tener al menos %d bytes y no ser el marcador de ejemplo", k.name, k.min))
		}
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
	errs = append(errs, c.validateKeys()...)
	if err := validateOrigin(c.CORSAllowedOrigin); err != nil {
		errs = append(errs, fmt.Errorf("%s: %w", KeyCORSOrigin, err))
	}
	if c.JobHour < 0 || c.JobHour > maxJobHour {
		errs = append(errs, fmt.Errorf("%s debe estar entre 0 y %d", KeyJobHour, maxJobHour))
	}
	errs = append(errs, c.validateMail()...)
	errs = append(errs, c.IA.validate()...)
	errs = append(errs, c.FCM.validate(c.Env)...)
	errs = append(errs, c.Imagenes.validate(c.Env)...)
	errs = append(errs, c.validateDataKeys()...)
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
	if c.TrustedProxyHops < 1 || c.TrustedProxyHops > MaxTrustedProxyHops {
		errs = append(errs, fmt.Errorf("%s debe estar entre 1 y %d", KeyTrustedProxyHops, MaxTrustedProxyHops))
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

// validateMail aplica nv-06: en producción hace falta servidor, con TLS y enlaces https://.
func (c Config) validateMail() []error {
	m := c.Mail
	var errs []error
	if c.IsProd() && !m.Enabled() {
		errs = append(errs, fmt.Errorf("en prod %s es obligatorio: sin correo nadie verifica su cuenta ni recupera su contraseña", KeySMTPHost))
	}
	switch m.SMTPTLS {
	case SMTPTLSStartTLS, SMTPTLSImplicit:
	case SMTPTLSNone:
		if c.IsProd() {
			errs = append(errs, fmt.Errorf("en prod %s no puede ser none", KeySMTPTLS))
		}
		if m.SMTPUser != "" {
			errs = append(errs, fmt.Errorf("%s=none con %s mandaría la contraseña sin cifrar", KeySMTPTLS, KeySMTPUser))
		}
	default:
		errs = append(errs, fmt.Errorf("%s admite starttls, tls o none", KeySMTPTLS))
	}
	if m.Enabled() {
		errs = append(errs, m.validateServer(c.IsProd())...)
	}
	return append(errs, m.validateQueue()...)
}

func (m Mail) validateServer(prod bool) []error {
	var errs []error
	if m.SMTPPort < 1 || m.SMTPPort > 65535 {
		errs = append(errs, fmt.Errorf("%s fuera de rango", KeySMTPPort))
	}
	if addr, err := mail.ParseAddress(m.From); err != nil || addr.Name != "" || strings.ContainsAny(m.From, "\r\n") {
		errs = append(errs, fmt.Errorf("%s debe ser un correo simple (remitente@dominio)", KeyMailFrom))
	}
	if err := validateLinkBase(m.LinkBase, prod); err != nil {
		errs = append(errs, fmt.Errorf("%s: %w", KeyAppLinkBase, err))
	}
	return errs
}

func (m Mail) validateQueue() []error {
	var errs []error
	if m.Timeout <= 0 || m.Timeout > maxMailTimeout {
		errs = append(errs, fmt.Errorf("%s debe ser positivo y no mayor a %s", KeyMailTimeout, maxMailTimeout))
	}
	if m.Workers < 1 || m.Workers > maxMailWorkers {
		errs = append(errs, fmt.Errorf("%s debe estar entre 1 y %d", KeyMailWorkers, maxMailWorkers))
	}
	if m.QueueSize < 1 || m.QueueSize > maxMailQueue {
		errs = append(errs, fmt.Errorf("%s debe estar entre 1 y %d", KeyMailQueueSize, maxMailQueue))
	}
	return errs
}

// validateLinkBase exige http(s)://host[/ruta] sin consulta ni fragmento; en prod solo https.
func validateLinkBase(base string, prod bool) error {
	u, err := url.Parse(base)
	if err != nil || u.Host == "" || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("debe tener la forma http(s)://host[/ruta], sin consulta ni fragmento")
	}
	switch {
	case u.Scheme == "https":
		return nil
	case u.Scheme == "http" && !prod:
		return nil
	case prod:
		return errors.New("en prod debe ser https://")
	default:
		return errors.New("debe ser http:// o https://")
	}
}

func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// validate aplica nv-26: el plazo cabe dentro del de la petición y la llave no es un marcador.
func (i IA) validate() []error {
	var errs []error
	if i.Timeout <= 0 || i.Timeout > maxIATimeout {
		errs = append(errs, fmt.Errorf("%s debe ser positivo y no mayor a %s", KeyIATimeout, maxIATimeout))
	}
	if strings.HasPrefix(i.GeminiAPIKey, jwtKeyPlaceholder) {
		errs = append(errs, fmt.Errorf("%s no puede ser el marcador de ejemplo; déjala vacía para apagar la IA", KeyGeminiAPIKey))
	}
	// nv-27: !(x > 0) también rechaza NaN.
	if !(i.TopeUSDMes > 0) || i.TopeUSDMes > maxIATopeUSDMes {
		errs = append(errs, fmt.Errorf("%s debe ser positivo y no mayor a %v", KeyIATopeUSDMes, maxIATopeUSDMes))
	}
	errs = append(errs, validatePrice(KeyIAPrecioEntrada, i.PrecioEntradaUSDMTok)...)
	errs = append(errs, validatePrice(KeyIAPrecioSalida, i.PrecioSalidaUSDMTok)...)
	errs = append(errs, validateLimit(KeyIALimiteHora, i.LimiteHora, maxIALimiteHora)...)
	errs = append(errs, validateLimit(KeyIALimiteMin, i.LimiteMin, maxIALimiteMin)...)
	errs = append(errs, validateLimit(KeyIALimiteDia, i.LimiteDia, maxIALimiteDia)...)
	errs = append(errs, validateLimit(KeyIAEmbLimiteMin, i.EmbLimiteMin, maxIAEmbLimiteMin)...)
	errs = append(errs, validateLimit(KeyIAEmbLimiteDia, i.EmbLimiteDia, maxIAEmbLimiteDia)...)
	if strings.ContainsAny(i.ModeloID, "/?#: ") {
		errs = append(errs, fmt.Errorf("%s debe ser solo el nombre del modelo (ej. gemini-3.5-flash-lite)", KeyIAModeloID))
	}
	if strings.ContainsAny(i.EmbeddingsID, "/?#: ") {
		errs = append(errs, fmt.Errorf("%s debe ser solo el nombre del modelo (ej. gemini-embedding-2)", KeyIAEmbeddingsID))
	}
	return errs
}

// validateLimit acepta de 1 al máximo (nv-28).
func validateLimit(key string, value, maxValue int) []error {
	if value < 1 || value > maxValue {
		return []error{fmt.Errorf("%s debe estar entre 1 y %d", key, maxValue)}
	}
	return nil
}

// validatePrice acepta de 0 al máximo; !(x >= 0) también rechaza NaN.
func validatePrice(key string, price float64) []error {
	if !(price >= 0) || price > maxIAPrecioUSDMTok {
		return []error{fmt.Errorf("%s debe estar entre 0 y %v", key, maxIAPrecioUSDMTok)}
	}
	return nil
}

// FCM es la configuración del push (nv-41). Las credenciales son el JSON de la cuenta de
// servicio de Firebase en base64 (una línea, para `.env` y Parameter Store).
type FCM struct {
	ProjectID      string
	CredentialsB64 string
	BaseURL        string
}

// Enabled dice si hay push: proyecto y credenciales.
func (f FCM) Enabled() bool { return f.ProjectID != "" && f.CredentialsB64 != "" }

// validate: van juntos o ninguno; las credenciales deben ser una cuenta de servicio válida y
// FCM_URL solo cambia fuera de producción (pruebas locales).
func (f FCM) validate(env string) []error {
	var errs []error
	if (f.ProjectID == "") != (f.CredentialsB64 == "") {
		errs = append(errs, fmt.Errorf("%s y %s van juntos (o ninguno, y no hay push)", KeyFCMProjectID, KeyFCMCredenciales))
	}
	if f.CredentialsB64 != "" {
		if _, _, err := push.ParseCredentials(f.CredentialsB64); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", KeyFCMCredenciales, err))
		}
	}
	if f.BaseURL != "" && env == EnvProd {
		errs = append(errs, fmt.Errorf("%s no se cambia en prod", KeyFCMURL))
	}
	return errs
}

// Imagenes configura la subida (nv-45): bucket privado y URL pública de su CloudFront.
type Imagenes struct {
	Bucket string
	URL    string
	Region string
}

// Enabled dice si hay imágenes: bucket y URL.
func (i Imagenes) Enabled() bool { return i.Bucket != "" && i.URL != "" }

// validate: en prod son obligatorias; fuera de prod van juntas o ninguna (sin ellas la API
// arranca y las rutas de imágenes responden 503). La URL es HTTPS salvo en dev.
func (i Imagenes) validate(env string) []error {
	var errs []error
	switch {
	case !i.Enabled() && env == EnvProd:
		errs = append(errs, fmt.Errorf("%s y %s son obligatorias en prod (nv-45)", KeyImagenesBucket, KeyImagenesURL))
	case (i.Bucket == "") != (i.URL == ""):
		errs = append(errs, fmt.Errorf("%s y %s van juntas (o ninguna, y no hay imágenes)", KeyImagenesBucket, KeyImagenesURL))
	}
	if i.URL != "" && !urlImagenesValida(i.URL, env) {
		errs = append(errs, fmt.Errorf("%s debe ser https://dominio, sin ruta", KeyImagenesURL))
	}
	return errs
}

// urlImagenesValida: https://dominio sin ruta ni consulta; http solo en dev.
func urlImagenesValida(raw, env string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.Path != "" || u.RawQuery != "" {
		return false
	}
	return u.Scheme == "https" || (env == EnvDev && u.Scheme == "http")
}

// validateDataKeys: en prod las llaves son obligatorias (sin ellas la API no arranca); fuera
// de prod pueden faltar y los campos sensibles se guardan en claro con un aviso al arrancar.
func (c Config) validateDataKeys() []error {
	if c.DataKeys == "" {
		if c.Env == EnvProd {
			return []error{fmt.Errorf("%s es obligatoria en prod (cifrado de campos, nv-48)", KeyDataKeys)}
		}
		return nil
	}
	if _, err := cifrado.CargarLlavero(c.DataKeys); err != nil {
		return []error{fmt.Errorf("%s: %w", KeyDataKeys, err)}
	}
	return nil
}

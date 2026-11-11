package config_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
)

func TestLoad_ValidConfigWithDefaults(t *testing.T) {
	cfg, err := config.Load(testutil.Env(nil))

	require.NoError(t, err)
	assert.Equal(t, config.EnvDev, cfg.Env)
	assert.Equal(t, 8080, cfg.Port)
	assert.Equal(t, 10*1024*1024, cfg.BodyLimit)
	assert.Equal(t, 60*time.Second, cfg.RequestTimeout)
	assert.Equal(t, 1000, cfg.RateLimitGlobal)
	assert.Equal(t, 10, cfg.RateLimitWrites)
	assert.Equal(t, 15*time.Minute, cfg.AccessTTL)
	assert.Equal(t, 30*24*time.Hour, cfg.RefreshTTL)
	assert.Empty(t, cfg.CORSAllowedOrigin)
}

func TestLoad_MissingRequiredKeyFails(t *testing.T) {
	_, err := config.Load(testutil.Env(map[string]string{config.KeyMongoURI: ""}))

	require.Error(t, err)
	assert.Contains(t, err.Error(), config.KeyMongoURI)
}

func TestLoad_ShortJWTKeyFails(t *testing.T) {
	for name, key := range map[string]string{
		"corta":    "corta",
		"marcador": "PLACEHOLDER_MIN_32_BYTES_PERO_MAS_LARGO",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := config.Load(testutil.Env(map[string]string{config.KeyJWTKey: key}))
			require.Error(t, err)
			assert.Contains(t, err.Error(), config.KeyJWTKey)
		})
	}
}

func TestLoad_RejectsInvalidValues(t *testing.T) {
	cases := map[string]map[string]string{
		"APP_ENV desconocido":     {config.KeyAppEnv: "local"},
		"CORS con comodín":        {config.KeyCORSOrigin: "*"},
		"CORS con dos orígenes":   {config.KeyCORSOrigin: "https://a.com,https://b.com"},
		"cuerpo mayor a 10MB":     {config.KeyBodyLimit: "20971520"},
		"timeout mayor a 60s":     {config.KeyRequestTimeout: "90s"},
		"rate relajado en prod":   {config.KeyAppEnv: config.EnvProd, config.KeyRateLimitGlobal: "5000"},
		"entero inválido":         {config.KeyPort: "ochenta"},
		"access mayor a 15 min":   {config.KeyJWTAccessTTL: "30m"},
		"refresh mayor a 30 d":    {config.KeyJWTRefreshTTL: "1000h"},
		"access no positivo":      {config.KeyJWTAccessTTL: "0s"},
		"llave de teléfono corta": {config.KeyPhoneHMACKey: "corta"},
		"gracia mayor a 30 días":  {config.KeyArcoGraceDays: "45"},
	}
	for name, overrides := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := config.Load(testutil.Env(overrides))
			assert.Error(t, err)
		})
	}
}

func prodMail(overrides map[string]string) map[string]string {
	values := map[string]string{
		config.KeyAppEnv:      config.EnvProd,
		config.KeySMTPHost:    "smtp.proveedor.test",
		config.KeyMailFrom:    "no-responder@wherever.test",
		config.KeyAppLinkBase: "https://wherever.test",
	}
	for k, v := range overrides {
		values[k] = v
	}
	return values
}

func TestLoad_MailDefaultsWithoutServer(t *testing.T) {
	cfg, err := config.Load(testutil.Env(nil))

	require.NoError(t, err)
	assert.False(t, cfg.Mail.Enabled(), "dev sin SMTP_HOST usa el LogMailer")
	assert.Equal(t, config.SMTPTLSStartTLS, cfg.Mail.SMTPTLS)
	assert.Equal(t, 587, cfg.Mail.SMTPPort)
	assert.Equal(t, 5*time.Second, cfg.Mail.Timeout)
	assert.Equal(t, 4, cfg.Mail.Workers)
	assert.Equal(t, 100, cfg.Mail.QueueSize)
}

func TestLoad_ProdMailRules(t *testing.T) {
	_, err := config.Load(testutil.Env(prodMail(nil)))
	require.NoError(t, err, "prod con servidor, TLS y https arranca")

	for name, overrides := range map[string]map[string]string{
		"sin SMTP_HOST":        {config.KeySMTPHost: ""},
		"SMTP_TLS none":        {config.KeySMTPTLS: "none"},
		"enlace http":          {config.KeyAppLinkBase: "http://wherever.test"},
		"enlace con consulta":  {config.KeyAppLinkBase: "https://wherever.test/?x=1"},
		"remitente con nombre": {config.KeyMailFrom: "Wherever <no-responder@wherever.test>"},
		"modo TLS desconocido": {config.KeySMTPTLS: "ssl"},
		"cola enorme":          {config.KeyMailQueueSize: "50000"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := config.Load(testutil.Env(prodMail(overrides)))
			assert.Error(t, err)
		})
	}
}

func TestLoad_NoPasswordOverPlainSMTP(t *testing.T) {
	_, err := config.Load(testutil.Env(map[string]string{
		config.KeySMTPHost: "127.0.0.1", config.KeySMTPPort: "1025", config.KeySMTPTLS: "none",
		config.KeyMailFrom: "dev@wherever.test", config.KeyAppLinkBase: "http://localhost:8080",
	}))
	require.NoError(t, err, "Mailpit en dev: sin TLS y sin usuario")

	_, err = config.Load(testutil.Env(map[string]string{
		config.KeySMTPHost: "127.0.0.1", config.KeySMTPTLS: "none", config.KeySMTPUser: "alguien",
		config.KeyMailFrom: "dev@wherever.test", config.KeyAppLinkBase: "http://localhost:8080",
	}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sin cifrar")
}

func TestLoad_IAApagadaSinLlave(t *testing.T) {
	cfg, err := config.Load(testutil.Env(nil))

	require.NoError(t, err)
	assert.False(t, cfg.IA.Enabled(), "sin GEMINI_API_KEY responde el motor de reglas")
	assert.Equal(t, 25*time.Second, cfg.IA.Timeout)
	assert.InDelta(t, 5.0, cfg.IA.TopeUSDMes, 1e-9, "nv-27: tope de 5 USD al mes por defecto")
	assert.Zero(t, cfg.IA.PrecioEntradaUSDMTok, "capa gratuita: costo cero")
	assert.Zero(t, cfg.IA.PrecioSalidaUSDMTok)
}

func TestLoad_IATopeYPrecios(t *testing.T) {
	cfg, err := config.Load(testutil.Env(map[string]string{
		config.KeyIATopeUSDMes: "12.5", config.KeyIAPrecioEntrada: "0.1", config.KeyIAPrecioSalida: "0.4",
	}))

	require.NoError(t, err)
	assert.InDelta(t, 12.5, cfg.IA.TopeUSDMes, 1e-9)
	assert.InDelta(t, 0.1, cfg.IA.PrecioEntradaUSDMTok, 1e-9)
	assert.InDelta(t, 0.4, cfg.IA.PrecioSalidaUSDMTok, 1e-9)
}

func TestLoad_IAEncendidaConLlaveYModelo(t *testing.T) {
	cfg, err := config.Load(testutil.Env(map[string]string{
		config.KeyGeminiAPIKey: "llave-de-prueba", config.KeyIAModeloID: "gemini-3.5-flash-lite",
	}))

	require.NoError(t, err)
	assert.True(t, cfg.IA.Enabled())
}

func TestLoad_IARechazaValoresInvalidos(t *testing.T) {
	cases := map[string]map[string]string{
		"plazo mayor a 55s":     {config.KeyIATimeout: "90s"},
		"plazo no positivo":     {config.KeyIATimeout: "0s"},
		"llave marcador":        {config.KeyGeminiAPIKey: "PLACEHOLDER_GEMINI"},
		"modelo con models/":    {config.KeyIAModeloID: "models/gemini-3.5-flash-lite"},
		"modelo con parámetros": {config.KeyIAModeloID: "gemini?key=x"},
		"tope cero":             {config.KeyIATopeUSDMes: "0"},
		"tope sobre 50":         {config.KeyIATopeUSDMes: "51"},
		"tope NaN":              {config.KeyIATopeUSDMes: "NaN"},
		"tope no numérico":      {config.KeyIATopeUSDMes: "cinco"},
		"precio negativo":       {config.KeyIAPrecioEntrada: "-1"},
		"precio infinito":       {config.KeyIAPrecioSalida: "Inf"},
	}
	for name, overrides := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := config.Load(testutil.Env(overrides))
			assert.Error(t, err)
		})
	}
}

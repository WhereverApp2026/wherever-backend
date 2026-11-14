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
		"APP_ENV desconocido":      {config.KeyAppEnv: "local"},
		"CORS con comodín":         {config.KeyCORSOrigin: "*"},
		"CORS con dos orígenes":    {config.KeyCORSOrigin: "https://a.com,https://b.com"},
		"cuerpo mayor a 10MB":      {config.KeyBodyLimit: "20971520"},
		"timeout mayor a 60s":      {config.KeyRequestTimeout: "90s"},
		"rate relajado en prod":    {config.KeyAppEnv: config.EnvProd, config.KeyRateLimitGlobal: "5000"},
		"entero inválido":          {config.KeyPort: "ochenta"},
		"access mayor a 15 min":    {config.KeyJWTAccessTTL: "30m"},
		"refresh mayor a 30 d":     {config.KeyJWTRefreshTTL: "1000h"},
		"access no positivo":       {config.KeyJWTAccessTTL: "0s"},
		"llave de teléfono corta":  {config.KeyPhoneHMACKey: "corta"},
		"llave de cursor corta":    {config.KeyCursorHMACKey: "corta"},
		"llave de cursor marcador": {config.KeyCursorHMACKey: "PLACEHOLDER_MIN_32_BYTES_xxxxxxxxxxxxx"},
		"gracia mayor a 30 días":   {config.KeyArcoGraceDays: "45"},
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
		config.KeyAppEnv:         config.EnvProd,
		config.KeySMTPHost:       "smtp.proveedor.test",
		config.KeyMailFrom:       "no-responder@wherever.test",
		config.KeyAppLinkBase:    "https://wherever.test",
		config.KeyImagenesBucket: "wherever-prod-imagenes",
		config.KeyImagenesURL:    "https://imagenes.wherever.test",
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
	assert.Equal(t, 10, cfg.IA.LimiteHora, "nv-33: 10 generaciones por usuario y hora (5 planes de dos pasos)")
	assert.Equal(t, 12, cfg.IA.LimiteMin, "nv-28: 80 %% de 15 por minuto")
	assert.Equal(t, 400, cfg.IA.LimiteDia, "nv-28: 80 %% de 500 por día")
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
		"plazo mayor a 55s":      {config.KeyIATimeout: "90s"},
		"límite por hora cero":   {config.KeyIALimiteHora: "0"},
		"límite por minuto alto": {config.KeyIALimiteMin: "16"},
		"límite por día alto":    {config.KeyIALimiteDia: "501"},
		"límite no entero":       {config.KeyIALimiteDia: "mucho"},
		"plazo no positivo":      {config.KeyIATimeout: "0s"},
		"llave marcador":         {config.KeyGeminiAPIKey: "PLACEHOLDER_GEMINI"},
		"modelo con models/":     {config.KeyIAModeloID: "models/gemini-3.5-flash-lite"},
		"modelo con parámetros":  {config.KeyIAModeloID: "gemini?key=x"},
		"tope cero":              {config.KeyIATopeUSDMes: "0"},
		"tope sobre 50":          {config.KeyIATopeUSDMes: "51"},
		"tope NaN":               {config.KeyIATopeUSDMes: "NaN"},
		"tope no numérico":       {config.KeyIATopeUSDMes: "cinco"},
		"precio negativo":        {config.KeyIAPrecioEntrada: "-1"},
		"precio infinito":        {config.KeyIAPrecioSalida: "Inf"},
	}
	for name, overrides := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := config.Load(testutil.Env(overrides))
			assert.Error(t, err)
		})
	}
}

// nv-48: sin DATA_KEYS la API no arranca en prod; fuera de prod puede faltar; mal formada
// nunca arranca.
func TestLoad_DataKeys(t *testing.T) {
	_, err := config.Load(testutil.Env(prodMail(map[string]string{config.KeyDataKeys: ""})))
	assert.ErrorContains(t, err, config.KeyDataKeys)
	_, err = config.Load(testutil.Env(map[string]string{config.KeyDataKeys: ""}))
	assert.NoError(t, err, "en dev puede faltar")
	_, err = config.Load(testutil.Env(map[string]string{config.KeyDataKeys: "v1:corta"}))
	assert.ErrorContains(t, err, config.KeyDataKeys)
	cfg, err := config.Load(testutil.Env(prodMail(nil)))
	require.NoError(t, err)
	assert.Equal(t, testutil.DataKeys, cfg.DataKeys)
}

func TestLoad_Imagenes(t *testing.T) {
	cfg, err := config.Load(testutil.Env(nil))
	require.NoError(t, err, "fuera de prod pueden faltar")
	assert.False(t, cfg.Imagenes.Enabled())
	assert.Equal(t, config.DefaultImagenesRegion, cfg.Imagenes.Region)

	cfg, err = config.Load(testutil.Env(map[string]string{config.KeyImagenesBucket: "b", config.KeyImagenesURL: "https://d.cloudfront.net/"}))
	require.NoError(t, err)
	assert.True(t, cfg.Imagenes.Enabled())
	assert.Equal(t, "https://d.cloudfront.net", cfg.Imagenes.URL, "sin la diagonal final")

	for name, env := range map[string]map[string]string{
		"solo bucket":   {config.KeyImagenesBucket: "b"},
		"solo url":      {config.KeyImagenesURL: "https://d.cloudfront.net"},
		"url con ruta":  {config.KeyImagenesBucket: "b", config.KeyImagenesURL: "https://d.cloudfront.net/i"},
		"url sin https": {config.KeyAppEnv: config.EnvStaging, config.KeyImagenesBucket: "b", config.KeyImagenesURL: "http://d.cloudfront.net"},
	} {
		_, err := config.Load(testutil.Env(env))
		assert.ErrorContains(t, err, config.KeyImagenesURL, name)
	}
	_, err = config.Load(testutil.Env(prodMail(map[string]string{config.KeyImagenesBucket: "", config.KeyImagenesURL: ""})))
	assert.ErrorContains(t, err, config.KeyImagenesBucket, "obligatorias en prod")
}

func TestLoad_Stripe(t *testing.T) {
	cfg, err := config.Load(testutil.Env(nil))
	require.NoError(t, err, "sin llaves la API arranca")
	assert.False(t, cfg.Stripe.Enabled())

	prueba, live := "sk_"+"test_prueba", "sk_"+"live_prueba"
	cfg, err = config.Load(testutil.Env(map[string]string{config.KeyStripeSecretKey: prueba, config.KeyStripeWebhook: "whsec_prueba", config.KeyStripeAPIURL: "http://127.0.0.1:1/"}))
	require.NoError(t, err)
	assert.True(t, cfg.Stripe.Enabled())
	assert.Equal(t, "http://127.0.0.1:1", cfg.Stripe.APIURL)

	for name, env := range map[string]map[string]string{
		"live en dev":        {config.KeyStripeSecretKey: live},
		"restringida en dev": {config.KeyStripeSecretKey: "rk_" + "live_prueba"},
		"live en staging":    {config.KeyAppEnv: config.EnvStaging, config.KeyStripeSecretKey: live},
		"prueba en prod":     prodMail(map[string]string{config.KeyStripeSecretKey: prueba}),
		"url del doble prod": prodMail(map[string]string{config.KeyStripeAPIURL: "http://127.0.0.1:1"}),
	} {
		_, err := config.Load(testutil.Env(env))
		assert.Error(t, err, name)
	}
}

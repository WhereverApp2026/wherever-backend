// Package httpapi arma la app Fiber con el pipeline fijo de la constitución §4.
package httpapi

import (
	"context"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	goredis "github.com/redis/go-redis/v9"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/versionado"
)

const (
	idleTimeout  = 120 * time.Second
	globalWindow = time.Minute
	healthPath   = "/health"
)

// Pinger es lo que /ready necesita de cada base.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Module es un módulo de negocio (nv-04+) que registra sus rutas bajo /api/v1. Recibe el
// limitador para sus políticas por ruta; sus dependencias llegan por su propio constructor.
type Module interface {
	Routes(v1 fiber.Router, limiter *middleware.Limiter)
}

// Deps son las dependencias ya construidas; se inyectan para que los tests reutilicen NewApp.
type Deps struct {
	Logger  *slog.Logger
	Version string
	Redis   *goredis.Client
	Mongo   Pinger
	Neo4j   Pinger
	// MongoIndexes, si viene, hace que /ready dé 503 cuando falta un índice declarado.
	MongoIndexes Pinger
	Modules      []Module
}

// Stage es una etapa con nombre del pipeline.
type Stage struct {
	Name    string
	Handler fiber.Handler
}

// Pipeline devuelve las etapas en el orden constitucional. El critic (B3) y los tests
// verifican este orden; no se reordena sin enmendar la constitución.
func Pipeline(cfg config.Config, deps Deps, limiter *middleware.Limiter) []Stage {
	errorHandler := middleware.NewErrorHandler(cfg.ExposeInternals(), deps.Logger)
	jwtCfg := middleware.JWTConfig{Key: cfg.JWTKey, Issuer: cfg.JWTIssuer, Audience: cfg.JWTAudience}
	return []Stage{
		stage("security_headers", middleware.SecurityHeaders(cfg.Env)),
		stage("errors", middleware.Errors(errorHandler, deps.Logger)),
		stage("request_id", middleware.RequestID()),
		stage("request_timeout", middleware.RequestTimeout(cfg.RequestTimeout)),
		stage("proxy_headers", middleware.ProxyHeaders(cfg.TrustedProxyHops)),
		stage("cors", middleware.CORS(cfg.CORSAllowedOrigin)),
		stage("compress", middleware.Compress()),
		stage("rate_limit_global", exceptPath(healthPath, limiter.Window(middleware.PolicyGlobal, cfg.RateLimitGlobal, globalWindow))),
		// El webhook de Stripe (nv-60) llega de pocas IP de Stripe: solo cuenta en el límite global.
		stage("rate_limit_writes", exceptPath(middleware.StripeWebhookPath, limiter.Concurrent(middleware.PolicyWrites, cfg.RateLimitWrites))),
		stage("version", middleware.Version(versionado.NuevaPolitica(cfg.VersionApp.Minima, cfg.VersionApp.Mantenimiento))),
		stage("jwt", middleware.JWT(jwtCfg, deps.Redis)),
	}
}

// exceptPath salta el manejador en una ruta. /health no toca bases y lo consulta el balanceador
// cada pocos segundos: contarlo en Redis agotaría la cuota del plan gratuito (nv-11).
func exceptPath(path string, handler fiber.Handler) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if c.Path() == path {
			return c.Next()
		}
		return handler(c)
	}
}

func stage(name string, handler fiber.Handler) Stage {
	return Stage{Name: name, Handler: handler}
}

// NewApp construye la app con el pipeline y las rutas base. Roles se aplica por ruta
// con middleware.RequireRoles cuando un módulo lo necesite.
func NewApp(cfg config.Config, deps Deps) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:                 "wherever-api",
		DisableStartupMessage:   true,
		StrictRouting:           true,
		CaseSensitive:           true,
		BodyLimit:               cfg.BodyLimit,
		ReadTimeout:             cfg.RequestTimeout,
		WriteTimeout:            cfg.RequestTimeout,
		IdleTimeout:             idleTimeout,
		ErrorHandler:            middleware.NewErrorHandler(cfg.ExposeInternals(), deps.Logger),
		ProxyHeader:             fiber.HeaderXForwardedFor,
		EnableTrustedProxyCheck: true,
		TrustedProxies:          cfg.TrustedProxies,
	})

	limiter := middleware.NewLimiter(deps.Redis, deps.Logger)
	for _, s := range Pipeline(cfg, deps, limiter) {
		app.Use(s.Handler)
	}

	registerRoutes(app, cfg, deps, limiter)
	return app
}

func registerRoutes(app *fiber.App, cfg config.Config, deps Deps, limiter *middleware.Limiter) {
	health := newHealth(deps)
	app.Get(healthPath, health.live)
	app.Get("/ready", health.ready)

	v1 := app.Group("/api/v1")
	v1.Get("/ping", ping)
	for _, m := range deps.Modules {
		m.Routes(v1, limiter)
	}
	if cfg.ExposeInternals() {
		registerProbes(v1.Group("/probe"))
	}
	if cfg.IsDev() {
		registerDocs(app)
	}
}

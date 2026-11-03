// Command api arranca la API de Wherever.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
	_ "time/tzdata" // el trabajo nocturno usa America/Mexico_City aunque la imagen no traiga zonas

	goredis "github.com/redis/go-redis/v9"

	"github.com/WhereverApp2026/wherever-backend/internal/auth"
	"github.com/WhereverApp2026/wherever-backend/internal/ciudades"
	httpapi "github.com/WhereverApp2026/wherever-backend/internal/http"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/indexes"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/logger"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/mailer"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/neo4j"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/redis"
	"github.com/WhereverApp2026/wherever-backend/internal/privacidad"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios/account"
)

// version se fija al compilar: -ldflags "-X main.version=v0.1.0".
var version = "dev"

const shutdownTimeout = 10 * time.Second

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error al arrancar:", err)
		os.Exit(1)
	}
}

func run() error {
	if err := config.LoadDotEnv(".env"); err != nil {
		return err
	}
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	log := logger.New(os.Stdout, cfg.Env)

	mongoClient, err := mongo.New(cfg.MongoURI)
	if err != nil {
		return fmt.Errorf("mongodb: %w", err)
	}
	redisClient, err := redis.New(cfg.RedisAddr)
	if err != nil {
		return fmt.Errorf("redis: %w", err)
	}
	neo4jDriver, err := neo4j.New(cfg.Neo4jURI)
	if err != nil {
		return fmt.Errorf("neo4j: %w", err)
	}

	mail := buildMail(cfg.Mail, log)
	modules, job, err := buildModules(cfg, log, mongoClient, redisClient, mail)
	if err != nil {
		return err
	}
	stopJob, err := job.Start()
	if err != nil {
		return fmt.Errorf("trabajo nocturno: %w", err)
	}
	defer func() { _ = stopJob() }()

	app := httpapi.NewApp(cfg, httpapi.Deps{
		Logger:  log,
		Version: version,
		Redis:   redisClient,
		Mongo:   mongoClient,
		Neo4j:   neo4jDriver,
		// La API no crea índices (constitución §5); /ready da 503 si falta alguno.
		MongoIndexes: mongo.NewIndexVerifier(mongoClient.DB(), indexes.All()...),
		Modules:      modules,
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serveErr := make(chan error, 1)
	go func() { serveErr <- app.Listen(":" + strconv.Itoa(cfg.Port)) }()
	log.Info("API escuchando", "puerto", cfg.Port, "entorno", cfg.Env, "version", version)

	select {
	case err = <-serveErr:
	case <-ctx.Done():
		log.Info("apagando")
		err = app.ShutdownWithTimeout(shutdownTimeout)
	}

	closeCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	// Antes de cerrar las bases se vacía la cola de correo (con el mismo plazo).
	return errors.Join(err, mail.shutdown(closeCtx), mongoClient.Close(closeCtx), redisClient.Close(), neo4jDriver.Close(closeCtx))
}

// buildModules arma los módulos de negocio y el trabajo nocturno con sus dependencias.
func buildModules(cfg config.Config, log *slog.Logger, mongoClient *mongo.Client, redisClient *goredis.Client, mail mailers) ([]httpapi.Module, *privacidad.Job, error) {
	validator, err := validation.New()
	if err != nil {
		return nil, nil, err
	}
	db := mongoClient.DB()
	users := usuarios.NewRepository(db)
	authRepo := auth.NewRepository(db)
	authSvc, err := auth.NewService(users, authRepo, redisClient, mail.requests, auth.Config{
		JWT:        middleware.JWTConfig{Key: cfg.JWTKey, Issuer: cfg.JWTIssuer, Audience: cfg.JWTAudience},
		AccessTTL:  cfg.AccessTTL,
		RefreshTTL: cfg.RefreshTTL,
	}, log)
	if err != nil {
		return nil, nil, fmt.Errorf("auth: %w", err)
	}
	authSvc.WithSyncMailer(mail.sync)
	citySvc := ciudades.NewService(ciudades.NewRepository(db))
	exports := privacidad.NewExports(db)

	// ARCO: el documento del usuario se borra al final (si otro módulo falla, se reintenta).
	registry := privacidad.NewRegistry()
	for _, p := range []privacidad.Purger{auth.NewPrivacy(authRepo), exports, account.NewPrivacy(users)} {
		registry.RegisterPurger(p)
	}
	registry.RegisterExporter(account.NewPrivacy(users))
	registry.RegisterExporter(auth.NewPrivacy(authRepo))

	accountSvc, err := account.NewService(users, authSvc, citySvc, exports, registry,
		account.Config{PhoneHMACKey: cfg.PhoneHMACKey, GraceDays: cfg.ArcoGraceDays}, log)
	if err != nil {
		return nil, nil, fmt.Errorf("usuarios: %w", err)
	}
	// Con la cuenta en eliminación pendiente solo quedan GET /usuarios/yo y cancelar (nv-05).
	active := usuarios.RequireActive(users)
	modules := []httpapi.Module{
		auth.NewHandler(authSvc, validator).WithActiveGuard(active),
		account.NewHandler(accountSvc, validator).WithActiveGuard(active),
		ciudades.NewHandler(citySvc, validator),
	}
	return modules, privacidad.NewJob(redisClient, users, exports, registry, authSvc, log), nil
}

// mailers son los dos caminos del correo (nv-06): las peticiones encolan y nunca esperan al
// servidor; el trabajo nocturno envía directo porque necesita saber si el aviso salió.
type mailers struct {
	requests mailer.Mailer
	sync     mailer.Mailer
	shutdown func(context.Context) error
}

// buildMail arma el correo según la configuración. Sin SMTP_HOST (solo dev y staging; prod no
// arranca) se usa el Log, que registra tipo y destinatario enmascarado.
func buildMail(cfg config.Mail, log *slog.Logger) mailers {
	if !cfg.Enabled() {
		log.Warn("sin SMTP_HOST: los correos no se envían, solo se registran (tipo y destinatario enmascarado)")
		logMailer := mailer.NewLog(log)
		return mailers{requests: logMailer, sync: logMailer, shutdown: func(context.Context) error { return nil }}
	}
	smtpMailer := mailer.NewSMTP(mailer.SMTPConfig{
		Host: cfg.SMTPHost, Port: cfg.SMTPPort, User: cfg.SMTPUser, Password: cfg.SMTPPassword, TLSMode: cfg.SMTPTLS, From: cfg.From,
	}, mailer.NewLinks(cfg.LinkBase), log)
	direct := mailer.NewRetrying(smtpMailer, mailer.DefaultAttempts, mailer.DefaultRetryWait, cfg.Timeout)
	queue := mailer.NewAsync(direct, cfg.Workers, cfg.QueueSize, log)
	return mailers{requests: queue, sync: direct, shutdown: queue.Shutdown}
}

// Command api arranca la API de Wherever.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	httpapi "github.com/WhereverApp2026/wherever-backend/internal/http"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/logger"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/neo4j"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/redis"
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

	app := httpapi.NewApp(cfg, httpapi.Deps{
		Logger:  log,
		Version: version,
		Redis:   redisClient,
		Mongo:   mongoClient,
		Neo4j:   neo4jDriver,
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
	return errors.Join(err, mongoClient.Close(closeCtx), redisClient.Close(), neo4jDriver.Close(closeCtx))
}

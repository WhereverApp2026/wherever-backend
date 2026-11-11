// Command recalcular-embeddings rehace los vectores de significado de los negocios aprobados
// (spec nv-50). Por defecto solo los desactualizados; con -todos, todos. Lee la configuración
// del entorno o de .env, como la API. Imprime solo conteos.
//
//	go run ./scripts/recalcular-embeddings [-todos]
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/WhereverApp2026/wherever-backend/internal/embeddings"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/redis"
)

const timeout = 5 * time.Minute

func main() {
	todos := flag.Bool("todos", false, "recalcula todos, no solo los desactualizados")
	flag.Parse()
	if err := run(*todos); err != nil {
		fmt.Fprintln(os.Stderr, "recalcular-embeddings:", err)
		os.Exit(1)
	}
}

func run(todos bool) error {
	if err := config.LoadDotEnv(".env"); err != nil {
		return err
	}
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	mongoClient, err := mongo.New(cfg.MongoURI)
	if err != nil {
		return err
	}
	defer func() { _ = mongoClient.Close(context.Background()) }()
	redisClient, err := redis.New(cfg.RedisAddr)
	if err != nil {
		return err
	}
	defer func() { _ = redisClient.Close() }()

	rec, err := embeddings.Armar(cfg.IA, redisClient, mongoClient.DB(), "")
	if err != nil {
		return err
	}
	if rec == nil {
		return errors.New("faltan " + config.KeyGeminiAPIKey + " o " + config.KeyIAEmbeddingsID)
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	rep, err := rec.Recalcular(ctx, todos)
	fmt.Printf("recalcular-embeddings: visibles=%d calculados=%d borrados=%d\n", rep.Visibles, rep.Calculados, rep.Borrados)
	return err
}

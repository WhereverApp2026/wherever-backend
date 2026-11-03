// Command indexes crea los índices de MongoDB de todos los módulos (constitución §5:
// solo por script idempotente, nombres sin fechas). Lee MONGO_URI del entorno o de .env.
//
//	go run ./scripts/indexes
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/WhereverApp2026/wherever-backend/internal/indexes"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
)

const timeout = 30 * time.Second

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "indexes:", err)
		os.Exit(1)
	}
	fmt.Println("indexes: OK")
}

func run() error {
	if err := config.LoadDotEnv(".env"); err != nil {
		return err
	}
	uri := os.Getenv(config.KeyMongoURI)
	if uri == "" {
		return errors.New("falta " + config.KeyMongoURI)
	}
	client, err := mongo.New(uri)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	defer func() { _ = client.Close(context.Background()) }()
	return mongo.EnsureIndexes(ctx, client.DB(), indexes.All()...)
}

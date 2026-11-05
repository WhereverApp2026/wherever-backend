// Comando siembra crea el esquema de Neo4j y deja los datos de demostración (spec nv-11).
// Es idempotente y se corre a mano. Lee NEO4J_URI del entorno o de .env.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/neo4j"
)

const timeout = 2 * time.Minute

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "siembra:", err)
		os.Exit(1)
	}
}

func run() error {
	if err := config.LoadDotEnv(".env"); err != nil {
		return err
	}
	uri := os.Getenv(config.KeyNeo4jURI)
	if uri == "" {
		return errors.New("falta " + config.KeyNeo4jURI)
	}
	driver, err := neo4j.New(uri)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	defer func() { _ = driver.Close(context.Background()) }()

	if err := driver.EnsureSchema(ctx); err != nil {
		return err
	}
	if err := driver.SeedDemo(ctx); err != nil {
		return err
	}
	size, err := driver.GraphSize(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("siembra: OK, %d nodos y %d relaciones\n", size.Nodes, size.Relationships)
	return nil
}

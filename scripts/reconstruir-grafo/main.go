// Comando reconstruir-grafo borra las relaciones AMIGO_DE y FUE_A de Neo4j y las arma de
// nuevo desde MongoDB (spec nv-52). Es idempotente y se corre a mano. Lee MONGO_URI y
// NEO4J_URI del entorno o de .env.
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/grafo"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/neo4j"
)

const timeout = 10 * time.Minute

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "reconstruir-grafo:", err)
		os.Exit(1)
	}
}

func run() error {
	if err := config.LoadDotEnv(".env"); err != nil {
		return err
	}
	for _, key := range []string{config.KeyMongoURI, config.KeyNeo4jURI} {
		if os.Getenv(key) == "" {
			return fmt.Errorf("falta %s", key)
		}
	}
	client, err := mongo.New(os.Getenv(config.KeyMongoURI))
	if err != nil {
		return err
	}
	driver, err := neo4j.New(os.Getenv(config.KeyNeo4jURI))
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	defer func() {
		_ = client.Close(context.Background())
		_ = driver.Close(context.Background())
	}()

	if err := driver.EnsureSchema(ctx); err != nil {
		return err
	}
	db := client.DB()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	worker := grafo.NewWorker(grafo.NewQueue(db, log), amistades.NewRepository(db), planes.NewRepository(db), driver, log)
	users, err := worker.Rebuild(ctx, db)
	if err != nil {
		return err
	}
	size, err := driver.GraphSize(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("reconstruir-grafo: OK, %d usuarios; %d nodos y %d relaciones\n", users, size.Nodes, size.Relationships)
	if grafo.NearLimit(size) {
		fmt.Printf("reconstruir-grafo: aviso, pasa del %d %% del límite de Aura Free\n", grafo.WarnPercent)
	}
	return nil
}

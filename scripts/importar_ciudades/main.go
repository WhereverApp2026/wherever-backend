// Command importar_ciudades carga el catálogo de ciudades desde un CSV
// (`nombre,estado,latitud,longitud,con_cobertura`) o un JSON con esos campos. Es idempotente:
// repetir el archivo actualiza en vez de duplicar. Lee MONGO_URI del entorno o de .env.
//
//	go run ./scripts/importar_ciudades ciudades.csv
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/WhereverApp2026/wherever-backend/internal/ciudades"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
)

const timeout = 5 * time.Minute

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "importar_ciudades:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 1 {
		return errors.New("uso: go run ./scripts/importar_ciudades <archivo.csv|archivo.json>")
	}
	if err := config.LoadDotEnv(".env"); err != nil {
		return err
	}
	uri := os.Getenv(config.KeyMongoURI)
	if uri == "" {
		return errors.New("falta " + config.KeyMongoURI)
	}
	path := filepath.Clean(args[0])
	f, err := os.Open(path) // #nosec G304 G703 -- ruta que da quien corre el script a mano
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	client, err := mongo.New(uri)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	defer func() { _ = client.Close(context.Background()) }()

	n, err := ciudades.NewService(ciudades.NewRepository(client.DB())).Import(ctx, f, strings.TrimPrefix(filepath.Ext(path), "."))
	if err != nil {
		return err
	}
	fmt.Printf("importar_ciudades: %d ciudades cargadas\n", n)
	return nil
}

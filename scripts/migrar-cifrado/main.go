// Command migrar-cifrado cifra (o re-cifra con la llave activa) las alergias y la fecha de
// nacimiento de todas las cuentas (nv-49). Es idempotente. Sin `-aplicar` solo cuenta lo
// pendiente; también sirve para rotar la llave (poner la nueva primero en DATA_KEYS).
// Lee MONGO_URI y DATA_KEYS del entorno o de .env.
//
//	go run ./scripts/migrar-cifrado            # modo de prueba: solo cuenta
//	go run ./scripts/migrar-cifrado -aplicar   # cifra
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/WhereverApp2026/wherever-backend/internal/platform/cifrado"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

const timeout = 10 * time.Minute

func main() {
	apply := flag.Bool("aplicar", false, "cifra de verdad; sin él solo cuenta")
	flag.Parse()
	stats, err := run(*apply)
	if err != nil {
		fmt.Fprintln(os.Stderr, "migrar-cifrado:", err)
		os.Exit(1)
	}
	mode := "prueba (sin cambios)"
	if *apply {
		mode = "aplicado"
	}
	fmt.Printf("migrar-cifrado: %s; %d cuentas revisadas, %d pendientes, %d cifradas\n", mode, stats.Revisados, stats.Pendientes, stats.Cifrados)
}

func run(apply bool) (usuarios.MigrationStats, error) {
	if err := config.LoadDotEnv(".env"); err != nil {
		return usuarios.MigrationStats{}, err
	}
	uri, keys := os.Getenv(config.KeyMongoURI), os.Getenv(config.KeyDataKeys)
	if uri == "" || keys == "" {
		return usuarios.MigrationStats{}, errors.New("faltan " + config.KeyMongoURI + " o " + config.KeyDataKeys)
	}
	llavero, err := cifrado.CargarLlavero(keys)
	if err != nil {
		return usuarios.MigrationStats{}, err
	}
	client, err := mongo.New(uri)
	if err != nil {
		return usuarios.MigrationStats{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	defer func() { _ = client.Close(context.Background()) }()
	return usuarios.NewRepository(client.DB()).WithCipher(llavero).Reseal(ctx, apply)
}

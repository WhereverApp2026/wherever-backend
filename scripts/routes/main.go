// Command routes imprime las rutas que registra NewApp (método y ruta, una por línea).
// Lo usa scripts/check_endpoints.sh para compararlas con docs/endpoints.md.
// Construye la app en dev, el entorno con más rutas (sondas y docs), sin conectar bases.
package main

import (
	"fmt"
	"io"
	"log/slog"
	"sort"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/asistente"
	"github.com/WhereverApp2026/wherever-backend/internal/auth"
	"github.com/WhereverApp2026/wherever-backend/internal/busquedas"
	"github.com/WhereverApp2026/wherever-backend/internal/ciudades"
	"github.com/WhereverApp2026/wherever-backend/internal/clima"
	"github.com/WhereverApp2026/wherever-backend/internal/configuracion"
	httpapi "github.com/WhereverApp2026/wherever-backend/internal/http"
	"github.com/WhereverApp2026/wherever-backend/internal/inicio"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/personas"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/redis"
	"github.com/WhereverApp2026/wherever-backend/internal/recomendaciones"
	"github.com/WhereverApp2026/wherever-backend/internal/semantica"
	"github.com/WhereverApp2026/wherever-backend/internal/sugerencias"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios/account"
)

func main() {
	cfg := config.Config{
		Env:             config.EnvDev,
		BodyLimit:       config.MaxBodyLimit,
		RequestTimeout:  config.MaxRequestTimeout,
		RateLimitGlobal: config.DefaultRateGlobal,
		RateLimitWrites: config.DefaultRateWrites,
	}
	// El cliente no se conecta hasta usarse; aquí nunca se usa.
	rdb, err := redis.New("127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	// Los módulos se registran sin service: aquí solo importan sus rutas, nunca se atienden.
	modules := []httpapi.Module{auth.NewHandler(nil, nil), account.NewHandler(nil, nil), ciudades.NewHandler(nil, nil), configuracion.NewHandler(nil), semantica.NewHandler(nil, nil), busquedas.NewHandler(nil, nil), negocios.NewHandler(nil, nil), planes.NewHandler(nil, nil), asistente.NewHandler(nil, nil), recomendaciones.NewHandler(nil, nil), clima.NewHandler(nil, nil, nil), inicio.NewHandler(nil, nil), personas.NewHandler(nil), sugerencias.NewHandler(nil), amistades.NewHandler(nil, nil)}
	app := httpapi.NewApp(cfg, httpapi.Deps{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Redis: rdb, Modules: modules})

	var lines []string
	for _, route := range app.GetRoutes(true) {
		if route.Method == "HEAD" {
			continue // Fiber agrega HEAD a cada GET
		}
		lines = append(lines, route.Method+" "+route.Path)
	}
	sort.Strings(lines)
	for _, line := range lines {
		fmt.Println(line)
	}
}

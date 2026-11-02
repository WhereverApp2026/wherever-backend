package httpapi

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"

	platformredis "github.com/WhereverApp2026/wherever-backend/internal/platform/redis"
)

// Cada base tiene 1 s máximo en /ready (constitución §4).
const readyCheckTimeout = time.Second

// Estados de /ready y de cada dependencia.
const (
	statusOK       = "ok"
	statusDegraded = "degraded"
	statusError    = "error"
)

type healthResponse struct {
	Status  string `json:"estado"`
	Version string `json:"version"`
}

type dependencies struct {
	Mongo string `json:"mongodb"`
	Redis string `json:"redis"`
	Neo4j string `json:"neo4j"`
}

type readyResponse struct {
	Status       string       `json:"estado"`
	Dependencies dependencies `json:"dependencias"`
}

type health struct {
	version string
	log     *slog.Logger
	mongo   Pinger
	redis   Pinger
	neo4j   Pinger
}

func newHealth(deps Deps) *health {
	return &health{
		version: deps.Version,
		log:     deps.Logger,
		mongo:   deps.Mongo,
		redis:   platformredis.Pinger{Client: deps.Redis},
		neo4j:   deps.Neo4j,
	}
}

// live es liveness: 200 siempre y sin tocar bases.
func (h *health) live(c *fiber.Ctx) error {
	return c.JSON(healthResponse{Status: statusOK, Version: h.version})
}

// ready consulta las tres bases en paralelo. MongoDB o Neo4j caídos = 503; Redis caído =
// 200 `degraded`, porque es compartido y un 503 sacaría a todas las instancias a la vez.
func (h *health) ready(c *fiber.Ctx) error {
	var deps dependencies
	var wg sync.WaitGroup
	checks := []struct {
		name   string
		pinger Pinger
		result *string
	}{
		{"mongodb", h.mongo, &deps.Mongo},
		{"redis", h.redis, &deps.Redis},
		{"neo4j", h.neo4j, &deps.Neo4j},
	}
	// fiber.Ctx no es seguro entre goroutines: el contexto se toma antes.
	parent := c.UserContext()
	for _, check := range checks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			*check.result = h.check(parent, check.name, check.pinger)
		}()
	}
	wg.Wait()

	resp := readyResponse{Status: statusOK, Dependencies: deps}
	switch {
	case deps.Mongo != statusOK || deps.Neo4j != statusOK:
		resp.Status = statusError
		return c.Status(fiber.StatusServiceUnavailable).JSON(resp)
	case deps.Redis != statusOK:
		resp.Status = statusDegraded
	}
	return c.JSON(resp)
}

// check no expone el error en la respuesta; solo lo registra.
func (h *health) check(parent context.Context, name string, pinger Pinger) string {
	ctx, cancel := context.WithTimeout(parent, readyCheckTimeout)
	defer cancel()
	if err := pinger.Ping(ctx); err != nil {
		h.log.Warn("dependencia no disponible", "dependencia", name, "error", err.Error())
		return statusError
	}
	return statusOK
}

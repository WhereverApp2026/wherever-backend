package testutil

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Puertos fijos de prueba (AGENTS.md §7), distintos a los de docker-compose para no chocar.
const (
	MongoTestPort = "27018"
	RedisTestPort = "6380"
	Neo4jTestPort = "7688"

	startupTimeout = 3 * time.Minute
)

// Mismas versiones que deployments/docker-compose.yml.
const (
	mongoImage = "mongo:7.0.14"
	redisImage = "redis:7.4.1-alpine"
	neo4jImage = "neo4j:5.26.0-community"
)

// Stack son las tres bases reales levantadas para Integration.
type Stack struct {
	MongoURI  string
	RedisAddr string
	Neo4jURI  string
	stop      []func(context.Context) error
}

// StartStack levanta MongoDB, Redis y Neo4j con puertos fijos. Las suites con contenedor
// nunca corren en paralelo (`go test -p 1`).
func StartStack(ctx context.Context) (*Stack, error) {
	stack := &Stack{
		MongoURI:  "mongodb://127.0.0.1:" + MongoTestPort + "/wherever_test",
		RedisAddr: "127.0.0.1:" + RedisTestPort,
		Neo4jURI:  "neo4j://127.0.0.1:" + Neo4jTestPort,
	}
	specs := []struct {
		image    string
		port     string
		hostPort string
		env      map[string]string
		ready    *wait.LogStrategy
	}{
		{mongoImage, "27017", MongoTestPort, nil, wait.ForLog("Waiting for connections")},
		{redisImage, "6379", RedisTestPort, nil, wait.ForLog("Ready to accept connections")},
		{neo4jImage, "7687", Neo4jTestPort, map[string]string{"NEO4J_AUTH": "none"}, wait.ForLog("Bolt enabled")},
	}
	for _, spec := range specs {
		c, err := testcontainers.Run(ctx, spec.image,
			testcontainers.WithExposedPorts(spec.port+"/tcp"),
			testcontainers.WithEnv(spec.env),
			testcontainers.WithHostConfigModifier(fixedPort(spec.port, spec.hostPort)),
			testcontainers.WithWaitStrategy(spec.ready.WithStartupTimeout(startupTimeout)),
		)
		if err != nil {
			return nil, errors.Join(fmt.Errorf("%s: %w", spec.image, err), stack.Stop(ctx))
		}
		stack.stop = append(stack.stop, func(ctx context.Context) error { return c.Terminate(ctx) })
	}
	return stack, nil
}

// Stop apaga los contenedores levantados.
func (s *Stack) Stop(ctx context.Context) error {
	var errs []error
	for _, stop := range s.stop {
		errs = append(errs, stop(ctx))
	}
	return errors.Join(errs...)
}

func fixedPort(containerPort, hostPort string) func(*container.HostConfig) {
	return func(hc *container.HostConfig) {
		hc.PortBindings = network.PortMap{
			network.MustParsePort(containerPort + "/tcp"): {
				{HostIP: netip.MustParseAddr("127.0.0.1"), HostPort: hostPort},
			},
		}
	}
}

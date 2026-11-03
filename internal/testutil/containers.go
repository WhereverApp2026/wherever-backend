package testutil

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"runtime"
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
	// Mailpit (nv-06): SMTP y API web.
	MailpitSMTPTestPort = "1026"
	MailpitWebTestPort  = "8026"

	startupTimeout = 3 * time.Minute
)

// Mismas versiones que deployments/docker-compose.yml.
const (
	mongoImage   = "mongo:7.0.14"
	redisImage   = "redis:7.4.1-alpine"
	neo4jImage   = "neo4j:5.26.0-community"
	mailpitImage = "axllent/mailpit:v1.21.8"
)

// Stack son las tres bases reales y Mailpit (servidor de correo de prueba, nv-06).
type Stack struct {
	MongoURI  string
	RedisAddr string
	Neo4jURI  string
	Mailpit   *Mailpit
	stop      []func(context.Context) error
	redis     testcontainers.Container
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
		if spec.image == redisImage {
			stack.redis = c
		}
	}
	if err := stack.startMailpit(ctx); err != nil {
		return nil, errors.Join(err, stack.Stop(ctx))
	}
	return stack, nil
}

// startMailpit levanta Mailpit con sus dos puertos fijos (SMTP y API web).
func (s *Stack) startMailpit(ctx context.Context) error {
	c, err := testcontainers.Run(ctx, mailpitImage,
		testcontainers.WithExposedPorts("1025/tcp", "8025/tcp"),
		testcontainers.WithHostConfigModifier(func(hc *container.HostConfig) {
			hc.PortBindings = network.PortMap{
				network.MustParsePort("1025/tcp"): {{HostIP: netip.MustParseAddr("127.0.0.1"), HostPort: MailpitSMTPTestPort}},
				network.MustParsePort("8025/tcp"): {{HostIP: netip.MustParseAddr("127.0.0.1"), HostPort: MailpitWebTestPort}},
			}
		}),
		testcontainers.WithWaitStrategy(wait.ForLog("accessible via").WithStartupTimeout(startupTimeout)),
	)
	if err != nil {
		return fmt.Errorf("%s: %w", mailpitImage, err)
	}
	s.stop = append(s.stop, func(ctx context.Context) error { return c.Terminate(ctx) })
	s.Mailpit = &Mailpit{API: "http://127.0.0.1:" + MailpitWebTestPort, SMTPHost: "127.0.0.1", SMTPPort: MailpitSMTPTestPort}
	return nil
}

// StopRedis detiene Redis sin borrarlo, para probar la caída real.
func (s *Stack) StopRedis(ctx context.Context) error {
	timeout := 5 * time.Second
	return s.redis.Stop(ctx, &timeout)
}

// StartRedis vuelve a levantar Redis en el mismo puerto fijo (AGENTS.md §7), así el
// cliente existente se reconecta sin cambiar de dirección.
func (s *Stack) StartRedis(ctx context.Context) error {
	return s.redis.Start(ctx)
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

// RunWithStack es el TestMain de una suite con bases reales: levanta el Stack, llama a
// setup (para abrir clientes) y corre las pruebas; al terminar apaga todo. Ryuk no arranca
// en Docker Desktop para Windows, así que ahí se desactiva y el apagado queda en el defer.
func RunWithStack(m interface{ Run() int }, setup func(*Stack) (cleanup func(), err error)) int {
	if runtime.GOOS == "windows" && os.Getenv("TESTCONTAINERS_RYUK_DISABLED") == "" {
		_ = os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	stack, err := StartStack(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "no se pudieron levantar las bases:", err)
		return 1
	}
	defer func() { _ = stack.Stop(context.Background()) }()
	cleanup, err := setup(stack)
	if err != nil {
		fmt.Fprintln(os.Stderr, "preparar la suite:", err)
		return 1
	}
	defer cleanup()
	return m.Run()
}

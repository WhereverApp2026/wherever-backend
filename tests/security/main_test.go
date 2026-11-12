package security

import (
	"context"
	"errors"
	"os"
	"testing"

	goredis "github.com/redis/go-redis/v9"

	"github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/neo4j"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/redis"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
)

// Las pruebas de auth (nv-04) necesitan MongoDB y Redis reales; las de nv-01 siguen con
// bases caídas simuladas. Correr con `go test -p 1`: los puertos fijos no admiten paralelo.
var (
	mongoClient *mongo.Client
	redisClient *goredis.Client
	mailpit     *testutil.Mailpit
	neo4jDriver *neo4j.Driver // sugerencias y señal social (nv-53)
)

func TestMain(m *testing.M) {
	os.Exit(testutil.RunWithStack(m, func(s *testutil.Stack) (func(), error) {
		var err error
		mailpit = s.Mailpit
		if mongoClient, err = mongo.New(s.MongoURI); err != nil {
			return nil, err
		}
		if redisClient, err = redis.New(s.RedisAddr); err != nil {
			return nil, errors.Join(err, mongoClient.Close(context.Background()))
		}
		if neo4jDriver, err = neo4j.New(s.Neo4jURI); err != nil {
			return nil, errors.Join(err, mongoClient.Close(context.Background()), redisClient.Close())
		}
		return func() {
			_ = mongoClient.Close(context.Background())
			_ = redisClient.Close()
			_ = neo4jDriver.Close(context.Background())
		}, nil
	}))
}

// Package database prueba los índices de MongoDB contra una base real (constitución §7,
// suite Database): nombres, tipo, TTL y que scripts/indexes sea idempotente.
// Correr con `go test -p 1`: los puertos fijos no admiten suites en paralelo.
package database

import (
	"context"
	"os"
	"testing"

	"github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
)

var (
	mongoClient *mongo.Client
	neo4jURI    string
)

func TestMain(m *testing.M) {
	os.Exit(testutil.RunWithStack(m, func(s *testutil.Stack) (func(), error) {
		neo4jURI = s.Neo4jURI
		var err error
		if mongoClient, err = mongo.New(s.MongoURI); err != nil {
			return nil, err
		}
		return func() { _ = mongoClient.Close(context.Background()) }, nil
	}))
}

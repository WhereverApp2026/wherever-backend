package database

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/platform/neo4j"
)

// El grafo de demostración: 3 categorías, 3 lugares y 3 usuarios; cada lugar con su categoría,
// y cada usuario con un gusto y una visita.
const (
	demoNodes         = 9
	demoRelationships = 9
)

func TestGrafo_SiembraIdempotente(t *testing.T) {
	ctx := context.Background()
	driver, err := neo4j.New(neo4jURI)
	require.NoError(t, err)
	defer func() { _ = driver.Close(ctx) }()

	for range 2 {
		require.NoError(t, driver.EnsureSchema(ctx))
		require.NoError(t, driver.SeedDemo(ctx))
	}

	size, err := driver.GraphSize(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(demoNodes), size.Nodes, "correrla dos veces no duplica nodos")
	assert.Equal(t, int64(demoRelationships), size.Relationships, "ni relaciones")
}

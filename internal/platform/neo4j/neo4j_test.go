package neo4j

import (
	"testing"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitAuth_SeparaCredencialesDeLaURICifrada(t *testing.T) {
	raw := "neo4j+s://" + "usuario:" + "clave-de-prueba" + "@abc123.databases.neo4j.io"

	target, auth, err := splitAuth(raw)

	require.NoError(t, err)
	assert.Equal(t, "neo4j+s://abc123.databases.neo4j.io", target, "conserva el cifrado y quita las credenciales")
	assert.Equal(t, "usuario", auth.Tokens["principal"])
	assert.Equal(t, "clave-de-prueba", auth.Tokens["credentials"])
}

func TestSplitAuth_SinCredenciales(t *testing.T) {
	target, auth, err := splitAuth("neo4j://localhost:7687")

	require.NoError(t, err)
	assert.Equal(t, "neo4j://localhost:7687", target)
	assert.Equal(t, neo4j.NoAuth(), auth)
}

func TestNew_ElErrorNoRepiteLaClave(t *testing.T) {
	_, err := New("neo4j+s://" + "usuario:" + "clave-de-prueba" + "@host con espacio")

	require.Error(t, err)
	assert.NotContains(t, err.Error(), "clave-de-prueba")
}

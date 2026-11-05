package redis

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_URLCifradaActivaTLSYCredenciales(t *testing.T) {
	client, err := New("rediss://" + "default:" + "clave-de-prueba" + "@ejemplo.upstash.io:6379")
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	opts := client.Options()
	assert.Equal(t, "ejemplo.upstash.io:6379", opts.Addr)
	assert.Equal(t, "default", opts.Username)
	assert.Equal(t, "clave-de-prueba", opts.Password)
	require.NotNil(t, opts.TLSConfig, "rediss:// debe ir cifrado")
	assert.Equal(t, "ejemplo.upstash.io", opts.TLSConfig.ServerName)
	assert.Equal(t, dialTimeout, opts.DialTimeout, "los tiempos acotados aplican también a la URL")
	assert.True(t, opts.DisableIdentity)
}

func TestNew_DireccionSimpleSinTLS(t *testing.T) {
	client, err := New("localhost:6379")
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	assert.Nil(t, client.Options().TLSConfig)
}

func TestNew_ElErrorNoRepiteLaClave(t *testing.T) {
	_, err := New("rediss://" + "default:" + "clave-de-prueba" + "@host con espacio")

	require.Error(t, err)
	assert.NotContains(t, err.Error(), "clave-de-prueba")
}

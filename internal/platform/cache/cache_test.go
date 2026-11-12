package cache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	platformredis "github.com/WhereverApp2026/wherever-backend/internal/platform/redis"
)

// redisCaido apunta a un puerto sin servidor: cada comando falla rápido.
func redisCaido(t *testing.T) *Cache {
	t.Helper()
	rdb, err := platformredis.New("127.0.0.1:1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = rdb.Close() })
	return New(rdb, nil)
}

func TestCache_Clave(t *testing.T) {
	assert.Equal(t, "cache:negocio:abc", Clave("negocio", "abc"))
}

func TestCache_NilLlamaAlOrigen(t *testing.T) {
	var c *Cache
	v, err := Obtener(context.Background(), c, "cache:x:1", time.Minute, func(context.Context) (int, error) { return 7, nil })
	require.NoError(t, err)
	assert.Equal(t, 7, v)
	c.Invalidar(context.Background(), "cache:x:1")
}

func TestCache_RedisCaidoRespondeElOrigenYSeApaga(t *testing.T) {
	c := redisCaido(t)
	llamadas := 0
	for range 3 {
		v, err := Obtener(context.Background(), c, "cache:x:1", time.Minute, func(context.Context) (string, error) {
			llamadas++
			return "origen", nil
		})
		require.NoError(t, err)
		assert.Equal(t, "origen", v)
	}
	assert.Equal(t, 3, llamadas)
	assert.True(t, c.apagada(), "tras el error no vuelve a llamar a Redis durante la pausa")
	c.Invalidar(context.Background(), "cache:x:1")
}

func TestCache_SeEnciendeTrasLaPausa(t *testing.T) {
	c := redisCaido(t)
	ahora := time.Unix(1000, 0)
	c.now = func() time.Time { return ahora }
	c.apagar(errors.New("caída"))
	assert.True(t, c.apagada())
	ahora = ahora.Add(Pausa)
	assert.False(t, c.apagada())
}

func TestCache_ErrorDelOrigenNoSeGuarda(t *testing.T) {
	c := redisCaido(t)
	falla := errors.New("mongo caído")
	_, err := Obtener(context.Background(), c, "cache:x:1", time.Minute, func(context.Context) (int, error) { return 0, falla })
	assert.ErrorIs(t, err, falla)
}

func TestCache_AvalanchaUnaSolaConsulta(t *testing.T) {
	c := redisCaido(t)
	var llamadas atomic.Int32
	soltar := make(chan struct{})
	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			v, err := Obtener(context.Background(), c, "cache:x:avalancha", time.Minute, func(context.Context) (int, error) {
				llamadas.Add(1)
				<-soltar
				return 42, nil
			})
			assert.NoError(t, err)
			assert.Equal(t, 42, v)
		})
	}
	time.Sleep(200 * time.Millisecond)
	close(soltar)
	wg.Wait()
	assert.Equal(t, int32(1), llamadas.Load())
}

package etag

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestETag_DebilYEstable(t *testing.T) {
	a := De([]byte(`{"x":1}`))
	assert.Regexp(t, `^W/"[0-9a-f]{32}"$`, a)
	assert.Equal(t, a, De([]byte(`{"x":1}`)))
}

func TestETag_CambiaConElCuerpo(t *testing.T) {
	assert.NotEqual(t, De([]byte(`{"x":1}`)), De([]byte(`{"x":2}`)))
}

func TestETag_CoincideConLista(t *testing.T) {
	e := De([]byte("a"))
	assert.True(t, Coincide(`W/"otro", `+e, e))
	assert.False(t, Coincide(`W/"otro", "mas"`, e))
	assert.False(t, Coincide("", e))
}

func TestETag_CoincideConAsterisco(t *testing.T) {
	assert.True(t, Coincide("*", De([]byte("a"))))
}

func TestETag_CoincideSinPrefijoDebil(t *testing.T) {
	e := De([]byte("a"))
	assert.True(t, Coincide(e[len("W/"):], e))
}

package versionado

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionado_ParseValidas(t *testing.T) {
	for _, raw := range []string{"1.2.3", "v1.2.3"} {
		v, err := Parse(raw)
		require.NoError(t, err, raw)
		assert.Equal(t, "1.2.3", v.String())
	}
}

func TestVersionado_ParseInvalidas(t *testing.T) {
	for _, raw := range []string{"1.2", "1.2.3.4", "1.2.x", "", "1.2.3-beta", "1.2.3+7", "1.-2.3", "vv1.2.3"} {
		_, err := Parse(raw)
		assert.ErrorIs(t, err, ErrFormato, raw)
	}
}

func TestVersionado_ComparaNumerico(t *testing.T) {
	a, _ := Parse("1.10.0")
	b, _ := Parse("1.9.0")
	assert.True(t, b.MenorQue(a))
	assert.False(t, a.MenorQue(b))
	assert.False(t, a.MenorQue(a), "iguales no son menores")
	c, _ := Parse("1.9.1")
	assert.True(t, b.MenorQue(c))
}

func TestVersionado_Decidir(t *testing.T) {
	minima, _ := Parse("1.4.0")
	p := NuevaPolitica(minima, false)
	cases := []struct {
		ruta, cabecera string
		want           Decision
	}{
		{"/api/v1/planes", "1.3.9", Actualizar},
		{"/api/v1/planes", "1.4.0", Seguir},
		{"/api/v1/planes", "2.0.0", Seguir},
		{"/api/v1/planes", "", Seguir},
		{"/api/v1/planes", "basura", Seguir},
		{"/api/v1/auth/login", "1.0.0", Seguir},
		{"/api/v1/configuracion", "1.0.0", Seguir},
		{"/api/v1/ping", "1.0.0", Seguir},
		{"/health", "1.0.0", Seguir},
		{"/planes", "1.0.0", Seguir},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, p.Decidir(c.ruta, c.cabecera), c.ruta+" "+c.cabecera)
	}
}

func TestVersionado_MantenimientoMandaSobre426(t *testing.T) {
	minima, _ := Parse("1.4.0")
	p := NuevaPolitica(minima, true)
	assert.Equal(t, Mantenimiento, p.Decidir("/api/v1/planes", "1.0.0"))
	assert.Equal(t, Mantenimiento, p.Decidir("/api/v1/planes", ""))
	assert.Equal(t, Seguir, p.Decidir("/api/v1/configuracion", "1.0.0"))
	assert.Equal(t, Seguir, p.Decidir("/api/v1/auth/logout", ""))
}

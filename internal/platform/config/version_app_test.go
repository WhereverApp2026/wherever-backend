package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
)

func TestConfiguracion_Defectos(t *testing.T) {
	cfg, err := config.Load(testutil.Env(nil))
	require.NoError(t, err)
	v := cfg.VersionApp
	assert.Equal(t, "0.0.0", v.Minima.String())
	assert.Equal(t, "1.0.0", v.Actual.String())
	assert.Equal(t, "2026-10", v.Terminos)
	assert.Equal(t, "2026-11", v.Aviso)
	assert.False(t, v.Mantenimiento)
}

func TestConfiguracion_ValoresExplicitos(t *testing.T) {
	cfg, err := config.Load(testutil.Env(map[string]string{
		config.KeyAppVersionMinima: "v1.4.0", config.KeyAppVersionActual: "1.6.2",
		config.KeyTerminosVersion: "2026-12", config.KeyAvisoVersion: "2027-01", config.KeyMantenimiento: "TRUE",
	}))
	require.NoError(t, err)
	v := cfg.VersionApp
	assert.Equal(t, "1.4.0", v.Minima.String())
	assert.Equal(t, "1.6.2", v.Actual.String())
	assert.Equal(t, "2026-12", v.Terminos)
	assert.Equal(t, "2027-01", v.Aviso)
	assert.True(t, v.Mantenimiento)
}

func TestConfiguracion_SemverInvalidoFalla(t *testing.T) {
	for _, key := range []string{config.KeyAppVersionMinima, config.KeyAppVersionActual} {
		_, err := config.Load(testutil.Env(map[string]string{key: "1.2"}))
		require.Error(t, err)
		assert.Contains(t, err.Error(), key)
	}
}

func TestConfiguracion_MinimaMayorQueActualFalla(t *testing.T) {
	_, err := config.Load(testutil.Env(map[string]string{config.KeyAppVersionMinima: "1.10.0", config.KeyAppVersionActual: "1.9.0"}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), config.KeyAppVersionMinima)
}

func TestConfiguracion_MantenimientoInvalidoFalla(t *testing.T) {
	_, err := config.Load(testutil.Env(map[string]string{config.KeyMantenimiento: "si"}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), config.KeyMantenimiento)
}

func TestConfiguracion_TerminosVacioUsaDefecto(t *testing.T) {
	cfg, err := config.Load(testutil.Env(map[string]string{config.KeyTerminosVersion: "  "}))
	require.NoError(t, err)
	assert.Equal(t, config.DefaultTerminosVersion, cfg.VersionApp.Terminos)

	_, err = config.Load(testutil.Env(map[string]string{config.KeyAvisoVersion: "con espacio"}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), config.KeyAvisoVersion)
}

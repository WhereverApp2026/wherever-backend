package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
)

func TestLoad_JobHour(t *testing.T) {
	cfg, err := config.Load(testutil.Env(nil))
	require.NoError(t, err)
	assert.Equal(t, config.DefaultJobHour, cfg.JobHour)

	cfg, err = config.Load(testutil.Env(map[string]string{config.KeyJobHour: "21"}))
	require.NoError(t, err)
	assert.Equal(t, 21, cfg.JobHour)

	for _, bad := range []string{"-1", "24", "noche"} {
		_, err := config.Load(testutil.Env(map[string]string{config.KeyJobHour: bad}))
		require.Error(t, err, bad)
		assert.Contains(t, err.Error(), config.KeyJobHour)
	}
}

func TestConfig_ExposeInternals(t *testing.T) {
	for name, tc := range map[string]struct {
		env  string
		hide bool
		want bool
	}{
		"dev":                    {config.EnvDev, false, true},
		"staging":                {config.EnvStaging, false, true},
		"staging público":        {config.EnvStaging, true, false},
		"prod":                   {config.EnvProd, false, false},
		"prod, con el indicador": {config.EnvProd, true, false},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, config.Config{Env: tc.env, HideInternals: tc.hide}.ExposeInternals())
		})
	}
}

func TestLoad_HideInternals(t *testing.T) {
	cfg, err := config.Load(testutil.Env(map[string]string{config.KeyHideInternals: "true"}))
	require.NoError(t, err)
	assert.True(t, cfg.HideInternals)

	cfg, err = config.Load(testutil.Env(nil))
	require.NoError(t, err)
	assert.False(t, cfg.HideInternals)
}

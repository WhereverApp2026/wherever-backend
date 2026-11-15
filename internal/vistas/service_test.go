package vistas

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

func TestDTO_Validacion(t *testing.T) {
	v, err := validation.New()
	require.NoError(t, err)
	assert.NoError(t, v.Struct(VistaRequest{}))
	assert.NoError(t, v.Struct(VistaRequest{Origen: "mapa"}))
	assert.Error(t, v.Struct(VistaRequest{Origen: "otro"}))
	assert.NoError(t, v.Struct(RecientesQuery{Limit: DefaultLimit}))
	assert.Error(t, v.Struct(RecientesQuery{Limit: MaxLimit + 1}))
	assert.Error(t, v.Struct(RecientesQuery{Limit: 0}))
}

func TestLimites_DeLaSpec(t *testing.T) {
	assert.Equal(t, 60, MaxPerHour)
	assert.Equal(t, 10, int(MinGap.Minutes()))
	assert.Equal(t, 180, int(Retention.Hours()/24))
}

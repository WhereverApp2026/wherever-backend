package cupones

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
)

func TestEnHorarioDeAviso_DeDiezALasVeintidos(t *testing.T) {
	at := func(h, m int) time.Time { return time.Date(2026, 11, 23, h, m, 0, 0, negocios.MexicoCity()) }
	assert.False(t, EnHorarioDeAviso(at(9, 59)))
	assert.True(t, EnHorarioDeAviso(at(10, 0)))
	assert.True(t, EnHorarioDeAviso(at(21, 59)))
	assert.False(t, EnHorarioDeAviso(at(22, 0)))
	assert.False(t, EnHorarioDeAviso(at(3, 0).UTC()), "la hora se mide en México aunque llegue en UTC")
}

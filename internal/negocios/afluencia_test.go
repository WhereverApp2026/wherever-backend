package negocios

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDay_RangoYHoy(t *testing.T) {
	// 2026-11-15 18:00 UTC es domingo en México.
	now := time.Date(2026, 11, 15, 18, 0, 0, 0, time.UTC)
	d, err := ParseDay("", now)
	require.NoError(t, err)
	assert.Equal(t, 0, d)
	for _, ok := range []string{"0", "3", "6"} {
		_, err := ParseDay(ok, now)
		assert.NoError(t, err, ok)
	}
	for _, bad := range []string{"7", "-1", "10", "a"} {
		_, err := ParseDay(bad, now)
		assert.Error(t, err, bad)
	}
}

func TestTrafficOf_Suficiente(t *testing.T) {
	now := time.Date(2026, 11, 15, 18, 0, 0, 0, time.UTC) // domingo 12:00 en México
	matrix := make([][]int, 7)
	for d := range matrix {
		matrix[d] = make([]int, 24)
	}
	matrix[0][12] = 80
	got := TrafficOf(matrix, 0, now)
	assert.True(t, got.Enough)
	assert.Equal(t, 80, got.Hours[12])
	require.NotNil(t, got.CurrentHour)
	assert.Equal(t, 12, *got.CurrentHour)

	monday := TrafficOf(matrix, 1, now)
	assert.False(t, monday.Enough, "día en ceros")
	assert.Nil(t, monday.CurrentHour, "hora_actual solo hoy")
	assert.Len(t, monday.Hours, 24)

	missing := TrafficOf(nil, 0, now)
	assert.False(t, missing.Enough)
	assert.Len(t, missing.Hours, 24)
	bad := TrafficOf([][]int{{1}}, 0, now)
	assert.False(t, bad.Enough, "no es 7 × 24")
}

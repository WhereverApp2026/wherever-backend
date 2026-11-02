package apperr

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Los 11 errores tipados de la constitución §5 y su código HTTP.
func TestConstructors_MapToConstitutionStatus(t *testing.T) {
	cases := map[int]*Error{
		http.StatusBadRequest:          BadRequest("x"),
		http.StatusUnauthorized:        Unauthorized("x"),
		http.StatusForbidden:           Forbidden("x"),
		http.StatusNotFound:            NotFound("x"),
		http.StatusRequestTimeout:      Timeout("x"),
		http.StatusConflict:            Conflict("x"),
		http.StatusGone:                Gone("x"),
		http.StatusUnprocessableEntity: Validation("x"),
		http.StatusLocked:              Locked("x"),
		http.StatusTooManyRequests:     RateLimited(30),
		http.StatusServiceUnavailable:  Unavailable("x"),
	}
	for status, err := range cases {
		assert.Equal(t, status, err.Kind.Status(), err.Kind.Code())
		kind, ok := KindFromStatus(status)
		assert.True(t, ok)
		assert.Equal(t, err.Kind, kind)
	}
}

func TestUnknownKind_IsInternal(t *testing.T) {
	unknown := Kind(99)

	assert.Equal(t, http.StatusInternalServerError, unknown.Status())
	assert.Equal(t, KindInternal.Code(), unknown.Code())
	assert.Equal(t, KindInternal.DefaultMessage(), unknown.DefaultMessage())
	_, ok := KindFromStatus(http.StatusTeapot)
	assert.False(t, ok)
}

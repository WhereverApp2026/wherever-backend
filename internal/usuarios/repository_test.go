package usuarios

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFriendCode_EightUnambiguousChars(t *testing.T) {
	seen := map[string]bool{}
	for range 300 {
		code, err := newFriendCode()
		require.NoError(t, err)
		assert.Len(t, code, friendCodeLen)
		for _, r := range code {
			assert.True(t, strings.ContainsRune(friendCodeAlphabet, r), "carácter %q fuera del alfabeto", r)
		}
		assert.NotContains(t, code, "0")
		assert.NotContains(t, code, "O")
		seen[code] = true
	}
	assert.Greater(t, len(seen), 295, "con 31^8 combinaciones casi no debe repetirse")
}

func TestCanSignIn(t *testing.T) {
	assert.True(t, User{Status: StatusActive}.CanSignIn())
	assert.True(t, User{Status: StatusPendingDeletion}.CanSignIn(), "entra para cancelar el borrado")
	assert.False(t, User{Status: StatusBlocked}.CanSignIn())
	assert.False(t, User{}.CanSignIn())
}

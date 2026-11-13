package notificaciones

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

func TestDTO_Validacion(t *testing.T) {
	v, err := validation.New()
	require.NoError(t, err)
	token := strings.Repeat("a", 152)
	yes := true
	cases := []struct {
		name string
		dto  any
		ok   bool
	}{
		{"registrar", DeviceRequest{Token: token}, true},
		{"registrar android", DeviceRequest{Token: token, Platform: "android"}, true},
		{"plataforma ios", DeviceRequest{Token: token, Platform: "ios"}, false},
		{"token corto", DeviceRequest{Token: "abc"}, false},
		{"quitar sin token", DeviceRemoveRequest{}, false},
		{"marcar", MarkRequest{Read: &yes}, true},
		{"marcar sin valor", MarkRequest{}, false},
		{"lista por defecto", NewListQuery(), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := v.Struct(c.dto)
			if c.ok {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestNewItem_LeidaYDatosVacios(t *testing.T) {
	now := time.Now()
	n := Notification{ID: bson.NewObjectID(), Kind: KindFriendRequest, CreatedAt: now}
	item := newItem(n)
	assert.False(t, item.Read)
	assert.NotNil(t, item.Data, "datos siempre es un objeto")
	n.ReadAt = &now
	assert.True(t, newItem(n).Read)
}

func TestHashToken(t *testing.T) {
	h := HashToken("token-de-prueba-largo")
	assert.Len(t, h, 64)
	assert.NotContains(t, h, "token")
	assert.Equal(t, h, HashToken("token-de-prueba-largo"))
}

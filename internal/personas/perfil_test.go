package personas

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

func TestNormalizeContacts_DedupeEIndice(t *testing.T) {
	key := []byte("llave-de-prueba")
	hashes, first := normalizeContacts(key, []string{"basura", "55 1234 5678", "+525512345678", "(55) 1234-5678", "+1 202 555 0143", "123"})
	require.Len(t, hashes, 2, "inválidos fuera y el mismo número una vez")
	assert.Equal(t, 1, first[usuarios.HashPhone(key, "+525512345678")], "el primero donde apareció")
	assert.Equal(t, 4, first[usuarios.HashPhone(key, "+12025550143")])
	none, _ := normalizeContacts(key, []string{"hola", ""})
	assert.Empty(t, none)
}

func TestProfileResponse_FormaSegunRelacion(t *testing.T) {
	basic, err := json.Marshal(ProfileResponse{Person: Person{ID: "1", Name: "Ana"}, Relation: RelationNone})
	require.NoError(t, err)
	for _, hidden := range []string{"nivel", "amigos_en_comun", "grupos_en_comun", "silenciado", "degradado"} {
		assert.NotContains(t, string(basic), hidden, "un no amigo no ve "+hidden)
	}
	friend, err := json.Marshal(ProfileResponse{Person: Person{ID: "1"}, Relation: RelationFriend, FriendExtras: &FriendExtras{Level: "explorador", Degraded: true}})
	require.NoError(t, err)
	assert.Contains(t, string(friend), `"amigos_en_comun":null`, "degradado = null")
	assert.Contains(t, string(friend), `"degradado":true`)
	assert.Contains(t, string(friend), `"nivel":"explorador"`)
}

func TestContactsRequest_Validacion(t *testing.T) {
	v, err := validation.New()
	require.NoError(t, err)
	assert.Error(t, v.Struct(ContactsRequest{}))
	assert.Error(t, v.Struct(ContactsRequest{Phones: make([]string, MaxContacts+1)}))
	assert.NoError(t, v.Struct(ContactsRequest{Phones: []string{"5512345678"}}))
}

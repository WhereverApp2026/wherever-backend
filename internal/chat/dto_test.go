package chat

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

func ptr(s string) *string { return &s }

func TestDTO_Validacion(t *testing.T) {
	v, err := validation.New()
	require.NoError(t, err)
	uuid := "3f2b8c1e-5d4a-4b6f-9a7e-1c2d3e4f5a6b"
	cases := []struct {
		name string
		dto  any
		ok   bool
	}{
		{"mensaje válido", SendRequest{Text: "hola", ClientID: uuid}, true},
		{"mensaje de 1000", SendRequest{Text: strings.Repeat("ñ", 1000), ClientID: uuid}, true},
		{"mensaje de 1001", SendRequest{Text: strings.Repeat("a", 1001), ClientID: uuid}, false},
		{"sin texto", SendRequest{ClientID: uuid}, false},
		{"sin id_cliente", SendRequest{Text: "hola"}, false},
		{"id_cliente no UUID", SendRequest{Text: "hola", ClientID: "abc"}, false},
		{"reacción de la lista", ReactRequest{Emoji: ptr("🎉")}, true},
		{"quitar reacción", ReactRequest{}, true},
		{"emoji fuera de la lista", ReactRequest{Emoji: ptr("🍕")}, false},
		{"texto libre como emoji", ReactRequest{Emoji: ptr("hola")}, false},
		{"lista por defecto", NewListQuery(), true},
		{"límite 100", ListQuery{Limit: 100}, true},
		{"límite 101", ListQuery{Limit: 101}, false},
		{"búsqueda de 1", ListQuery{Limit: 30, Query: "a"}, false},
		{"búsqueda de 101", ListQuery{Limit: 30, Query: strings.Repeat("a", 101)}, false},
		{"despues_de válido", ListQuery{Limit: 30, After: bson.NewObjectID().Hex()}, true},
		{"despues_de mal formado", ListQuery{Limit: 30, After: "xyz"}, false},
		{"cursor manipulado", ListQuery{Limit: 30, Cursor: "no-es-cursor"}, false},
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

// La lista del DTO (oneof) y Emojis no deben separarse.
func TestEmojis_DTOyListaIguales(t *testing.T) {
	field, ok := reflect.TypeOf(ReactRequest{}).FieldByName("Emoji")
	require.True(t, ok)
	tag := field.Tag.Get("validate")
	assert.Equal(t, "omitempty,oneof="+strings.Join(Emojis, " "), tag)
	for _, e := range Emojis {
		assert.True(t, IsEmoji(e))
	}
	assert.False(t, IsEmoji(""))
}

func TestCleanText(t *testing.T) {
	got, err := CleanText("  hola  ")
	require.NoError(t, err)
	assert.Equal(t, "hola", got)
	_, err = CleanText("   ")
	assert.Error(t, err, "solo espacios")
	_, err = CleanText(strings.Repeat("é", MaxText+1))
	assert.Error(t, err)
}

func TestPreview_RecortaSinPartirCaracteres(t *testing.T) {
	assert.Equal(t, "corto", Preview("corto"))
	long := strings.Repeat("ñ", 100)
	got := Preview(long)
	assert.Equal(t, PreviewRunes, len([]rune(got)))
	assert.True(t, strings.HasPrefix(long, got))
}

func TestNewView_ReaccionesYBorrado(t *testing.T) {
	me, other, third := bson.NewObjectID(), bson.NewObjectID(), bson.NewObjectID()
	author := usuarios.User{ID: other, Name: "Ana", PhotoURL: "https://img/x.jpg"}
	m := Message{
		ID: bson.NewObjectID(), AuthorID: &other, Kind: KindText, Text: "hola", CreatedAt: time.Now(),
		Reactions: []Reaction{{UserID: me, Emoji: "🎉"}, {UserID: third, Emoji: "🎉"}, {UserID: other, Emoji: "👍"}},
	}
	v := newView(m, me, map[bson.ObjectID]usuarios.User{other: author})
	require.NotNil(t, v.Author)
	assert.Equal(t, "Ana", v.Author.Name)
	assert.Equal(t, []ReactionView{{Emoji: "👍", Total: 1}, {Emoji: "🎉", Total: 2, Mine: true}}, v.Reactions)

	// Autor inactivo: sin persona. Borrado: sin texto ni reacciones.
	now := time.Now()
	m.DeletedAt = &now
	v = newView(m, me, nil)
	assert.Nil(t, v.Author)
	assert.True(t, v.Deleted)
	assert.Empty(t, v.Text)
	assert.Empty(t, v.Reactions)
}

func TestView_JSONSnakeCase(t *testing.T) {
	raw, err := json.Marshal(View{ID: "1", Kind: KindSystem, Reactions: []ReactionView{{Emoji: "👍", Total: 1, Mine: true}}})
	require.NoError(t, err)
	s := string(raw)
	for _, k := range []string{`"id"`, `"tipo"`, `"texto"`, `"reacciones"`, `"mia"`, `"total"`, `"eliminado"`, `"creado_en"`} {
		assert.Contains(t, s, k)
	}
	assert.NotContains(t, s, `"autor"`, "un aviso del sistema no tiene autor")
}

func TestFromCursor(t *testing.T) {
	id := bson.NewObjectID()
	p, err := fromCursor(paginationCursor(id.Hex(), "antes"), Page{Limit: 10})
	require.NoError(t, err)
	require.NotNil(t, p.Before)
	assert.Nil(t, p.After)
	p, err = fromCursor(paginationCursor(id.Hex(), "despues"), Page{Limit: 10})
	require.NoError(t, err)
	require.NotNil(t, p.After)
	_, err = fromCursor(paginationCursor(id.Hex(), "otro"), Page{})
	assert.Error(t, err)
	_, err = fromCursor(paginationCursor("xyz", "antes"), Page{})
	assert.Error(t, err)
}

func paginationCursor(id, sort string) pagination.Cursor {
	return pagination.Cursor{ID: id, Sort: sort}
}

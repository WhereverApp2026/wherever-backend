package chat

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

func TestDirectos_DTOValidacion(t *testing.T) {
	v, err := validation.New()
	require.NoError(t, err)
	uuid := "3f2b8c1e-5d4a-4b6f-9a7e-1c2d3e4f5a6b"
	id := bson.NewObjectID().Hex()
	cases := []struct {
		name string
		dto  any
		ok   bool
	}{
		{"abrir válido", OpenRequest{UserID: id}, true},
		{"abrir sin persona", OpenRequest{}, false},
		{"abrir con id corto", OpenRequest{UserID: "abc"}, false},
		{"mensaje sin adjunto", DirectSendRequest{Text: "hola", ClientID: uuid}, true},
		{"mensaje con plan", DirectSendRequest{Text: "vamos", ClientID: uuid, Attachment: &AttachmentRequest{Kind: AttachPlan, ID: id}}, true},
		{"mensaje con lugar", DirectSendRequest{Text: "mira", ClientID: uuid, Attachment: &AttachmentRequest{Kind: AttachPlace, ID: id}}, true},
		{"respuesta a historia", DirectSendRequest{Text: "¡qué bien!", ClientID: uuid, Attachment: &AttachmentRequest{Kind: AttachMoment, ID: id}}, true},
		{"adjunto de otro tipo", DirectSendRequest{Text: "hola", ClientID: uuid, Attachment: &AttachmentRequest{Kind: "audio", ID: id}}, false},
		{"adjunto sin id", DirectSendRequest{Text: "hola", ClientID: uuid, Attachment: &AttachmentRequest{Kind: AttachPlan}}, false},
		{"mensaje de 1001", DirectSendRequest{Text: strings.Repeat("a", 1001), ClientID: uuid}, false},
		{"mensaje sin id_cliente", DirectSendRequest{Text: "hola"}, false},
		{"lista por defecto", NewDirectListQuery(), true},
		{"lista con límite 0", DirectListQuery{}, false},
		{"despues_de no hex", DirectListQuery{Limit: 10, After: strings.Repeat("z", 24)}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.ok {
				assert.NoError(t, v.Struct(tc.dto))
			} else {
				assert.Error(t, v.Struct(tc.dto))
			}
		})
	}
}

// Un mensaje de grupo no guarda `conversacion_id` y uno directo no guarda `grupo_id`: los
// índices y filtros de cada tipo dependen de que el otro campo no exista.
func TestDirectos_MensajeSoloUnaConversacion(t *testing.T) {
	group, err := bson.Marshal(Message{ID: bson.NewObjectID(), GroupID: bson.NewObjectID(), Kind: KindText, Text: "hola"})
	require.NoError(t, err)
	_, err = bson.Raw(group).LookupErr(fieldDirect)
	assert.Error(t, err)
	direct, err := bson.Marshal(Message{ID: bson.NewObjectID(), ConversationID: bson.NewObjectID(), Kind: KindText, Text: "hola"})
	require.NoError(t, err)
	_, err = bson.Raw(direct).LookupErr(fieldGroup)
	assert.Error(t, err)
	_, err = bson.Raw(direct).LookupErr("adjunto")
	assert.Error(t, err, "sin adjunto no se guarda el campo")
}

func TestDirectos_ParYOtro(t *testing.T) {
	a, b := bson.NewObjectID(), bson.NewObjectID()
	assert.Equal(t, ConversationPair(a, b), ConversationPair(b, a))
	assert.NotEqual(t, ConversationPair(a, b), ConversationPair(a, bson.NewObjectID()))
	read := bson.NewObjectID()
	c := Conversation{Participants: []bson.ObjectID{a, b}, ReadUpTo: map[string]bson.ObjectID{a.Hex(): read}}
	assert.Equal(t, b, c.Other(a))
	assert.Equal(t, a, c.Other(b))
	assert.Equal(t, read, c.ReadBy(a))
	assert.True(t, c.ReadBy(b).IsZero())
}

func TestDirectos_VistaDelMensaje(t *testing.T) {
	me, other := bson.NewObjectID(), bson.NewObjectID()
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	plan := bson.NewObjectID()
	m := Message{ID: bson.NewObjectID(), AuthorID: &me, Text: "vamos", Attachment: &Attachment{Kind: AttachPlan, ID: plan, Name: "Tarde de tacos"},
		Reactions: []Reaction{{UserID: other, Emoji: "🎉"}}, CreatedAt: now}
	v := newDirectView(m, me, map[bson.ObjectID]string{m.ID: InviteAccepted})
	require.NotNil(t, v.Attachment)
	assert.Equal(t, AttachmentView{Kind: AttachPlan, ID: plan.Hex(), Name: "Tarde de tacos", Invite: InviteAccepted}, *v.Attachment)
	assert.Equal(t, me.Hex(), v.AuthorID)
	assert.Equal(t, []ReactionView{{Emoji: "🎉", Total: 1}}, v.Reactions)

	deleted := now
	m.DeletedAt = &deleted
	v = newDirectView(m, me, nil)
	assert.True(t, v.Deleted)
	assert.Empty(t, v.Text)
	assert.Nil(t, v.Attachment, "un mensaje borrado no muestra su adjunto")
	assert.Empty(t, v.Reactions)
}

func TestDirectos_VistaDeConversacion(t *testing.T) {
	me, other := bson.NewObjectID(), bson.NewObjectID()
	c := Conversation{ID: bson.NewObjectID(), Participants: []bson.ObjectID{me, other},
		Last: &DirectLast{MessageID: bson.NewObjectID(), Text: "hola", AuthorID: &other, At: time.Now()}}
	people := map[bson.ObjectID]usuarios.User{other: {ID: other, Name: "Jorge", Username: "jorge", PhotoURL: "https://img/x.webp"}}
	v := newConversationView(c, me, people, map[bson.ObjectID]int{c.ID: 3})
	require.NotNil(t, v.Other)
	assert.Equal(t, Person{ID: other.Hex(), Name: "Jorge", Username: "jorge", PhotoURL: "https://img/x.webp"}, *v.Other)
	assert.Equal(t, 3, v.Unread)
	assert.Equal(t, other.Hex(), v.Last.AuthorID)

	v = newConversationView(c, me, nil, nil)
	assert.Nil(t, v.Other, "la otra cuenta ya no está activa")
	assert.Zero(t, v.Unread)
}

// Lo que el módulo dueño no deja ver (404) es 422 del adjunto, sin decir si existe; un fallo de
// la base sigue siendo 503.
func TestDirectos_AdjuntoOculto(t *testing.T) {
	var appErr *apperr.Error
	require.ErrorAs(t, hidden(apperr.NotFound("plan no encontrado")), &appErr)
	assert.Equal(t, apperr.KindValidation, appErr.Kind)
	require.ErrorAs(t, hidden(apperr.Unavailable("x")), &appErr)
	assert.Equal(t, apperr.KindUnavailable, appErr.Kind)
}

func TestDirectos_SinSesion(t *testing.T) {
	s := NewDirectService(DirectDeps{})
	_, _, err := s.Open(context.Background(), "no-es-id", OpenRequest{UserID: bson.NewObjectID().Hex()})
	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperr.KindUnauthorized, appErr.Kind)
}

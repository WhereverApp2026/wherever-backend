package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/chat"
	"github.com/WhereverApp2026/wherever-backend/internal/moderacion"
	"github.com/WhereverApp2026/wherever-backend/internal/notificaciones"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
)

const dmPath = "/api/v1/conversaciones"

// dmSetup es el organizador de nv-37 (con plan y lugar) y un amigo suyo.
type dmSetup struct {
	gruposSetup
	friend amigo
}

func newDM(t *testing.T) dmSetup {
	t.Helper()
	s := newGrupos(t)
	return dmSetup{gruposSetup: s, friend: s.friends(t, "directo", 1)[0]}
}

func (s dmSetup) open(t *testing.T, who amigo, other string, want int) chat.ConversationView {
	t.Helper()
	return chatCall[chat.ConversationView](t, s.app, http.MethodPost, dmPath, map[string]string{"usuario_id": other}, who.token, want)
}

func (s dmSetup) write(t *testing.T, who amigo, convID string, body map[string]any, want int) chat.DirectView {
	t.Helper()
	if _, ok := body["id_cliente"]; !ok {
		body["id_cliente"] = newUUID(t)
	}
	return chatCall[chat.DirectView](t, s.app, http.MethodPost, dmPath+"/"+convID+"/mensajes", body, who.token, want)
}

func (s dmSetup) say(t *testing.T, who amigo, convID, text string) chat.DirectView {
	t.Helper()
	return s.write(t, who, convID, map[string]any{"texto": text}, http.StatusCreated)
}

func (s dmSetup) messages(t *testing.T, who amigo, convID, query string, want int) pagination.PageResult[chat.DirectView] {
	t.Helper()
	return chatCall[pagination.PageResult[chat.DirectView]](t, s.app, http.MethodGet, dmPath+"/"+convID+"/mensajes"+query, nil, who.token, want)
}

func (s dmSetup) inbox(t *testing.T, who amigo) []chat.ConversationView {
	t.Helper()
	return chatCall[chat.ConversationList](t, s.app, http.MethodGet, dmPath, nil, who.token, http.StatusOK).Items
}

func dmTexts(page pagination.PageResult[chat.DirectView]) []string {
	out := make([]string, 0, len(page.Items))
	for _, m := range page.Items {
		out = append(out, m.Text)
	}
	return out
}

func TestDirectos_AbrirOReutilizar(t *testing.T) {
	s := newDM(t)
	first := s.open(t, s.org, s.friend.id(), http.StatusCreated)
	require.NotNil(t, first.Other)
	assert.Equal(t, s.friend.id(), first.Other.ID)
	assert.Equal(t, s.friend.user.Name, first.Other.Name)
	assert.Nil(t, first.Last)
	again := s.open(t, s.org, s.friend.id(), http.StatusOK)
	assert.Equal(t, first.ID, again.ID)
	mirror := s.open(t, s.friend, s.org.id(), http.StatusOK)
	assert.Equal(t, first.ID, mirror.ID, "una sola conversación por par")

	stranger := seedAmigo(t, s.env, "desconocido_dm")
	s.open(t, s.org, stranger.id(), http.StatusNotFound)
	s.open(t, s.org, s.org.id(), http.StatusNotFound)
	s.open(t, s.org, bson.NewObjectID().Hex(), http.StatusNotFound)
	// Vacía, no sale en la lista.
	assert.Empty(t, s.inbox(t, s.org))
}

func TestDirectos_MensajesPaginasYReenvio(t *testing.T) {
	s := newDM(t)
	conv := s.open(t, s.org, s.friend.id(), http.StatusCreated)
	for i := range 5 {
		who := s.org
		if i%2 == 1 {
			who = s.friend
		}
		s.say(t, who, conv.ID, fmt.Sprintf("mensaje %d", i))
	}
	first := s.messages(t, s.friend, conv.ID, "?limite=2", http.StatusOK)
	assert.Equal(t, []string{"mensaje 4", "mensaje 3"}, dmTexts(first))
	require.NotEmpty(t, first.NextCursor)
	second := s.messages(t, s.friend, conv.ID, "?limite=2&cursor="+url.QueryEscape(first.NextCursor), http.StatusOK)
	assert.Equal(t, []string{"mensaje 2", "mensaje 1"}, dmTexts(second))
	newer := s.messages(t, s.friend, conv.ID, "?despues_de="+second.Items[0].ID, http.StatusOK)
	assert.Equal(t, []string{"mensaje 3", "mensaje 4"}, dmTexts(newer))
	// El cursor es de quien lo pidió.
	s.messages(t, s.org, conv.ID, "?cursor="+url.QueryEscape(first.NextCursor), http.StatusUnprocessableEntity)

	// Mismo id_cliente: 200 con el mismo mensaje, sin duplicar.
	id := newUUID(t)
	sent := s.write(t, s.org, conv.ID, map[string]any{"texto": "una vez", "id_cliente": id}, http.StatusCreated)
	again := s.write(t, s.org, conv.ID, map[string]any{"texto": "una vez", "id_cliente": id}, http.StatusOK)
	assert.Equal(t, sent.ID, again.ID)
	n, err := s.env.Mongo.DB().Collection(chat.Collection).CountDocuments(context.Background(), bson.D{{Key: "id_cliente", Value: id}})
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
	n, err = s.env.Mongo.DB().Collection(chat.Collection).CountDocuments(context.Background(), bson.D{{Key: "grupo_id", Value: bson.D{{Key: "$exists", Value: true}}}, {Key: "id_cliente", Value: id}})
	require.NoError(t, err)
	assert.Zero(t, n, "un mensaje directo no lleva grupo")
}

func TestDirectos_ListaNoLeidosYLeido(t *testing.T) {
	s := newDM(t)
	conv := s.open(t, s.org, s.friend.id(), http.StatusCreated)
	s.say(t, s.org, conv.ID, "hola")
	s.say(t, s.org, conv.ID, "¿vamos por tacos?")

	got := s.inbox(t, s.friend)
	require.Len(t, got, 1)
	assert.Equal(t, 2, got[0].Unread)
	require.NotNil(t, got[0].Last)
	assert.Equal(t, "¿vamos por tacos?", got[0].Last.Text)
	assert.Equal(t, s.org.id(), got[0].Last.AuthorID)
	require.NotNil(t, got[0].Other)
	assert.Equal(t, s.org.id(), got[0].Other.ID)
	assert.Zero(t, s.inbox(t, s.org)[0].Unread, "lo propio no cuenta")

	resp := sendJSON(t, s.app, http.MethodPost, dmPath+"/"+conv.ID+"/leido", nil, s.friend.token)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode, readAll(t, resp))
	assert.Zero(t, s.inbox(t, s.friend)[0].Unread)
	s.say(t, s.org, conv.ID, "¿sí?")
	assert.Equal(t, 1, s.inbox(t, s.friend)[0].Unread)

	// La conversación con el último mensaje sube en la lista.
	other := s.friends(t, "directo_b", 1)[0]
	conv2 := s.open(t, s.org, other.id(), http.StatusCreated)
	s.say(t, s.org, conv2.ID, "también a ti")
	list := s.inbox(t, s.org)
	require.Len(t, list, 2)
	assert.Equal(t, conv2.ID, list[0].ID)
}

func TestDirectos_Adjuntos(t *testing.T) {
	s := newDM(t)
	conv := s.open(t, s.org, s.friend.id(), http.StatusCreated)
	place := s.write(t, s.org, conv.ID, map[string]any{"texto": "mira este lugar", "adjunto": map[string]string{"tipo": "lugar", "id": s.shop.Hex()}}, http.StatusCreated)
	require.NotNil(t, place.Attachment)
	assert.Equal(t, chat.AttachPlace, place.Attachment.Kind)
	assert.NotEmpty(t, place.Attachment.Name)

	// Invitar a un plan: el adjunto y la invitación de nv-37/nv-38.
	plan := s.write(t, s.org, conv.ID, map[string]any{"texto": "¿te unes?", "adjunto": map[string]string{"tipo": "plan", "id": s.plan.ID.Hex()}}, http.StatusCreated)
	require.NotNil(t, plan.Attachment)
	assert.Equal(t, s.plan.Nombre, plan.Attachment.Name)
	assert.Empty(t, plan.Attachment.Invite, "aún sin grupo")
	g := s.create(t, []amigo{s.friend})
	page := s.messages(t, s.friend, conv.ID, "", http.StatusOK)
	require.NotNil(t, page.Items[0].Attachment)
	assert.Equal(t, chat.InviteSent, page.Items[0].Attachment.Invite)
	grupoCall(t, s.app, http.MethodPatch, gruposPath+"/"+g.ID+"/integrantes/yo", map[string]string{"estado": "confirmado"}, s.friend.token, http.StatusOK)
	page = s.messages(t, s.org, conv.ID, "", http.StatusOK)
	assert.Equal(t, chat.InviteAccepted, page.Items[0].Attachment.Invite, "se unió al plan")

	// Lo que no puedo ver es 422 sin decir si existe.
	stranger := seedAmigo(t, s.env, "ajeno_dm")
	foreign := s.newPlan(t, stranger)
	resp := sendJSON(t, s.app, http.MethodPost, dmPath+"/"+conv.ID+"/mensajes",
		map[string]any{"texto": "mira", "id_cliente": newUUID(t), "adjunto": map[string]string{"tipo": "plan", "id": foreign.ID.Hex()}}, s.org.token)
	body := readAll(t, resp)
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode, body)
	assert.NotContains(t, body, foreign.Nombre)
	for _, kind := range []string{"lugar", "momento", "plan"} {
		s.write(t, s.org, conv.ID, map[string]any{"texto": "x", "adjunto": map[string]string{"tipo": kind, "id": bson.NewObjectID().Hex()}}, http.StatusUnprocessableEntity)
	}
}

func TestDirectos_ReaccionesYBorrar(t *testing.T) {
	s := newDM(t)
	conv := s.open(t, s.org, s.friend.id(), http.StatusCreated)
	mine := s.say(t, s.org, conv.ID, "borra esto")
	base := dmPath + "/" + conv.ID + "/mensajes/" + mine.ID
	reacted := chatCall[chat.DirectView](t, s.app, http.MethodPut, base+"/reacciones", map[string]any{"emoji": "❤️"}, s.friend.token, http.StatusOK)
	assert.Equal(t, []chat.ReactionView{{Emoji: "❤️", Total: 1, Mine: true}}, reacted.Reactions)
	chatCall[chat.DirectView](t, s.app, http.MethodPut, base+"/reacciones", map[string]any{"emoji": "🍕"}, s.friend.token, http.StatusUnprocessableEntity)

	resp := sendJSON(t, s.app, http.MethodDelete, base, nil, s.friend.token)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "solo el autor borra")
	resp = sendJSON(t, s.app, http.MethodDelete, base, nil, s.org.token)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode, readAll(t, resp))
	page := s.messages(t, s.friend, conv.ID, "", http.StatusOK)
	require.Len(t, page.Items, 1)
	assert.True(t, page.Items[0].Deleted)
	assert.Empty(t, page.Items[0].Text)
	last := s.inbox(t, s.friend)[0].Last
	require.NotNil(t, last)
	assert.True(t, last.Deleted)
	assert.Empty(t, last.Text)
	var raw bson.M
	require.NoError(t, s.env.Mongo.DB().Collection(chat.Collection).FindOne(context.Background(), bson.D{{Key: "_id", Value: mustID(t, mine.ID)}}).Decode(&raw))
	assert.Equal(t, "", raw["texto"], "el texto no se guarda")
}

func dmBlock(t *testing.T, s dmSetup, from, to amigo) {
	t.Helper()
	_, err := s.env.Mongo.DB().Collection(amistades.Collection).UpdateOne(context.Background(),
		bson.D{{Key: "par", Value: amistades.PairKey(from.user.ID, to.user.ID)}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "estado", Value: amistades.StatusBlocked}, {Key: "de", Value: from.user.ID}, {Key: "para", Value: to.user.ID}}}})
	require.NoError(t, err)
}

// Un bloqueo corta la conversación para los dos y de inmediato.
func TestDirectos_BloqueoCortaLaConversacion(t *testing.T) {
	s := newDM(t)
	conv := s.open(t, s.org, s.friend.id(), http.StatusCreated)
	msg := s.say(t, s.org, conv.ID, "antes del bloqueo")
	dmBlock(t, s, s.friend, s.org)
	for _, who := range []amigo{s.org, s.friend} {
		assert.Empty(t, s.inbox(t, who))
		s.messages(t, who, conv.ID, "", http.StatusNotFound)
		s.write(t, who, conv.ID, map[string]any{"texto": "¿hola?"}, http.StatusNotFound)
		resp := sendJSON(t, s.app, http.MethodPost, dmPath+"/"+conv.ID+"/leido", nil, who.token)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		resp = sendJSON(t, s.app, http.MethodPut, dmPath+"/"+conv.ID+"/mensajes/"+msg.ID+"/reacciones", map[string]any{"emoji": "👍"}, who.token)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	}
	s.open(t, s.org, s.friend.id(), http.StatusNotFound)
}

// Sin amistad (la eliminaron) se puede leer, pero no escribir.
func TestDirectos_SinAmistadNoSeEscribe(t *testing.T) {
	s := newDM(t)
	conv := s.open(t, s.org, s.friend.id(), http.StatusCreated)
	s.say(t, s.org, conv.ID, "hola")
	_, err := s.env.Mongo.DB().Collection(amistades.Collection).DeleteOne(context.Background(), bson.D{{Key: "par", Value: amistades.PairKey(s.org.user.ID, s.friend.user.ID)}})
	require.NoError(t, err)
	s.messages(t, s.friend, conv.ID, "", http.StatusOK)
	s.write(t, s.friend, conv.ID, map[string]any{"texto": "¿sigues?"}, http.StatusNotFound)
}

func dmNotices(t *testing.T, s dmSetup, who amigo) int64 {
	t.Helper()
	n, err := s.env.Mongo.DB().Collection(notificaciones.Collection).CountDocuments(context.Background(),
		bson.D{{Key: "usuario_id", Value: who.user.ID}, {Key: "tipo", Value: notificaciones.KindDirectMessage}})
	require.NoError(t, err)
	return n
}

func TestDirectos_AvisoAgrupadoYSilencio(t *testing.T) {
	s := newDM(t)
	conv := s.open(t, s.org, s.friend.id(), http.StatusCreated)
	s.say(t, s.org, conv.ID, "uno")
	s.say(t, s.org, conv.ID, "dos")
	assert.Equal(t, int64(1), dmNotices(t, s, s.friend), "uno por conversación en el lapso")
	assert.Zero(t, dmNotices(t, s, s.org), "a quien escribe no")
	var n notificaciones.Notification
	require.NoError(t, s.env.Mongo.DB().Collection(notificaciones.Collection).FindOne(context.Background(),
		bson.D{{Key: "usuario_id", Value: s.friend.user.ID}, {Key: "tipo", Value: notificaciones.KindDirectMessage}}).Decode(&n))
	raw, err := json.Marshal(n)
	require.NoError(t, err)
	assert.Contains(t, string(raw), conv.ID)
	assert.NotContains(t, string(raw), "uno", "el aviso no lleva el texto")

	// La otra persona silenció a quien escribe (nv-70): no hay aviso.
	_, err = s.env.Mongo.DB().Collection(amistades.Collection).UpdateOne(context.Background(),
		bson.D{{Key: "par", Value: amistades.PairKey(s.org.user.ID, s.friend.user.ID)}},
		bson.D{{Key: "$addToSet", Value: bson.D{{Key: "silenciado_por", Value: s.org.user.ID}}}})
	require.NoError(t, err)
	s.say(t, s.friend, conv.ID, "¿me oyes?")
	assert.Zero(t, dmNotices(t, s, s.org))
}

func TestDirectos_ModeracionPorReglas(t *testing.T) {
	s := newDM(t)
	conv := s.open(t, s.org, s.friend.id(), http.StatusCreated)
	s.env.ModeradorDirectos.Fijar(moderacion.VeredictoRechazado)
	s.write(t, s.org, conv.ID, map[string]any{"texto": "algo prohibido"}, http.StatusUnprocessableEntity)
	s.env.ModeradorDirectos.Fijar(moderacion.VeredictoCuarentena)
	s.say(t, s.org, conv.ID, "dudoso sí se guarda")
}

func TestDirectos_LimitePorHora(t *testing.T) {
	s := newDM(t)
	conv := s.open(t, s.org, s.friend.id(), http.StatusCreated)
	convID := mustID(t, conv.ID)
	repo := chat.NewRepository(s.env.Mongo.DB())
	now := time.Now().UTC()
	for range chat.DirectSendLimit {
		m := chat.Message{ID: bson.NewObjectID(), ConversationID: convID, AuthorID: &s.org.user.ID, Kind: chat.KindText, Text: "x", Reactions: []chat.Reaction{}, CreatedAt: now}
		require.NoError(t, repo.Insert(context.Background(), &m))
	}
	resp := sendJSON(t, s.app, http.MethodPost, dmPath+"/"+conv.ID+"/mensajes", map[string]string{"texto": "uno de más", "id_cliente": newUUID(t)}, s.org.token)
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode, readAll(t, resp))
	assert.NotEmpty(t, resp.Header.Get(fiber.HeaderRetryAfter))
	s.say(t, s.friend, conv.ID, "la otra persona sí")
}

func TestDirectos_ExportarYBorrarCuenta(t *testing.T) {
	s := newDM(t)
	conv := s.open(t, s.org, s.friend.id(), http.StatusCreated)
	mine := s.say(t, s.org, conv.ID, "mi mensaje directo")
	theirs := s.say(t, s.friend, conv.ID, "el de mi amigo")
	chatCall[chat.DirectView](t, s.app, http.MethodPut, dmPath+"/"+conv.ID+"/mensajes/"+theirs.ID+"/reacciones", map[string]any{"emoji": "👍"}, s.org.token, http.StatusOK)
	db := s.env.Mongo.DB()
	priv := chat.NewDirectPrivacy(chat.NewRepository(db), chat.NewConversationRepository(db))

	got, err := priv.Export(context.Background(), s.org.user.ID)
	require.NoError(t, err)
	raw, err := json.Marshal(got)
	require.NoError(t, err)
	assert.Contains(t, string(raw), "mi mensaje directo")
	assert.Contains(t, string(raw), theirs.ID, "su reacción")
	assert.Contains(t, string(raw), conv.ID)
	assert.NotContains(t, string(raw), "el de mi amigo", "nunca el texto de la otra persona")
	group, err := chat.NewPrivacy(chat.NewRepository(db), nil).Export(context.Background(), s.org.user.ID)
	require.NoError(t, err)
	raw, err = json.Marshal(group)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "mi mensaje directo", "el chat de grupo no exporta los directos")

	require.NoError(t, priv.PurgeAccount(context.Background(), s.org.user.ID))
	page := s.messages(t, s.friend, conv.ID, "", http.StatusOK)
	require.Len(t, page.Items, 2)
	assert.Equal(t, mine.ID, page.Items[1].ID)
	assert.True(t, page.Items[1].Deleted, "queda «Mensaje eliminado»")
	assert.Empty(t, page.Items[1].AuthorID)
	assert.Empty(t, page.Items[0].Reactions, "sus reacciones se quitan")
	n, err := db.Collection(chat.ConversationCollection).CountDocuments(context.Background(), bson.D{{Key: "_id", Value: mustID(t, conv.ID)}})
	require.NoError(t, err)
	assert.Equal(t, int64(1), n, "la otra persona la conserva")

	// Las dos borraron su cuenta: la conversación y sus mensajes desaparecen.
	require.NoError(t, priv.PurgeAccount(context.Background(), s.friend.user.ID))
	n, err = db.Collection(chat.ConversationCollection).CountDocuments(context.Background(), bson.D{{Key: "_id", Value: mustID(t, conv.ID)}})
	require.NoError(t, err)
	assert.Zero(t, n)
	n, err = db.Collection(chat.Collection).CountDocuments(context.Background(), bson.D{{Key: "conversacion_id", Value: mustID(t, conv.ID)}})
	require.NoError(t, err)
	assert.Zero(t, n)
}

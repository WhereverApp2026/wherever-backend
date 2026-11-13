package integration

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/chat"
	"github.com/WhereverApp2026/wherever-backend/internal/grupos"
	"github.com/WhereverApp2026/wherever-backend/internal/notificaciones"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
)

// chatSetup es un grupo con tres amigos del organizador: dos confirmados y uno invitado.
type chatSetup struct {
	gruposSetup
	groupID string
	friends []amigo
}

func newChat(t *testing.T) chatSetup {
	t.Helper()
	s := newGrupos(t)
	fr := s.friends(t, "charla", 3)
	g := s.create(t, fr)
	for _, f := range fr[:2] {
		grupoCall(t, s.app, http.MethodPatch, gruposPath+"/"+g.ID+"/integrantes/yo", map[string]string{"estado": "confirmado"}, f.token, http.StatusOK)
	}
	return chatSetup{gruposSetup: s, groupID: g.ID, friends: fr}
}

// newUUID arma un UUID v4 para `id_cliente`.
func newUUID(t *testing.T) string {
	t.Helper()
	b := make([]byte, 16)
	_, err := rand.Read(b)
	require.NoError(t, err)
	b[6], b[8] = b[6]&0x0f|0x40, b[8]&0x3f|0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func (s chatSetup) path(rest string) string { return gruposPath + "/" + s.groupID + "/mensajes" + rest }

func chatCall[T any](t *testing.T, app *fiber.App, method, path string, body any, token string, want int) T {
	t.Helper()
	resp := sendJSON(t, app, method, path, body, token)
	raw := readAll(t, resp)
	require.Equal(t, want, resp.StatusCode, raw)
	var out T
	if want == http.StatusOK || want == http.StatusCreated {
		require.NoError(t, json.Unmarshal([]byte(raw), &out), raw)
		assert.Equal(t, "no-store", resp.Header.Get(fiber.HeaderCacheControl))
	}
	return out
}

func (s chatSetup) say(t *testing.T, who amigo, text string, want int) chat.View {
	t.Helper()
	return chatCall[chat.View](t, s.app, http.MethodPost, s.path(""), map[string]string{"texto": text, "id_cliente": newUUID(t)}, who.token, want)
}

func (s chatSetup) list(t *testing.T, who amigo, query string, want int) pagination.PageResult[chat.View] {
	t.Helper()
	return chatCall[pagination.PageResult[chat.View]](t, s.app, http.MethodGet, s.path(query), nil, who.token, want)
}

func textsOf(page pagination.PageResult[chat.View]) []string {
	out := make([]string, 0, len(page.Items))
	for _, m := range page.Items {
		out = append(out, m.Text)
	}
	return out
}

func TestChat_EnviarListarYPaginar(t *testing.T) {
	s := newChat(t)
	for i := range 5 {
		s.say(t, s.friends[i%2], fmt.Sprintf("mensaje %d", i), http.StatusCreated)
	}
	first := s.list(t, s.org, "?limite=2", http.StatusOK)
	assert.Equal(t, []string{"mensaje 4", "mensaje 3"}, textsOf(first), "del más nuevo al más viejo")
	require.NotEmpty(t, first.NextCursor)
	require.NotNil(t, first.Items[0].Author)
	assert.Equal(t, s.friends[0].id(), first.Items[0].Author.ID)

	second := s.list(t, s.org, "?limite=2&cursor="+url.QueryEscape(first.NextCursor), http.StatusOK)
	assert.Equal(t, []string{"mensaje 2", "mensaje 1"}, textsOf(second))

	// Ponerse al día: lo nuevo después del 2, del más viejo al más nuevo.
	newer := s.list(t, s.org, "?despues_de="+second.Items[0].ID, http.StatusOK)
	assert.Equal(t, []string{"mensaje 3", "mensaje 4"}, textsOf(newer))

	// El cursor es de quien lo pidió: otro integrante recibe 422.
	s.list(t, s.friends[0], "?cursor="+url.QueryEscape(first.NextCursor), http.StatusUnprocessableEntity)
}

func TestChat_Buscar(t *testing.T) {
	s := newChat(t)
	s.say(t, s.org, "Nos vemos en la plaza", http.StatusCreated)
	s.say(t, s.friends[0], "Llevo el pastel", http.StatusCreated)
	s.say(t, s.friends[1], "La plaza está cerrada", http.StatusCreated)
	got := s.list(t, s.org, "?q=plaza", http.StatusOK)
	assert.Equal(t, []string{"La plaza está cerrada", "Nos vemos en la plaza"}, textsOf(got))
	s.list(t, s.org, "?q=p", http.StatusUnprocessableEntity)

	// Otro grupo con la misma palabra no se mezcla.
	foreign := chat.Message{ID: bson.NewObjectID(), GroupID: bson.NewObjectID(), Kind: chat.KindText, Text: "plaza de otro grupo", Reactions: []chat.Reaction{}, CreatedAt: time.Now()}
	require.NoError(t, chat.NewRepository(s.env.Mongo.DB()).Insert(context.Background(), &foreign))
	assert.Len(t, s.list(t, s.org, "?q=plaza", http.StatusOK).Items, 2)
}

func TestChat_ReenvioConMismoIdCliente(t *testing.T) {
	s := newChat(t)
	body := map[string]string{"texto": "hola", "id_cliente": newUUID(t)}
	first := chatCall[chat.View](t, s.app, http.MethodPost, s.path(""), body, s.org.token, http.StatusCreated)
	again := chatCall[chat.View](t, s.app, http.MethodPost, s.path(""), body, s.org.token, http.StatusOK)
	assert.Equal(t, first.ID, again.ID)
	n, err := s.env.Mongo.DB().Collection(chat.Collection).CountDocuments(context.Background(), bson.D{})
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
	chatCall[chat.View](t, s.app, http.MethodPost, s.path(""), map[string]string{"texto": "   ", "id_cliente": newUUID(t)}, s.org.token, http.StatusUnprocessableEntity)
}

// Por el service: por HTTP, el límite de 10 escrituras simultáneas da 429 antes.
func TestChat_Concurrencia_MismoIdCliente(t *testing.T) {
	s := newChat(t)
	req := chat.SendRequest{Text: "una vez", ClientID: newUUID(t)}
	var wg sync.WaitGroup
	created := make(chan bool, 10)
	for range 10 {
		wg.Go(func() {
			_, ok, err := s.env.Chat.Send(context.Background(), s.org.id(), s.groupID, req)
			assert.NoError(t, err)
			created <- ok
		})
	}
	wg.Wait()
	close(created)
	n := 0
	for ok := range created {
		if ok {
			n++
		}
	}
	assert.Equal(t, 1, n, "solo uno responde 201")
	count, err := s.env.Mongo.DB().Collection(chat.Collection).CountDocuments(context.Background(), bson.D{{Key: "id_cliente", Value: req.ClientID}})
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestChat_Reacciones(t *testing.T) {
	s := newChat(t)
	m := s.say(t, s.org, "¿pizza?", http.StatusCreated)
	path := s.path("/" + m.ID + "/reacciones")
	got := chatCall[chat.View](t, s.app, http.MethodPut, path, map[string]any{"emoji": "👍"}, s.friends[0].token, http.StatusOK)
	assert.Equal(t, []chat.ReactionView{{Emoji: "👍", Total: 1, Mine: true}}, got.Reactions)
	chatCall[chat.View](t, s.app, http.MethodPut, path, map[string]any{"emoji": "👍"}, s.friends[1].token, http.StatusOK)
	got = chatCall[chat.View](t, s.app, http.MethodPut, path, map[string]any{"emoji": "🎉"}, s.friends[0].token, http.StatusOK)
	assert.Equal(t, []chat.ReactionView{{Emoji: "👍", Total: 1}, {Emoji: "🎉", Total: 1, Mine: true}}, got.Reactions, "cambiar reemplaza la anterior")
	got = chatCall[chat.View](t, s.app, http.MethodPut, path, map[string]any{"emoji": nil}, s.friends[0].token, http.StatusOK)
	assert.Equal(t, []chat.ReactionView{{Emoji: "👍", Total: 1}}, got.Reactions, "null la quita")

	chatCall[chat.View](t, s.app, http.MethodPut, path, map[string]any{"emoji": "🍕"}, s.org.token, http.StatusUnprocessableEntity)
	chatCall[chat.View](t, s.app, http.MethodPut, s.path("/"+bson.NewObjectID().Hex()+"/reacciones"), map[string]any{"emoji": "👍"}, s.org.token, http.StatusNotFound)
}

func TestChat_BorrarPropioYAjeno(t *testing.T) {
	s := newChat(t)
	m := s.say(t, s.friends[0], "me equivoqué", http.StatusCreated)
	chatCall[chat.View](t, s.app, http.MethodPut, s.path("/"+m.ID+"/reacciones"), map[string]any{"emoji": "😂"}, s.org.token, http.StatusOK)

	chatCall[any](t, s.app, http.MethodDelete, s.path("/"+m.ID), nil, s.org.token, http.StatusNotFound)
	chatCall[any](t, s.app, http.MethodDelete, s.path("/"+m.ID), nil, s.friends[0].token, http.StatusNoContent)
	chatCall[any](t, s.app, http.MethodDelete, s.path("/"+m.ID), nil, s.friends[0].token, http.StatusNoContent)

	got := s.list(t, s.org, "", http.StatusOK)
	require.Len(t, got.Items, 1)
	assert.True(t, got.Items[0].Deleted)
	assert.Empty(t, got.Items[0].Text)
	assert.Empty(t, got.Items[0].Reactions)
	var doc chat.Message
	require.NoError(t, s.env.Mongo.DB().Collection(chat.Collection).FindOne(context.Background(), bson.D{{Key: "_id", Value: mustID(t, m.ID)}}).Decode(&doc))
	assert.Empty(t, doc.Text, "no se guarda lo borrado")
	chatCall[chat.View](t, s.app, http.MethodPut, s.path("/"+m.ID+"/reacciones"), map[string]any{"emoji": "👍"}, s.org.token, http.StatusNotFound)
}

// Solo los confirmados leen y escriben; quien declina, sale o es sacado deja de leer lo nuevo.
func TestChat_PrivacidadSoloConfirmados(t *testing.T) {
	s := newChat(t)
	invited := s.friends[2]
	outsider := seedAmigo(t, s.env, "fuera")
	s.say(t, s.org, "solo para el grupo", http.StatusCreated)
	for _, who := range []amigo{invited, outsider} {
		s.list(t, who, "", http.StatusNotFound)
		s.say(t, who, "hola", http.StatusNotFound)
	}

	// Declina: ya no lee; vuelve a confirmar y ve el historial completo.
	member := gruposPath + "/" + s.groupID + "/integrantes"
	grupoCall(t, s.app, http.MethodPatch, member+"/yo", map[string]string{"estado": "declinado"}, s.friends[1].token, http.StatusOK)
	s.list(t, s.friends[1], "", http.StatusNotFound)
	grupoCall(t, s.app, http.MethodPatch, member+"/yo", map[string]string{"estado": "confirmado"}, s.friends[1].token, http.StatusOK)
	assert.Len(t, s.list(t, s.friends[1], "", http.StatusOK).Items, 1)

	// Sale o lo sacan: 404 en todo.
	m := s.say(t, s.friends[0], "me voy", http.StatusCreated)
	grupoCall(t, s.app, http.MethodDelete, member+"/yo", nil, s.friends[0].token, http.StatusNoContent)
	s.list(t, s.friends[0], "", http.StatusNotFound)
	chatCall[any](t, s.app, http.MethodDelete, s.path("/"+m.ID), nil, s.friends[0].token, http.StatusNotFound)
	grupoCall(t, s.app, http.MethodDelete, member+"/"+s.friends[1].id(), nil, s.org.token, http.StatusNoContent)
	s.list(t, s.friends[1], "", http.StatusNotFound)

	// Plan borrado: el chat deja de verse también para el organizador.
	require.Equal(t, http.StatusNoContent, planCall(t, s.app, http.MethodDelete, planPath(s.plan, ""), nil, s.org.token, `"1"`).status)
	s.list(t, s.org, "", http.StatusNotFound)
}

// El bloqueo no saca a nadie del grupo: los dos siguen viendo los mensajes del otro (spec §5).
func TestChat_BloqueadosSiguenEnElGrupo(t *testing.T) {
	s := newChat(t)
	a, b := s.friends[0], s.friends[1]
	now := time.Now().UTC()
	block := amistades.Friendship{ID: bson.NewObjectID(), From: a.user.ID, To: b.user.ID, Status: amistades.StatusBlocked, Pair: amistades.PairKey(a.user.ID, b.user.ID), CreatedAt: now, UpdatedAt: now}
	require.NoError(t, amistades.NewRepository(s.env.Mongo.DB()).Insert(context.Background(), &block))
	s.say(t, a, "de a", http.StatusCreated)
	s.say(t, b, "de b", http.StatusCreated)
	assert.Equal(t, []string{"de b", "de a"}, textsOf(s.list(t, a, "", http.StatusOK)))
	assert.Equal(t, []string{"de b", "de a"}, textsOf(s.list(t, b, "", http.StatusOK)))
}

func TestChat_UltimoMensajeEnGrupos(t *testing.T) {
	s := newChat(t)
	long := ""
	for range 20 {
		long += "palabra "
	}
	s.say(t, s.org, "primero", http.StatusCreated)
	last := s.say(t, s.friends[0], long, http.StatusCreated)
	g := grupoCall(t, s.app, http.MethodGet, gruposPath+"/"+s.groupID, nil, s.org.token, http.StatusOK)
	require.NotNil(t, g.LastMessage)
	assert.Equal(t, last.ID, g.LastMessage.MessageID)
	assert.Equal(t, chat.Preview(last.Text), g.LastMessage.Text)
	assert.Len(t, []rune(g.LastMessage.Text), chat.PreviewRunes)

	// El invitado no lee el chat: tampoco el último mensaje.
	inv := grupoCall(t, s.app, http.MethodGet, gruposPath+"/"+s.groupID, nil, s.friends[2].token, http.StatusOK)
	assert.Nil(t, inv.LastMessage)

	// Borrar ese mensaje lo limpia.
	chatCall[any](t, s.app, http.MethodDelete, s.path("/"+last.ID), nil, s.friends[0].token, http.StatusNoContent)
	g = grupoCall(t, s.app, http.MethodGet, gruposPath+"/"+s.groupID, nil, s.org.token, http.StatusOK)
	assert.Nil(t, g.LastMessage)
}

func TestChat_AvisoDelSistemaAlCerrarVotacion(t *testing.T) {
	s := newVotacion(t)
	v := s.open(t, []bson.ObjectID{s.shop, s.b}, http.StatusCreated)
	votoCall(t, s.app, http.MethodPut, votePath(v, "/voto"), map[string]string{"negocio_id": s.b.Hex()}, s.friends[0].token, http.StatusOK)
	votoCall(t, s.app, http.MethodPost, votePath(v, "/cerrar"), nil, s.org.token, http.StatusOK)
	page := chatCall[pagination.PageResult[chat.View]](t, s.app, http.MethodGet, gruposPath+"/"+s.groupID+"/mensajes", nil, s.org.token, http.StatusOK)
	require.Len(t, page.Items, 1)
	assert.Equal(t, chat.KindSystem, page.Items[0].Kind)
	assert.Nil(t, page.Items[0].Author)
	assert.Contains(t, page.Items[0].Text, "con 1 voto")
	chatCall[chat.View](t, s.app, http.MethodPut, gruposPath+"/"+s.groupID+"/mensajes/"+page.Items[0].ID+"/reacciones", map[string]any{"emoji": "🎉"}, s.org.token, http.StatusNotFound)
}

// Un aviso por grupo y persona cada 5 minutos, solo a los demás confirmados.
func TestChat_PushAgrupado(t *testing.T) {
	s := newChat(t)
	for range 3 {
		s.say(t, s.org, "uno más", http.StatusCreated)
	}
	count := func(who amigo) int64 {
		n, err := s.env.Mongo.DB().Collection(notificaciones.Collection).CountDocuments(context.Background(),
			bson.D{{Key: "usuario_id", Value: who.user.ID}, {Key: "tipo", Value: notificaciones.KindGroupMessage}})
		require.NoError(t, err)
		return n
	}
	assert.Equal(t, int64(1), count(s.friends[0]))
	assert.Equal(t, int64(1), count(s.friends[1]))
	assert.Zero(t, count(s.friends[2]), "el invitado no recibe avisos del chat")
	assert.Zero(t, count(s.org), "quien escribe no se avisa")
	var n notificaciones.Notification
	require.NoError(t, s.env.Mongo.DB().Collection(notificaciones.Collection).FindOne(context.Background(),
		bson.D{{Key: "usuario_id", Value: s.friends[0].user.ID}, {Key: "tipo", Value: notificaciones.KindGroupMessage}}).Decode(&n))
	assert.Equal(t, map[string]string{"grupo_id": s.groupID}, n.Data, "sin texto ni autor")
}

func TestChat_LimiteDeMensajesPorMinuto(t *testing.T) {
	s := newChat(t)
	for i := range chat.SendLimit {
		s.say(t, s.org, fmt.Sprintf("m%d", i), http.StatusCreated)
	}
	resp := sendJSON(t, s.app, http.MethodPost, s.path(""), map[string]string{"texto": "uno de más", "id_cliente": newUUID(t)}, s.org.token)
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode, readAll(t, resp))
	assert.NotEmpty(t, resp.Header.Get(fiber.HeaderRetryAfter))
	s.say(t, s.friends[0], "otra persona sí", http.StatusCreated)
}

func TestChat_BorrarCuentaYExportar(t *testing.T) {
	s := newChat(t)
	mine := s.say(t, s.friends[0], "mi mensaje", http.StatusCreated)
	other := s.say(t, s.org, "de otra persona", http.StatusCreated)
	chatCall[chat.View](t, s.app, http.MethodPut, s.path("/"+other.ID+"/reacciones"), map[string]any{"emoji": "❤️"}, s.friends[0].token, http.StatusOK)
	priv := chat.NewPrivacy(chat.NewRepository(s.env.Mongo.DB()), grupos.NewRepository(s.env.Mongo.DB()))

	got, err := priv.Export(context.Background(), s.friends[0].user.ID)
	require.NoError(t, err)
	raw, err := json.Marshal(got)
	require.NoError(t, err)
	assert.Contains(t, string(raw), "mi mensaje")
	assert.Contains(t, string(raw), other.ID, "su reacción")
	assert.NotContains(t, string(raw), "de otra persona", "nunca el texto de otros")

	require.NoError(t, priv.PurgeAccount(context.Background(), s.friends[0].user.ID))
	page := s.list(t, s.org, "", http.StatusOK)
	require.Len(t, page.Items, 2)
	assert.Equal(t, other.ID, page.Items[0].ID)
	assert.Empty(t, page.Items[0].Reactions, "sus reacciones se quitan")
	assert.Equal(t, mine.ID, page.Items[1].ID)
	assert.True(t, page.Items[1].Deleted, "la conversación conserva su forma")
	assert.Nil(t, page.Items[1].Author)
	n, err := s.env.Mongo.DB().Collection(chat.Collection).CountDocuments(context.Background(), bson.D{{Key: "autor_id", Value: s.friends[0].user.ID}})
	require.NoError(t, err)
	assert.Zero(t, n)

	// El organizador borra su cuenta: el chat de su grupo desaparece con el grupo.
	require.NoError(t, priv.PurgeAccount(context.Background(), s.org.user.ID))
	n, err = s.env.Mongo.DB().Collection(chat.Collection).CountDocuments(context.Background(), bson.D{{Key: "grupo_id", Value: mustID(t, s.groupID)}})
	require.NoError(t, err)
	assert.Zero(t, n)
}

// Si sube alguna cifra, rehacer la estimación de specs/nv-57 §7 y de nv-11.
func TestPresupuestoRedis_Chat(t *testing.T) {
	s := newChat(t)
	assert.Equal(t, 7, commandsOf(t, func() {
		s.say(t, s.org, "cuánto cuesta", http.StatusCreated)
	}), "enviar: limitador global + escrituras + revocación; el límite del chat va en MongoDB")
	assert.Equal(t, 4, commandsOf(t, func() {
		s.list(t, s.org, "", http.StatusOK)
	}), "leer: limitador + revocación")
}

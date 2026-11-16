package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/notificaciones"
	"github.com/WhereverApp2026/wherever-backend/internal/reservas"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/reservastest"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// reservaSembrada guarda una reserva en ese estado y hora, sin pasar por POST /reservas.
func reservaSembrada(t *testing.T, env *authtest.Env, u usuarios.User, b negocios.Business, hora time.Time, estado string) reservas.Reserva {
	t.Helper()
	now := time.Now().UTC()
	r := reservas.Reserva{ID: bson.NewObjectID(), Codigo: strings.ToUpper(bson.NewObjectID().Hex()[18:]), UsuarioID: u.ID, NegocioID: b.ID,
		FechaHora: hora, Personas: 2, Estado: estado, Historial: []reservas.Cambio{}, Version: 1, CreadoEn: now, ActualizadoEn: now}
	require.NoError(t, reservas.NewRepository(env.Mongo.DB()).Insert(context.Background(), &r))
	return r
}

func rutaMensaje(b negocios.Business, r reservas.Reserva) string {
	return "/api/v1/negocios/" + b.ID.Hex() + "/reservas/" + r.ID.Hex() + "/mensajes"
}

// avisosDe cuenta los avisos de ese tipo de la persona.
func avisosDe(t *testing.T, env *authtest.Env, u bson.ObjectID, tipo string) []notificaciones.Notification {
	t.Helper()
	cur, err := env.Mongo.DB().Collection(notificaciones.Collection).Find(context.Background(), bson.D{{Key: "usuario_id", Value: u}, {Key: "tipo", Value: tipo}})
	require.NoError(t, err)
	out := []notificaciones.Notification{}
	require.NoError(t, cur.All(context.Background(), &out))
	return out
}

func TestReservasMensajes_EnviarLeerYLimite(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	b, tokenNegocio := dueno(t, env, "duena-msj@wherever.test", reservastest.Config{})
	u, tokenCliente := cliente(t, env, "cliente-msj@wherever.test")
	r := reservaSembrada(t, env, u, b, reservastest.Manana(14), reservas.EstadoConfirmada)

	resp := sendJSON(t, app, http.MethodPost, rutaMensaje(b, r), map[string]any{"texto": "  Tu mesa está junto a la ventana  "}, tokenNegocio)
	requireStatus(t, resp, http.StatusCreated)
	body := decode(t, resp)
	assertSnakeCase(t, body)
	assert.Equal(t, "Tu mesa está junto a la ventana", body["texto"], "recortado")
	assert.Equal(t, r.ID.Hex(), body["reserva_id"])
	primero := str(t, body["id"])

	avisos := avisosDe(t, env, u.ID, notificaciones.KindReservaMensaje)
	require.Len(t, avisos, 1, "un aviso al cliente")
	assert.Equal(t, map[string]string{"reserva_id": r.ID.Hex(), "negocio_id": b.ID.Hex(), "mensaje_id": primero}, avisos[0].Data, "solo ids")

	for i := range 4 {
		requireStatus(t, sendJSON(t, app, http.MethodPost, rutaMensaje(b, r), map[string]any{"texto": "aviso " + string(rune('a'+i))}, tokenNegocio), http.StatusCreated)
	}
	resp = sendJSON(t, app, http.MethodPost, rutaMensaje(b, r), map[string]any{"texto": "el sexto"}, tokenNegocio)
	requireStatus(t, resp, http.StatusTooManyRequests)
	assert.NotEmpty(t, resp.Header.Get("Retry-After"))
	assert.Len(t, avisosDe(t, env, u.ID, notificaciones.KindReservaMensaje), 5, "el 6.º no avisa")

	// El cliente lee los mensajes en el detalle, del más nuevo al más viejo.
	resp = sendJSON(t, app, http.MethodGet, "/api/v1/reservas/"+r.ID.Hex(), nil, tokenCliente)
	requireStatus(t, resp, http.StatusOK)
	mensajes, ok := decode(t, resp)["mensajes"].([]any)
	require.True(t, ok)
	require.Len(t, mensajes, 5)
	assert.Equal(t, "aviso d", obj(t, mensajes[0])["texto"])
	assert.Equal(t, primero, obj(t, mensajes[4])["id"])

	// El negocio no ve los mensajes en el detalle.
	resp = sendJSON(t, app, http.MethodGet, "/api/v1/reservas/"+r.ID.Hex(), nil, tokenNegocio)
	requireStatus(t, resp, http.StatusOK)
	_, conMensajes := decode(t, resp)["mensajes"]
	assert.False(t, conMensajes)

	// Sin mensajes, la lista va vacía.
	otra := reservaSembrada(t, env, u, b, reservastest.Manana(16), reservas.EstadoConfirmada)
	resp = sendJSON(t, app, http.MethodGet, "/api/v1/reservas/"+otra.ID.Hex(), nil, tokenCliente)
	requireStatus(t, resp, http.StatusOK)
	vacia, ok := decode(t, resp)["mensajes"].([]any)
	require.True(t, ok)
	assert.Empty(t, vacia)
}

func TestReservasMensajes_EstadosYAjenos(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	b, tokenNegocio := dueno(t, env, "duena-msj-estados@wherever.test", reservastest.Config{})
	otro, tokenOtro := dueno(t, env, "otro-msj-estados@wherever.test", reservastest.Config{})
	u, _ := cliente(t, env, "cliente-msj-estados@wherever.test")
	texto := map[string]any{"texto": "hola"}

	cancelada := reservaSembrada(t, env, u, b, reservastest.Manana(10), reservas.EstadoCancelada)
	resp := sendJSON(t, app, http.MethodPost, rutaMensaje(b, cancelada), texto, tokenNegocio)
	requireStatus(t, resp, http.StatusConflict)
	assert.Contains(t, str(t, decode(t, resp)["mensaje"]), "reserva_no_contactable")

	ayer := reservaSembrada(t, env, u, b, time.Now().UTC().Add(-3*time.Hour), reservas.EstadoAsistio)
	requireStatus(t, sendJSON(t, app, http.MethodPost, rutaMensaje(b, ayer), texto, tokenNegocio), http.StatusCreated)
	vieja := reservaSembrada(t, env, u, b, time.Now().UTC().Add(-48*time.Hour), reservas.EstadoNoAsistio)
	requireStatus(t, sendJSON(t, app, http.MethodPost, rutaMensaje(b, vieja), texto, tokenNegocio), http.StatusConflict)

	// Reserva de otro negocio por la ruta propia, o el negocio ajeno: 404 sin revelar nada.
	suya := reservaSembrada(t, env, u, otro, reservastest.Manana(11), reservas.EstadoConfirmada)
	requireStatus(t, sendJSON(t, app, http.MethodPost, rutaMensaje(b, suya), texto, tokenNegocio), http.StatusNotFound)
	confirmada := reservaSembrada(t, env, u, b, reservastest.Manana(12), reservas.EstadoConfirmada)
	requireStatus(t, sendJSON(t, app, http.MethodPost, rutaMensaje(b, confirmada), texto, tokenOtro), http.StatusNotFound)
	requireStatus(t, sendJSON(t, app, http.MethodPost, "/api/v1/negocios/"+b.ID.Hex()+"/reservas/"+bson.NewObjectID().Hex()+"/mensajes", texto, tokenNegocio), http.StatusNotFound)

	// Validación: vacío o HTML = 422; cuerpo ilegible = 400.
	requireStatus(t, sendJSON(t, app, http.MethodPost, rutaMensaje(b, confirmada), map[string]any{"texto": "   "}, tokenNegocio), http.StatusUnprocessableEntity)
	requireStatus(t, sendJSON(t, app, http.MethodPost, rutaMensaje(b, confirmada), map[string]any{"texto": "<b>hola</b>"}, tokenNegocio), http.StatusUnprocessableEntity)
	requireStatus(t, sendJSON(t, app, http.MethodPost, rutaMensaje(b, confirmada), "texto", tokenNegocio), http.StatusBadRequest)
	assert.Len(t, avisosDe(t, env, u.ID, notificaciones.KindReservaMensaje), 1, "solo el mensaje aceptado avisa")
}

func TestReservasMensajes_Privacidad(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	b, tokenNegocio := dueno(t, env, "duena-msj-arco@wherever.test", reservastest.Config{})
	u, _ := cliente(t, env, "cliente-msj-arco@wherever.test")
	r := reservaSembrada(t, env, u, b, reservastest.Manana(13), reservas.EstadoConfirmada)
	requireStatus(t, sendJSON(t, app, http.MethodPost, rutaMensaje(b, r), map[string]any{"texto": "Te guardamos la mesa"}, tokenNegocio), http.StatusCreated)

	repo := reservas.NewRepository(env.Mongo.DB())
	p := reservas.NewPrivacy(repo).WithNegocios(negocios.NewRepository(env.Mongo.DB()))
	exported, err := p.Export(context.Background(), u.ID)
	require.NoError(t, err)
	rawJSON, err := json.Marshal(exported)
	require.NoError(t, err)
	raw := string(rawJSON)
	assert.Contains(t, raw, "Te guardamos la mesa")
	assert.Contains(t, raw, b.Name, "con el nombre del negocio")

	require.NoError(t, p.PurgeHistory(context.Background(), u.ID))
	n, err := env.Mongo.DB().Collection(reservas.MensajesCollection).CountDocuments(context.Background(), bson.D{{Key: "usuario_id", Value: u.ID}})
	require.NoError(t, err)
	assert.Equal(t, int64(1), n, "borrar historial no los toca")

	require.NoError(t, p.PurgeAccount(context.Background(), u.ID))
	n, err = env.Mongo.DB().Collection(reservas.MensajesCollection).CountDocuments(context.Background(), bson.D{{Key: "usuario_id", Value: u.ID}})
	require.NoError(t, err)
	assert.Zero(t, n, "borrar la cuenta los quita")
}

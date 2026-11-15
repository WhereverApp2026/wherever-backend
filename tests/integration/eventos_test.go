package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/eventos"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/moderacion"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
)

const eventosPath = "/api/v1/eventos"

// eventosSetup: un comerciante con negocio aprobado, un administrador y la app.
type eventosSetup struct {
	env      *authtest.Env
	app      *fiber.App
	merchant string
	merchID  bson.ObjectID
	negocio  string
	admin    string
}

func newEventos(t *testing.T, prefix string) eventosSetup {
	t.Helper()
	env := authEnv(t)
	env.ClearRateLimits(t)
	duena := env.CreateUser(t, prefix+"-duena@wherever.test")
	b := env.AltaAprobada(t, duena, "Eventos "+prefix)
	admin := env.CreateAdmin(t, prefix+"-admin@wherever.test")
	return eventosSetup{
		env: env, app: env.App(), merchID: duena.ID, negocio: b.ID,
		merchant: testutil.ValidTokenFor(t, duena.ID.Hex(), "jti-"+prefix+"-duena", middleware.RoleConsumer, middleware.RoleMerchant),
		admin:    testutil.ValidTokenFor(t, admin.ID.Hex(), "jti-"+prefix+"-admin", middleware.RoleAdmin),
	}
}

func eventoBody(titulo, negocio string, inicio time.Time, dLat float64) map[string]any {
	body := map[string]any{
		"titulo": titulo, "tipo": "feria", "descripcion": "Comida y música", "direccion": "Plaza principal",
		"ubicacion": map[string]float64{"lat": centerLat + dLat, "lng": centerLng},
		"inicio":    inicio.Format(time.RFC3339), "fin": inicio.Add(3 * time.Hour).Format(time.RFC3339), "precio": 0,
	}
	if negocio != "" {
		body["negocio_id"] = negocio
	}
	return body
}

func eventoCall(t *testing.T, app *fiber.App, method, path string, body any, token, ifMatch string) (int, string, http.Header) {
	t.Helper()
	got := planCall(t, app, method, path, body, token, ifMatch)
	h := http.Header{}
	h.Set("ETag", got.etag)
	return got.status, got.body, h
}

func (s eventosSetup) crear(t *testing.T, token string, body map[string]any) eventos.EventoView {
	t.Helper()
	status, raw, _ := eventoCall(t, s.app, http.MethodPost, eventosPath, body, token, "")
	require.Equal(t, http.StatusCreated, status, raw)
	var v eventos.EventoView
	require.NoError(t, json.Unmarshal([]byte(raw), &v))
	return v
}

func listaEventos(t *testing.T, app *fiber.App, query string) (pageIDs []string, next string) {
	t.Helper()
	resp := sendJSON(t, app, http.MethodGet, eventosPath+"?lat="+fmt.Sprint(centerLat)+"&lng="+fmt.Sprint(centerLng)+query, nil, "")
	raw := readAll(t, resp)
	require.Equal(t, http.StatusOK, resp.StatusCode, raw)
	var out struct {
		Items []eventos.EventoView `json:"items"`
		Next  string               `json:"siguiente_cursor"`
	}
	require.NoError(t, json.Unmarshal([]byte(raw), &out))
	for _, e := range out.Items {
		require.NotNil(t, e.DistanciaM)
		pageIDs = append(pageIDs, e.Titulo)
	}
	return pageIDs, out.Next
}

func TestEventos_ListaPorFechaYDistancia(t *testing.T) {
	s := newEventos(t, "lista")
	_, err := s.env.Mongo.DB().Collection(eventos.Collection).DeleteMany(context.Background(), bson.D{})
	require.NoError(t, err)
	manana := time.Now().Add(24 * time.Hour).Truncate(time.Minute)
	s.crear(t, s.merchant, eventoBody("Cerca mañana", s.negocio, manana, 0.001))
	s.crear(t, s.merchant, eventoBody("Lejos mañana", s.negocio, manana, 0.02))
	s.crear(t, s.admin, eventoBody("Pasado mañana", "", manana.Add(24*time.Hour), 0))
	s.crear(t, s.admin, eventoBody("En diez días", "", manana.Add(9*24*time.Hour), 0))
	borrador := eventoBody("Borrador", s.negocio, manana, 0)
	borrador["estado"] = "borrador"
	s.crear(t, s.merchant, borrador)
	cancelado := s.crear(t, s.merchant, eventoBody("Cancelado", s.negocio, manana, 0))
	status, raw, _ := eventoCall(t, s.app, http.MethodPatch, eventosPath+"/"+cancelado.ID, map[string]string{"estado": "cancelado"}, s.merchant, "1")
	require.Equal(t, http.StatusOK, status, raw)
	terminado := s.crear(t, s.admin, eventoBody("Terminado", "", time.Now().Add(-30*time.Minute), 0))
	_, err = s.env.Mongo.DB().Collection(eventos.Collection).UpdateOne(context.Background(), bson.D{{Key: "_id", Value: mustOID(t, terminado.ID)}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "fin", Value: time.Now().Add(-time.Minute)}}}})
	require.NoError(t, err)

	names, next := listaEventos(t, s.app, "")
	assert.Equal(t, []string{"Cerca mañana", "Lejos mañana", "Pasado mañana"}, names, "por inicio y luego distancia; sin borradores, cancelados, terminados ni los de más de 7 días")
	assert.Empty(t, next)
	names, _ = listaEventos(t, s.app, "&dias=10")
	assert.Contains(t, names, "En diez días")
	names, _ = listaEventos(t, s.app, "&radio_m=1000")
	assert.NotContains(t, names, "Lejos mañana")
	names, _ = listaEventos(t, s.app, "&tipo=tianguis")
	assert.Empty(t, names)

	page, next := listaEventos(t, s.app, "&limite=2")
	require.Len(t, page, 2)
	require.NotEmpty(t, next)
	rest, _ := listaEventos(t, s.app, "&limite=2&cursor="+next)
	assert.Equal(t, []string{"Pasado mañana"}, rest)
	resp := sendJSON(t, s.app, http.MethodGet, eventosPath+"?lat=19&lng=-99&limite=2&cursor="+next, nil, "")
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode, "cursor de otra consulta")
	resp = sendJSON(t, s.app, http.MethodGet, eventosPath, nil, "")
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode, "sin lat ni lng")
}

func TestEventos_CrearPermisosEditarYBorrar(t *testing.T) {
	s := newEventos(t, "crud")
	manana := time.Now().Add(24 * time.Hour)
	otra := s.env.CreateUser(t, "crud-otra@wherever.test")
	otroNegocio := s.env.AltaAprobada(t, otra, "Otro negocio crud")
	otraToken := testutil.ValidTokenFor(t, otra.ID.Hex(), "jti-crud-otra", middleware.RoleConsumer, middleware.RoleMerchant)

	status, _, _ := eventoCall(t, s.app, http.MethodPost, eventosPath, eventoBody("Ajeno", otroNegocio.ID, manana, 0), s.merchant, "")
	assert.Equal(t, http.StatusForbidden, status, "negocio ajeno")
	enRevision := s.env.AltaEnRevision(t, s.env.CreateUser(t, "crud-rev@wherever.test"), "En revisión crud")
	status, _, _ = eventoCall(t, s.app, http.MethodPost, eventosPath, eventoBody("Sin aprobar", enRevision.ID, manana, 0), s.merchant, "")
	assert.Equal(t, http.StatusForbidden, status)
	status, _, _ = eventoCall(t, s.app, http.MethodPost, eventosPath, eventoBody("Sin negocio", "", manana, 0), s.merchant, "")
	assert.Equal(t, http.StatusUnprocessableEntity, status)
	mal := eventoBody("Fechas", s.negocio, manana, 0)
	mal["fin"] = manana.Add(-time.Hour).Format(time.RFC3339)
	status, _, _ = eventoCall(t, s.app, http.MethodPost, eventosPath, mal, s.merchant, "")
	assert.Equal(t, http.StatusUnprocessableEntity, status)
	foto := eventoBody("Foto ajena", s.negocio, manana, 0)
	foto["foto_url"] = "https://otro.com/a.jpg"
	status, _, _ = eventoCall(t, s.app, http.MethodPost, eventosPath, foto, s.merchant, "")
	assert.Equal(t, http.StatusUnprocessableEntity, status)

	resp := sendJSON(t, s.app, http.MethodPost, eventosPath, eventoBody("Mío", s.negocio, manana, 0), s.merchant)
	raw := readAll(t, resp)
	require.Equal(t, http.StatusCreated, resp.StatusCode, raw)
	var v eventos.EventoView
	require.NoError(t, json.Unmarshal([]byte(raw), &v))
	assert.Equal(t, eventosPath+"/"+v.ID, resp.Header.Get("Location"))
	assert.Equal(t, `"1"`, resp.Header.Get("ETag"))
	assert.NotContains(t, raw, "creado_por")
	path := eventosPath + "/" + v.ID

	status, _, _ = eventoCall(t, s.app, http.MethodPatch, path, map[string]string{"titulo": "Otro"}, otraToken, "1")
	assert.Equal(t, http.StatusForbidden, status)
	status, _, _ = eventoCall(t, s.app, http.MethodPatch, path, map[string]string{"titulo": "Otro"}, s.merchant, "")
	assert.Equal(t, http.StatusBadRequest, status, "sin If-Match")
	status, raw, _ = eventoCall(t, s.app, http.MethodPatch, path, map[string]string{"titulo": "Renombrado"}, s.merchant, "1")
	require.Equal(t, http.StatusOK, status, raw)
	assert.Contains(t, raw, `"version":2`)
	status, _, _ = eventoCall(t, s.app, http.MethodPatch, path, map[string]string{"titulo": "Viejo"}, s.merchant, "1")
	assert.Equal(t, http.StatusConflict, status, "versión vieja")
	status, raw, _ = eventoCall(t, s.app, http.MethodPatch, path, map[string]string{"estado": "cancelado"}, s.admin, "2")
	require.Equal(t, http.StatusOK, status, raw)
	status, _, _ = eventoCall(t, s.app, http.MethodPatch, path, map[string]string{"estado": "publicado"}, s.merchant, "3")
	assert.Equal(t, http.StatusConflict, status, "cancelado no se reabre")
	status, raw, _ = eventoCall(t, s.app, http.MethodGet, path, nil, "", "")
	assert.Equal(t, http.StatusNotFound, status, "cancelado oculto para el público: "+raw)
	status, _, _ = eventoCall(t, s.app, http.MethodGet, path, nil, s.merchant, "")
	assert.Equal(t, http.StatusOK, status, "quien puede editar lo ve")

	status, _, _ = eventoCall(t, s.app, http.MethodDelete, path, nil, otraToken, "3")
	assert.Equal(t, http.StatusForbidden, status)
	status, _, _ = eventoCall(t, s.app, http.MethodDelete, path, nil, s.merchant, "2")
	assert.Equal(t, http.StatusConflict, status)
	status, _, _ = eventoCall(t, s.app, http.MethodDelete, path, nil, s.merchant, "3")
	assert.Equal(t, http.StatusNoContent, status)
	status, _, _ = eventoCall(t, s.app, http.MethodGet, path, nil, s.merchant, "")
	assert.Equal(t, http.StatusNotFound, status)
	status, _, _ = eventoCall(t, s.app, http.MethodGet, eventosPath+"/no-es-id", nil, "", "")
	assert.Equal(t, http.StatusNotFound, status)
}

func TestEventos_ModeracionYLimite(t *testing.T) {
	s := newEventos(t, "moder")
	manana := time.Now().Add(24 * time.Hour)
	s.env.ModeradorEventos.Fijar(moderacion.VeredictoRechazado)
	status, _, _ := eventoCall(t, s.app, http.MethodPost, eventosPath, eventoBody("Rechazado", s.negocio, manana, 0), s.merchant, "")
	assert.Equal(t, http.StatusUnprocessableEntity, status)

	s.env.ModeradorEventos.Fijar(moderacion.VeredictoCuarentena)
	dudoso := s.crear(t, s.merchant, eventoBody("Dudoso", s.negocio, manana, 0))
	assert.True(t, dudoso.EnRevision)
	status, _, _ = eventoCall(t, s.app, http.MethodGet, eventosPath+"/"+dudoso.ID, nil, "", "")
	assert.Equal(t, http.StatusNotFound, status, "en revisión no se muestra")
	items, err := s.env.Moderacion.Listar(context.Background(), moderacion.ListaQuery{Tipo: eventos.TipoCola, Limit: 10})
	require.NoError(t, err)
	require.Len(t, items.Items, 1)
	admin := s.env.CreateAdmin(t, "moder-revisor@wherever.test")
	_, err = s.env.Moderacion.Decidir(context.Background(), admin.ID.Hex(), items.Items[0].ID, 1, moderacion.DecisionRequest{Decision: moderacion.EstadoAprobado})
	require.NoError(t, err)
	status, _, _ = eventoCall(t, s.app, http.MethodGet, eventosPath+"/"+dudoso.ID, nil, "", "")
	assert.Equal(t, http.StatusOK, status, "aprobado por el administrador")

	s.env.ModeradorEventos.Fijar(moderacion.VeredictoAprobado)
	s.env.ClearRateLimits(t)
	for i := range 10 {
		status, raw, _ := eventoCall(t, s.app, http.MethodPost, eventosPath, eventoBody("Evento "+strconv.Itoa(i), s.negocio, manana, 0), s.merchant, "")
		require.Equal(t, http.StatusCreated, status, raw)
	}
	status, _, _ = eventoCall(t, s.app, http.MethodPost, eventosPath, eventoBody("Once", s.negocio, manana, 0), s.merchant, "")
	assert.Equal(t, http.StatusTooManyRequests, status)
}

func TestPrivacidadEventos_ExportarYBorrarCuenta(t *testing.T) {
	s := newEventos(t, "arco")
	manana := time.Now().Add(24 * time.Hour)
	delNegocio := s.crear(t, s.merchant, eventoBody("Del negocio", s.negocio, manana, 0))
	repo := eventos.NewRepository(s.env.Mongo.DB())
	suelto := eventos.Evento{ID: bson.NewObjectID(), Titulo: "Suelto", Tipo: eventos.TipoEvento, Estado: eventos.EstadoPublicado,
		Ubicacion: eventos.Punto{Type: "Point", Coordinates: []float64{centerLng, centerLat}}, Inicio: manana, Fin: manana.Add(time.Hour), CreadoPor: &s.merchID, Version: 1}
	require.NoError(t, repo.Insertar(context.Background(), &suelto))

	priv := eventos.NewPrivacy(repo)
	out, err := priv.Export(context.Background(), s.merchID)
	require.NoError(t, err)
	raw, err := json.Marshal(out)
	require.NoError(t, err)
	assert.Contains(t, string(raw), "Del negocio")
	assert.Contains(t, string(raw), "Suelto")

	require.NoError(t, priv.PurgeAccount(context.Background(), s.merchID))
	_, err = repo.Buscar(context.Background(), suelto.ID)
	assert.ErrorIs(t, err, eventos.ErrNoEncontrado, "sin negocio se borra")
	kept, err := repo.Buscar(context.Background(), mustOID(t, delNegocio.ID))
	require.NoError(t, err, "del negocio se queda")
	assert.Nil(t, kept.CreadoPor)
}

// nv-65: publicar cuesta una escritura autenticada más la ventana `eventos_publicar`.
func TestPresupuestoRedis_EventosNV65(t *testing.T) {
	s := newEventos(t, "presupuesto65")
	manana := time.Now().Add(24 * time.Hour)
	read := commandsOf(t, func() {
		assert.Equal(t, http.StatusOK, sendJSON(t, s.app, http.MethodGet, "/api/v1/usuarios/yo", nil, s.merchant).StatusCode)
	})
	window := commandsOf(t, func() {
		assert.Equal(t, http.StatusOK, sendJSON(t, s.app, http.MethodGet, "/api/v1/personas/buscar?q=nadie", nil, s.merchant).StatusCode)
	}) - read
	write := commandsOf(t, func() {
		resp := sendJSON(t, s.app, http.MethodPatch, "/api/v1/usuarios/yo/privacidad", map[string]any{"datos_sensibles": false}, s.merchant)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
	require.Positive(t, window)
	assert.Equal(t, write+window, commandsOf(t, func() {
		status, _, _ := eventoCall(t, s.app, http.MethodPost, eventosPath, eventoBody("Presupuesto", s.negocio, manana, 0), s.merchant, "")
		assert.Equal(t, http.StatusCreated, status)
	}))
}

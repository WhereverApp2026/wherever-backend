package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/moderacion"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
)

const moderacionPath = "/api/v1/admin/moderacion"

func cuarentena(ref string, autor bson.ObjectID) moderacion.Entrada {
	return moderacion.Entrada{
		Tipo: "resena", Referencia: ref, AutorID: autor, Texto: "Llámame al 55 1234 5678",
		Resultado: moderacion.Resultado{Veredicto: moderacion.VeredictoCuarentena, Puntajes: map[string]float64{moderacion.FiltroDatosPersonales: 60}},
	}
}

// destinoMemoria guarda las decisiones que recibe, como haría el módulo dueño del recurso.
type destinoMemoria struct{ recibidos []moderacion.Item }

func (d *destinoMemoria) AlDecidir(_ context.Context, it moderacion.Item) error {
	d.recibidos = append(d.recibidos, it)
	return nil
}

func TestModeracionCola_AprobarRechazarYDobleDecision(t *testing.T) {
	env := authEnv(t)
	ctx := context.Background()
	admin := env.CreateAdmin(t, "admin-moderacion@wherever.test")
	autor := env.CreateUser(t, "autor-moderacion@wherever.test")
	destino := &destinoMemoria{}
	svc := env.Moderacion.WithDestino("resena", destino)

	a, err := svc.Encolar(ctx, cuarentena("r1", autor.ID))
	require.NoError(t, err)
	again, err := svc.Encolar(ctx, cuarentena("r1", autor.ID))
	require.NoError(t, err)
	assert.Equal(t, a.ID, again.ID, "el mismo recurso no entra dos veces mientras espera")
	b, err := svc.Encolar(ctx, cuarentena("r2", autor.ID))
	require.NoError(t, err)
	aprobado := cuarentena("r3", autor.ID)
	aprobado.Resultado.Veredicto = moderacion.VeredictoAprobado
	_, err = svc.Encolar(ctx, aprobado)
	require.ErrorIs(t, err, moderacion.ErrEntradaInvalida, "solo entra lo que quedó en cuarentena")

	page, err := svc.Listar(ctx, moderacion.ListaQuery{Estado: moderacion.EstadoPendiente, Limit: 1})
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	assert.Equal(t, a.ID.Hex(), page.Items[0].ID, "en orden de llegada")
	require.NotEmpty(t, page.NextCursor)
	page, err = svc.Listar(ctx, moderacion.ListaQuery{Estado: moderacion.EstadoPendiente, Limit: 1, Cursor: page.NextCursor})
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	assert.Equal(t, b.ID.Hex(), page.Items[0].ID)

	antes := time.Now().UTC().Add(-time.Second)
	d, err := svc.Decidir(ctx, admin.ID.Hex(), a.ID.Hex(), 1, moderacion.DecisionRequest{Decision: moderacion.EstadoAprobado})
	require.NoError(t, err)
	require.NotNil(t, d.Decision)
	assert.Equal(t, admin.ID.Hex(), d.Decision.RevisorID, "registra quién")
	assert.True(t, d.Decision.DecididoEn.After(antes), "registra cuándo")
	assert.Equal(t, 2, d.Version)
	require.Len(t, destino.recibidos, 1)
	assert.Equal(t, moderacion.EstadoAprobado, destino.recibidos[0].Estado)

	_, err = svc.Decidir(ctx, admin.ID.Hex(), a.ID.Hex(), 2, moderacion.DecisionRequest{Decision: moderacion.EstadoRechazado, Motivo: "spam"})
	assert.Equal(t, apperr.KindConflict, kindOf(t, err), "la segunda decisión es 409")

	_, err = svc.Decidir(ctx, admin.ID.Hex(), b.ID.Hex(), 1, moderacion.DecisionRequest{Decision: moderacion.EstadoRechazado, Motivo: "  "})
	assert.Equal(t, apperr.KindValidation, kindOf(t, err), "rechazar pide motivo")
	_, err = svc.Decidir(ctx, admin.ID.Hex(), b.ID.Hex(), 7, moderacion.DecisionRequest{Decision: moderacion.EstadoAprobado})
	assert.Equal(t, apperr.KindConflict, kindOf(t, err), "versión vieja = 409")
	d, err = svc.Decidir(ctx, admin.ID.Hex(), b.ID.Hex(), 1, moderacion.DecisionRequest{Decision: moderacion.EstadoRechazado, Motivo: "Comparte un teléfono"})
	require.NoError(t, err)
	assert.Equal(t, "Comparte un teléfono", d.Decision.Motivo)

	_, err = svc.Detalle(ctx, bson.NewObjectID().Hex())
	assert.Equal(t, apperr.KindNotFound, kindOf(t, err))

	// Ya decidido, el mismo recurso puede volver a la cola (por ejemplo, editado).
	c, err := svc.Encolar(ctx, cuarentena("r1", autor.ID))
	require.NoError(t, err)
	assert.NotEqual(t, a.ID, c.ID)

	// ARCO: la exportación trae el contenido sin el revisor y el borrado se lo lleva.
	privacy := moderacion.NewPrivacy(moderacion.NewRepository(env.Mongo.DB()))
	out, err := privacy.Export(ctx, autor.ID)
	require.NoError(t, err)
	raw, err := json.Marshal(out)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), admin.ID.Hex())
	require.NoError(t, privacy.PurgeAccount(ctx, autor.ID))
	n, err := env.Mongo.DB().Collection(moderacion.ColeccionCola).CountDocuments(ctx, bson.D{{Key: "autor_id", Value: autor.ID}})
	require.NoError(t, err)
	assert.Zero(t, n)
}

func TestModeracionReglas_AjusteSinReiniciar(t *testing.T) {
	env := authEnv(t)
	ctx := context.Background()
	admin := env.CreateAdmin(t, "admin-reglas@wherever.test")
	svc := env.Moderacion
	m := moderacion.New(nil, nil, svc, env.Log)

	v, err := svc.VerReglas(ctx)
	require.NoError(t, err)
	assert.True(t, v.PorDefecto)
	assert.Equal(t, 40.0, v.Cuarentena)
	_, err = svc.AjustarReglas(ctx, admin.ID.Hex(), 1, moderacion.ReglasPatch{})
	assert.Equal(t, apperr.KindNotFound, kindOf(t, err), "sin reglas guardadas se crean con POST")

	texto := moderacion.Contenido{Texto: "Aquí venden pulque adulterado"}
	assert.Equal(t, moderacion.VeredictoCuarentena, m.Evaluar(ctx, texto).Veredicto, "sin modelo, cuarentena")

	_, err = svc.CrearReglas(ctx, admin.ID.Hex(), moderacion.ReglasRequest{Cuarentena: 39, Rechazo: 80})
	assert.Equal(t, apperr.KindValidation, kindOf(t, err), "cuarentena bajo 40 no se acepta")
	creadas, err := svc.CrearReglas(ctx, admin.ID.Hex(), moderacion.ReglasRequest{Cuarentena: 50, Rechazo: 90, Palabras: []string{"Fentanilo", "fentanílo"}})
	require.NoError(t, err)
	assert.Equal(t, 1, creadas.Version)
	assert.Equal(t, []string{"Fentanilo"}, creadas.Palabras, "repetidas sin acentos ni mayúsculas se quitan")
	assert.Equal(t, admin.ID.Hex(), creadas.ActualizadoPor)
	_, err = svc.CrearReglas(ctx, admin.ID.Hex(), moderacion.ReglasRequest{Cuarentena: 50, Rechazo: 90})
	assert.Equal(t, apperr.KindConflict, kindOf(t, err))

	palabras := []string{"pulque adulterado"}
	ajustadas, err := svc.AjustarReglas(ctx, admin.ID.Hex(), 1, moderacion.ReglasPatch{Palabras: &palabras})
	require.NoError(t, err)
	assert.Equal(t, 2, ajustadas.Version)
	assert.Equal(t, 50.0, ajustadas.Cuarentena, "lo que no llega se queda igual")
	assert.Equal(t, moderacion.VeredictoRechazado, m.Evaluar(ctx, texto).Veredicto, "el Moderador usa las reglas nuevas sin reiniciar")

	_, err = svc.AjustarReglas(ctx, admin.ID.Hex(), 1, moderacion.ReglasPatch{Palabras: &palabras})
	assert.Equal(t, apperr.KindConflict, kindOf(t, err), "versión vieja = 409")
}

func sendIfMatch(t *testing.T, app *fiber.App, method, path, ifMatch string, body any, token string) (int, string) {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer "+token)
	if ifMatch != "" {
		req.Header.Set(fiber.HeaderIfMatch, ifMatch)
	}
	resp, err := app.Test(req, int((20 * time.Second).Milliseconds()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp.StatusCode, readAll(t, resp)
}

func TestModeracionAPI_DecisionPorHTTP(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	admin := env.CreateAdmin(t, "admin-api-moderacion@wherever.test")
	token := testutil.ValidTokenFor(t, admin.ID.Hex(), "jti-admin-moderacion", middleware.RoleAdmin)
	autor := env.CreateUser(t, "autor-api-moderacion@wherever.test")
	it, err := env.Moderacion.Encolar(context.Background(), cuarentena("api-1", autor.ID))
	require.NoError(t, err)
	path := moderacionPath + "/" + it.ID.Hex()

	resp := sendJSON(t, app, http.MethodGet, moderacionPath, nil, token)
	raw := readAll(t, resp)
	require.Equal(t, http.StatusOK, resp.StatusCode, raw)
	assert.Contains(t, raw, it.ID.Hex())

	resp = sendJSON(t, app, http.MethodGet, path, nil, token)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, `"1"`, resp.Header.Get(fiber.HeaderETag))

	status, raw := sendIfMatch(t, app, http.MethodPatch, path, "", map[string]string{"decision": "aprobado"}, token)
	assert.Equal(t, http.StatusBadRequest, status, raw)
	status, raw = sendIfMatch(t, app, http.MethodPatch, path, `"1"`, map[string]string{"decision": "rechazado"}, token)
	assert.Equal(t, http.StatusUnprocessableEntity, status, raw)
	status, raw = sendIfMatch(t, app, http.MethodPatch, path, `"1"`, map[string]string{"decision": "aprobado"}, token)
	require.Equal(t, http.StatusOK, status, raw)
	var d moderacion.Detalle
	require.NoError(t, json.Unmarshal([]byte(raw), &d))
	assert.Equal(t, admin.ID.Hex(), d.Decision.RevisorID)
	status, raw = sendIfMatch(t, app, http.MethodPatch, path, `"2"`, map[string]string{"decision": "aprobado"}, token)
	assert.Equal(t, http.StatusConflict, status, raw)

	status, raw = sendIfMatch(t, app, http.MethodPost, moderacionPath+"/reglas", "", map[string]any{"cuarentena": 45, "rechazo": 85, "palabras": []string{"vendo mota"}}, token)
	require.Equal(t, http.StatusCreated, status, raw)

	consumer := env.CreateUser(t, "consumidor-api-moderacion@wherever.test")
	consumerToken := testutil.ValidTokenFor(t, consumer.ID.Hex(), "jti-consumidor-moderacion", middleware.RoleConsumer)
	status, _ = sendIfMatch(t, app, http.MethodPatch, path, `"2"`, map[string]string{"decision": "rechazado", "motivo": "x"}, consumerToken)
	assert.Equal(t, http.StatusForbidden, status)
}

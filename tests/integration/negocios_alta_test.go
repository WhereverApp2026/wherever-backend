package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios/alta"
	"github.com/WhereverApp2026/wherever-backend/internal/notificaciones"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
)

const altaPath = "/api/v1/negocios"

func jsonRequest(t *testing.T, method, path string, body any, token string) *http.Request {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	if token != "" {
		req.Header.Set(fiber.HeaderAuthorization, "Bearer "+token)
	}
	return req
}

func withIfMatch(t *testing.T, app *fiber.App, method, path string, body any, token string, version int) *http.Response {
	t.Helper()
	req := jsonRequest(t, method, path, body, token)
	req.Header.Set(fiber.HeaderIfMatch, strconv.Itoa(version))
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func TestNegociosAlta_PasoAPasoRolYTope(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	u := env.CreateUser(t, "duena-alta@wherever.test")
	access, _ := loginHTTP(t, app, u.Email)

	resp := sendJSON(t, app, http.MethodPost, altaPath, map[string]any{"nombre": "Fonda Doña Mary", "categoria": "restaurante", "descripcion": "Comida corrida"}, access)
	requireStatus(t, resp, http.StatusCreated)
	body := decode(t, resp)
	assertSnakeCase(t, body)
	assert.Equal(t, negocios.StatusDraft, body["estado"])
	assert.Equal(t, true, body["requiere_refresh"])
	id := str(t, body["id"])

	stored, err := env.Users.FindByID(context.Background(), u.ID)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{middleware.RoleConsumer, middleware.RoleMerchant}, stored.Roles, "agrega comerciante sin quitar consumidor")

	// El access viejo no trae el rol: 403 hasta el refresh.
	resp = withIfMatch(t, app, http.MethodPatch, altaPath+"/"+id+"/alta/3", map[string]any{"horarios": []map[string]any{{"dia": 1, "abre": "09:00", "cierra": "18:00"}}}, access, 1)
	requireStatus(t, resp, http.StatusForbidden)
	access, _ = loginHTTP(t, app, u.Email)

	resp = withIfMatch(t, app, http.MethodPatch, altaPath+"/"+id+"/alta/3", map[string]any{"horarios": []map[string]any{{"dia": 1, "abre": "09:00", "cierra": "18:00"}}}, access, 1)
	requireStatus(t, resp, http.StatusOK)
	body = decode(t, resp)
	assert.Equal(t, float64(3), body["paso_alta"])
	assert.Equal(t, float64(2), body["version"])

	resp = withIfMatch(t, app, http.MethodPatch, altaPath+"/"+id+"/alta/3", map[string]any{"horarios": []map[string]any{{"dia": 2, "abre": "09:00", "cierra": "18:00"}}}, access, 1)
	requireStatus(t, resp, http.StatusConflict)
	resp = sendJSON(t, app, http.MethodPatch, altaPath+"/"+id+"/alta/3", map[string]any{"horarios": []map[string]any{{"dia": 2, "abre": "09:00", "cierra": "18:00"}}}, access)
	requireStatus(t, resp, http.StatusBadRequest)
	resp = withIfMatch(t, app, http.MethodPatch, altaPath+"/"+id+"/alta/7", map[string]any{}, access, 2)
	requireStatus(t, resp, http.StatusUnprocessableEntity)

	// Tope de 3 altas abiertas.
	for _, n := range []string{"Segundo", "Tercero"} {
		resp = sendJSON(t, app, http.MethodPost, altaPath, map[string]any{"nombre": n + " local", "categoria": "cafeteria"}, access)
		requireStatus(t, resp, http.StatusCreated)
		assert.Nil(t, decode(t, resp)["requiere_refresh"], "ya tenía el rol")
	}
	resp = sendJSON(t, app, http.MethodPost, altaPath, map[string]any{"nombre": "Cuarto local", "categoria": "bar"}, access)
	requireStatus(t, resp, http.StatusConflict)

	resp = sendJSON(t, app, http.MethodGet, altaPath+"/mios", nil, access)
	requireStatus(t, resp, http.StatusOK)
	assert.Len(t, list(t, decode(t, resp)["items"]), 3)
}

func TestNegociosAlta_EnviarConFaltantesYCompleto(t *testing.T) {
	env := authEnv(t)
	ctx := context.Background()
	u := env.CreateUser(t, "duena-envio@wherever.test")
	cl := authtest.Comerciante(u)

	b, err := env.Alta.Crear(ctx, cl, alta.CrearRequest{Nombre: "Cafetería Luna", Categoria: "cafeteria"})
	require.NoError(t, err)
	_, err = env.Alta.EnviarRevision(ctx, cl, b.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validacion")

	done := env.AltaCompleta(t, u, "Cafetería Sol")
	sent, err := env.Alta.EnviarRevision(ctx, cl, done.ID)
	require.NoError(t, err)
	assert.Equal(t, negocios.StatusReview, sent.Estado)
	require.NotNil(t, sent.Revision.EnviadoEn)
	assert.True(t, sent.Documentos.INE)
	assert.True(t, sent.Documentos.Comprobante)

	_, err = env.Alta.GuardarPaso(ctx, cl, done.ID, sent.Version, alta.Paso{Numero: 4})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "alta_cerrada", "en revisión ya no se edita")

	// Otro dueño ve 404.
	otro := env.CreateUser(t, "otro-envio@wherever.test")
	_, err = env.Alta.EnviarRevision(ctx, authtest.Comerciante(otro), done.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no_encontrado")
}

func TestNegociosAlta_ColaAprobarRechazarYConcurrencia(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	ctx := context.Background()
	admin := env.CreateAdmin(t, "admin-alta@wherever.test")
	adminToken, _ := loginHTTP(t, app, admin.Email)
	u := env.CreateUser(t, "duena-cola@wherever.test")
	primero := env.AltaEnRevision(t, u, "Primer local")
	segundo := env.AltaEnRevision(t, u, "Segundo local")

	resp := sendJSON(t, app, http.MethodGet, "/api/v1/admin/negocios?estado=en_revision&limite=1", nil, adminToken)
	requireStatus(t, resp, http.StatusOK)
	page := decode(t, resp)
	items := list(t, page["items"])
	require.Len(t, items, 1)
	assert.Equal(t, primero.ID, str(t, obj(t, items[0])["id"]), "del más antiguo al más nuevo")
	assert.NotContains(t, obj(t, items[0]), "datos_fiscales")
	resp = sendJSON(t, app, http.MethodGet, "/api/v1/admin/negocios?limite=1&cursor="+str(t, page["siguiente_cursor"]), nil, adminToken)
	requireStatus(t, resp, http.StatusOK)
	assert.Equal(t, segundo.ID, str(t, obj(t, list(t, decode(t, resp)["items"])[0])["id"]))

	// Rechazar sin motivo = 422; con motivo pasa a rechazado.
	resp = withIfMatch(t, app, http.MethodPatch, "/api/v1/admin/negocios/"+segundo.ID+"/revision", map[string]any{"decision": "rechazar"}, adminToken, segundo.Version)
	requireStatus(t, resp, http.StatusUnprocessableEntity)
	resp = withIfMatch(t, app, http.MethodPatch, "/api/v1/admin/negocios/"+segundo.ID+"/revision", map[string]any{"decision": "rechazar", "motivo": "La INE no se lee bien"}, adminToken, segundo.Version)
	requireStatus(t, resp, http.StatusOK)
	rechazado := decode(t, resp)
	assert.Equal(t, negocios.StatusRejected, rechazado["estado"])
	assert.Equal(t, "La INE no se lee bien", obj(t, rechazado["revision"])["motivo_rechazo"])

	// Dos administradores deciden a la vez: uno gana, el otro 409.
	var wg sync.WaitGroup
	codes := make([]int, 2)
	for i := range codes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := env.Alta.Decidir(ctx, authtest.ClaimsDe(admin.ID.Hex(), middleware.RoleAdmin), primero.ID, primero.Version, alta.DecisionRequest{Decision: alta.DecisionAprobar})
			if err == nil {
				codes[i] = http.StatusOK
			} else {
				codes[i] = http.StatusConflict
			}
		}(i)
	}
	wg.Wait()
	assert.ElementsMatch(t, []int{http.StatusOK, http.StatusConflict}, codes)

	got, err := negocios.NewRepository(env.Mongo.DB()).FindByID(ctx, mustOID(t, primero.ID))
	require.NoError(t, err)
	assert.Equal(t, negocios.StatusApproved, got.State)
	assert.True(t, got.Verified)

	notes, err := notificaciones.NewRepository(env.Mongo.DB()).ListByUser(ctx, u.ID)
	require.NoError(t, err)
	kinds := map[string]int{}
	for _, n := range notes {
		kinds[n.Kind]++
		assert.NotContains(t, n.Data, "motivo", "el aviso solo lleva ids")
	}
	assert.Equal(t, 2, kinds[notificaciones.KindBusinessReviewed])

	// El rechazado se corrige y se reenvía.
	cl := authtest.Comerciante(u)
	again, err := env.Alta.EnviarRevision(ctx, cl, segundo.ID)
	require.NoError(t, err)
	assert.Equal(t, negocios.StatusReview, again.Estado)
	assert.Empty(t, again.Revision.MotivoRechazo)
}

func mustOID(t *testing.T, hex string) bson.ObjectID {
	t.Helper()
	id, err := bson.ObjectIDFromHex(hex)
	require.NoError(t, err)
	return id
}

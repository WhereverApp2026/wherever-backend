package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/reportesnegocio"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
)

func reportarNegocio(t *testing.T, app *fiber.App, token string, id bson.ObjectID, body any) (int, string) {
	t.Helper()
	resp := sendJSON(t, app, http.MethodPost, "/api/v1/lugares/"+id.Hex()+"/reportes", body, token)
	return resp.StatusCode, readAll(t, resp)
}

func colaReportes(t *testing.T, app *fiber.App, token, query string) ([]reportesnegocio.ReporteView, string) {
	t.Helper()
	resp := sendJSON(t, app, http.MethodGet, "/api/v1/admin/reportes-negocio"+query, nil, token)
	raw := readAll(t, resp)
	require.Equal(t, http.StatusOK, resp.StatusCode, raw)
	var out struct {
		Items []reportesnegocio.ReporteView `json:"items"`
		Next  string                        `json:"siguiente_cursor"`
	}
	require.NoError(t, json.Unmarshal([]byte(raw), &out))
	return out.Items, out.Next
}

func TestReportesNegocio_CrearColaYResolver(t *testing.T) {
	env, app := amistadesEnv(t)
	me := seedAmigo(t, env, "reporta")
	other := seedAmigo(t, env, "reporta_otra")
	admin := env.CreateAdmin(t, "admin-reportes@wherever.test")
	adminToken := testutil.ValidTokenFor(t, admin.ID.Hex(), "jti-admin-reportes", middleware.RoleAdmin)
	places := insertPlaces(t, env, []placeSpec{{slug: "rep-a"}, {slug: "rep-b"}, {slug: "rep-oculto", state: negocios.StatusReview}})

	status, raw := reportarNegocio(t, app, me.token, places[0].ID, map[string]string{"motivo": "horario", "detalle": "Abren a las 10"})
	require.Equal(t, http.StatusCreated, status, raw)
	assert.Contains(t, raw, `"estado":"abierto"`)
	status, _ = reportarNegocio(t, app, me.token, places[0].ID, map[string]string{"motivo": "precio"})
	assert.Equal(t, http.StatusConflict, status, "otro abierto en el mismo negocio")
	status, _ = reportarNegocio(t, app, me.token, places[1].ID, map[string]string{"motivo": "otro"})
	assert.Equal(t, http.StatusUnprocessableEntity, status, "otro sin detalle")
	status, _ = reportarNegocio(t, app, me.token, places[2].ID, map[string]string{"motivo": "horario"})
	assert.Equal(t, http.StatusNotFound, status, "no visible")
	status, _ = reportarNegocio(t, app, me.token, places[1].ID, map[string]any{"motivo": "horario", "extra": true})
	assert.Equal(t, http.StatusBadRequest, status, "campo de más")
	status, raw = reportarNegocio(t, app, other.token, places[1].ID, map[string]string{"motivo": "cerrado_permanente"})
	require.Equal(t, http.StatusCreated, status, raw)

	items, _ := colaReportes(t, app, adminToken, "")
	require.Len(t, items, 2)
	assert.Equal(t, "Lugar rep-a", items[0].Negocio, "del más viejo al más nuevo")
	assert.Equal(t, me.id(), items[0].UsuarioID)
	only, _ := colaReportes(t, app, adminToken, "?negocio_id="+places[1].ID.Hex())
	require.Len(t, only, 1)
	page, next := colaReportes(t, app, adminToken, "?limite=1")
	require.Len(t, page, 1)
	require.NotEmpty(t, next)
	rest, _ := colaReportes(t, app, adminToken, "?limite=1&cursor="+next)
	require.Len(t, rest, 1)
	assert.NotEqual(t, page[0].ID, rest[0].ID)

	path := "/api/v1/admin/reportes-negocio/" + items[0].ID
	got := planCall(t, app, http.MethodPatch, path, map[string]string{"estado": "corregido"}, adminToken, "")
	assert.Equal(t, http.StatusBadRequest, got.status, "sin If-Match")
	got = planCall(t, app, http.MethodPatch, path, map[string]string{"estado": "corregido"}, adminToken, "2")
	assert.Equal(t, http.StatusConflict, got.status, "versión vieja")
	got = planCall(t, app, http.MethodPatch, path, map[string]string{"estado": "corregido"}, adminToken, "1")
	require.Equal(t, http.StatusOK, got.status, got.body)
	assert.Contains(t, got.body, `"resuelto_por":"`+admin.ID.Hex()+`"`)
	got = planCall(t, app, http.MethodPatch, path, map[string]string{"estado": "descartado"}, adminToken, "2")
	assert.Equal(t, http.StatusConflict, got.status, "ya resuelto")
	resolved, _ := colaReportes(t, app, adminToken, "?estado=corregido")
	assert.Len(t, resolved, 1)

	// Resuelto, se puede volver a reportar el mismo negocio.
	status, raw = reportarNegocio(t, app, me.token, places[0].ID, map[string]string{"motivo": "precio"})
	assert.Equal(t, http.StatusCreated, status, raw)
}

func TestReportesNegocio_DuenoYTopeDiario(t *testing.T) {
	env, app := amistadesEnv(t)
	duena := env.CreateUser(t, "reportes-duena@wherever.test")
	b := env.AltaAprobada(t, duena, "Reportes propio")
	ownerToken := testutil.ValidTokenFor(t, duena.ID.Hex(), "jti-reportes-duena", middleware.RoleConsumer, middleware.RoleMerchant)
	status, _ := reportarNegocio(t, app, ownerToken, mustOID(t, b.ID), map[string]string{"motivo": "horario"})
	assert.Equal(t, http.StatusForbidden, status, "el dueño no reporta su negocio")

	me := seedAmigo(t, env, "reportes_tope")
	specs := make([]placeSpec, 0, reportesnegocio.MaxPorDia+1)
	for i := range reportesnegocio.MaxPorDia + 1 {
		specs = append(specs, placeSpec{slug: "rep-tope-" + strconv.Itoa(i)})
	}
	places := insertPlaces(t, env, specs)
	for i := range reportesnegocio.MaxPorDia {
		status, raw := reportarNegocio(t, app, me.token, places[i].ID, map[string]string{"motivo": "precio"})
		require.Equal(t, http.StatusCreated, status, raw)
	}
	resp := sendJSON(t, app, http.MethodPost, "/api/v1/lugares/"+places[reportesnegocio.MaxPorDia].ID.Hex()+"/reportes", map[string]string{"motivo": "precio"}, me.token)
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.NotEmpty(t, resp.Header.Get(fiber.HeaderRetryAfter))
}

func TestPrivacidadReportes_ExportarYBorrarCuenta(t *testing.T) {
	env, app := amistadesEnv(t)
	me := seedAmigo(t, env, "reportes_arco")
	places := insertPlaces(t, env, []placeSpec{{slug: "rep-arco"}})
	status, raw := reportarNegocio(t, app, me.token, places[0].ID, map[string]string{"motivo": "ubicacion", "detalle": "Está en la otra esquina"})
	require.Equal(t, http.StatusCreated, status, raw)

	repo := reportesnegocio.NewRepository(env.Mongo.DB())
	priv := reportesnegocio.NewPrivacy(repo, negocios.NewRepository(env.Mongo.DB()))
	out, err := priv.Export(context.Background(), me.user.ID)
	require.NoError(t, err)
	b, err := json.Marshal(out)
	require.NoError(t, err)
	assert.Contains(t, string(b), "Lugar rep-arco")
	assert.Contains(t, string(b), "otra esquina")

	require.NoError(t, priv.PurgeHistory(context.Background(), me.user.ID))
	rows, err := repo.DelUsuario(context.Background(), me.user.ID)
	require.NoError(t, err)
	assert.Len(t, rows, 1, "borrar historial no los toca")
	require.NoError(t, priv.PurgeAccount(context.Background(), me.user.ID))
	rows, err = repo.DelUsuario(context.Background(), me.user.ID)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

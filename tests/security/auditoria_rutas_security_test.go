package security

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// Rutas que la auditoría del 2026-11-20 encontró sin prueba de seguridad propia: sincronizar
// pendientes, cerrar una sesión, inicio, eventos públicos, autocompletar, configuración, /ready,
// la documentación y las sondas. Lo esperado sale de docs/endpoints.md.

var (
	rutaPendientes = ruta{http.MethodPost, "/api/v1/sync/pendientes"}
	rutaSesiones   = ruta{http.MethodGet, "/api/v1/auth/sesiones"}
)

func pendientesBody(acciones ...map[string]any) map[string]any {
	return map[string]any{"acciones": acciones}
}

func sesionIDs(t *testing.T, r reply) []string {
	t.Helper()
	require.Equal(t, http.StatusOK, r.status, r.body)
	var out []struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal([]byte(r.body), &out), r.body)
	ids := make([]string, 0, len(out))
	for _, s := range out {
		ids = append(ids, s.ID)
	}
	return ids
}

// Sin token, vencido, sin firma o con alg none: 401 al sincronizar y al cerrar una sesión.
func TestAuditoriaSecurity_PendientesYSesionSinTokenValidoEs401(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	cerrar := ruta{http.MethodDelete, "/api/v1/auth/sesiones/" + bson.NewObjectID().Hex()}
	body := pendientesBody(map[string]any{"id_cliente": "accion-0001", "tipo": "privacidad.editar", "datos": map[string]any{"ubicacion": true}})
	for name, token := range map[string]string{
		"sin token": "",
		"vencido":   testutil.ExpiredToken(t),
		"sin firma": testutil.UnsignedToken(t),
		"alg none":  testutil.AlgNoneToken(t),
	} {
		assert.Equal(t, http.StatusUnauthorized, send(t, app, rutaPendientes.method, rutaPendientes.path, body, token).status, "pendientes %s", name)
		assert.Equal(t, http.StatusUnauthorized, send(t, app, cerrar.method, cerrar.path, nil, token).status, "cerrar sesión %s", name)
	}
}

// Con la cuenta en eliminación pendiente no se aplica nada sin conexión: 403.
func TestAuditoriaSecurity_PendientesConCuentaPendienteEs403(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	u := env.CreateUser(t, "pendiente-sync@wherever.test")
	access, _ := tokens(t, login(t, app, u.Email, authtest.Password))
	_, err := env.Users.Update(context.Background(), u.ID, bson.D{{Key: "estado", Value: usuarios.StatusPendingDeletion}})
	require.NoError(t, err)
	r := send(t, app, rutaPendientes.method, rutaPendientes.path,
		pendientesBody(map[string]any{"id_cliente": "accion-0001", "tipo": "privacidad.editar", "datos": map[string]any{"ubicacion": true}}), access)
	assert.Equal(t, http.StatusForbidden, r.status, r.body)
}

// Un lote no puede tocar el plan de otra persona: la acción queda rechazada con 404 y el plan
// sigue igual; `id_cliente` es por persona, así que repetir el de otra no revela su resultado.
func TestAuditoriaSecurity_PendientesNoTocaLoAjeno(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	ana := env.CreateUser(t, "ana-pendientes@wherever.test")
	beto := env.CreateUser(t, "beto-pendientes@wherever.test")
	anaToken, _ := tokens(t, login(t, app, ana.Email, authtest.Password))
	betoToken, _ := tokens(t, login(t, app, beto.Email, authtest.Password))
	place := lugarVisible(t, env, "pendientes-ajeno")
	siempre := make([]negocios.VentanaMin, 0, 7)
	for d := range 7 {
		siempre = append(siempre, negocios.VentanaMin{Dia: d, Desde: 0, Hasta: 24 * 60})
	}
	_, err := env.Mongo.DB().Collection(negocios.Collection).UpdateByID(context.Background(), place.ID, bson.D{{Key: "$set", Value: bson.D{{Key: "horario_min", Value: siempre}}}})
	require.NoError(t, err)
	day := time.Now().AddDate(0, 0, 7).Format("2006-01-02")
	plan := map[string]any{
		"nombre": "De Ana", "fecha": day, "personas": 2, "presupuesto": 500, "movilidad": "a_pie",
		"paradas": []map[string]any{{"negocio_id": place.ID.Hex(), "llega": day + "T12:00:00-06:00", "minutos": 60, "precio": 100}},
	}

	creado := send(t, app, rutaPendientes.method, rutaPendientes.path, pendientesBody(map[string]any{"id_cliente": "compartido-01", "tipo": "plan.crear", "datos": plan}), anaToken)
	require.Equal(t, http.StatusOK, creado.status, creado.body)
	var out struct {
		Resultados []struct {
			Resultado string `json:"resultado"`
			Original  string `json:"resultado_original"`
			PlanID    string `json:"plan_id"`
			Error     *struct {
				Codigo string `json:"codigo"`
			} `json:"error"`
		} `json:"resultados"`
	}
	require.NoError(t, json.Unmarshal([]byte(creado.body), &out))
	require.Len(t, out.Resultados, 1)
	require.Equal(t, "aplicada", out.Resultados[0].Resultado, creado.body)
	planID := out.Resultados[0].PlanID

	ajeno := send(t, app, rutaPendientes.method, rutaPendientes.path, pendientesBody(
		map[string]any{"id_cliente": "ajeno-borrar-01", "tipo": "plan.borrar", "plan_id": planID, "version": 1},
		map[string]any{"id_cliente": "ajeno-editar-01", "tipo": "plan.editar", "plan_id": planID, "version": 1, "datos": map[string]any{"nombre": "Ya no es de Ana"}},
		map[string]any{"id_cliente": "compartido-01", "tipo": "privacidad.editar", "datos": map[string]any{"ubicacion": true}},
	), betoToken)
	require.Equal(t, http.StatusOK, ajeno.status, ajeno.body)
	require.NoError(t, json.Unmarshal([]byte(ajeno.body), &out))
	require.Len(t, out.Resultados, 3)
	for _, r := range out.Resultados[:2] {
		assert.Equal(t, "rechazada", r.Resultado, ajeno.body)
		require.NotNil(t, r.Error)
		assert.Equal(t, "no_encontrado", r.Error.Codigo)
	}
	assert.Equal(t, "aplicada", out.Resultados[2].Resultado, "el id_cliente de Ana no cuenta para Beto")
	assert.Empty(t, out.Resultados[2].Original)
	assert.NotContains(t, ajeno.body, "De Ana")

	propio := send(t, app, http.MethodGet, "/api/v1/planes/"+planID, nil, anaToken)
	require.Equal(t, http.StatusOK, propio.status, propio.body)
	assert.Contains(t, propio.body, "De Ana", "el plan de Ana sigue igual")
}

// El lote tiene su límite por persona (30 por minuto): el 31.º da 429 con Retry-After.
func TestAuditoriaSecurity_PendientesLimitePorPersona(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	u := env.CreateUser(t, "limite-sync@wherever.test")
	token, _ := tokens(t, login(t, app, u.Email, authtest.Password))
	require.NoError(t, env.Redis.Set(context.Background(), "rate:sync_pendientes:"+u.ID.Hex(), 30, time.Minute).Err())
	r := send(t, app, rutaPendientes.method, rutaPendientes.path,
		pendientesBody(map[string]any{"id_cliente": "accion-0001", "tipo": "privacidad.editar", "datos": map[string]any{"ubicacion": true}}), token)
	assert.Equal(t, http.StatusTooManyRequests, r.status, r.body)
	assert.NotEmpty(t, r.headers.Get("Retry-After"))
}

// Solo se cierra una sesión propia: la de otra persona, una inexistente o un id malo dan 404 y
// la sesión ajena sigue abierta.
func TestAuditoriaSecurity_CerrarSesionSoloLaPropia(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	ana := env.CreateUser(t, "ana-sesion@wherever.test")
	beto := env.CreateUser(t, "beto-sesion@wherever.test")
	anaToken, _ := tokens(t, login(t, app, ana.Email, authtest.Password))
	betoToken, _ := tokens(t, login(t, app, beto.Email, authtest.Password))
	anaSesiones := sesionIDs(t, send(t, app, rutaSesiones.method, rutaSesiones.path, nil, anaToken))
	require.NotEmpty(t, anaSesiones)

	for _, id := range []string{anaSesiones[0], bson.NewObjectID().Hex(), "no-es-id", "%7B%22%24ne%22%3Anull%7D"} {
		r := send(t, app, http.MethodDelete, "/api/v1/auth/sesiones/"+id, nil, betoToken)
		assert.Equal(t, http.StatusNotFound, r.status, "%s: %s", id, r.body)
	}
	assert.Equal(t, anaSesiones, sesionIDs(t, send(t, app, rutaSesiones.method, rutaSesiones.path, nil, anaToken)), "la sesión de Ana sigue abierta")
	assert.Equal(t, http.StatusOK, send(t, app, http.MethodGet, "/api/v1/usuarios/yo", nil, anaToken).status)
}

// Inicio es de sesión opcional: sin token responde; un token inválido es 401 (no se trata como
// anónimo); parámetros malos son 422, nunca 500; no expone datos de la cuenta.
func TestAuditoriaSecurity_InicioSesionOpcional(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	const path = "/api/v1/inicio?lat=19.43&lng=-99.13"
	assert.Equal(t, http.StatusOK, send(t, app, http.MethodGet, path, nil, "").status)
	for name, token := range map[string]string{
		"vencido":   testutil.ExpiredToken(t),
		"sin firma": testutil.UnsignedToken(t),
		"alg none":  testutil.AlgNoneToken(t),
	} {
		assert.Equal(t, http.StatusUnauthorized, send(t, app, http.MethodGet, path, nil, token).status, name)
	}
	for _, q := range []string{"lat=91&lng=0", "lat=19.4", "lat=19.43&lng=-99.13&hora=99:99", "lat=19.43&lng=-99.13&compania=%7B%22%24ne%22%3Anull%7D"} {
		r := send(t, app, http.MethodGet, "/api/v1/inicio?"+q, nil, "")
		assert.Contains(t, []int{http.StatusBadRequest, http.StatusUnprocessableEntity}, r.status, "%s: %s", q, r.body)
	}
	u := env.CreateUser(t, "inicio-auditoria@wherever.test")
	token, _ := tokens(t, login(t, app, u.Email, authtest.Password))
	r := send(t, app, http.MethodGet, path, nil, token)
	require.Equal(t, http.StatusOK, r.status, r.body)
	assert.NotContains(t, r.body, u.Email)
	assert.NotContains(t, r.body, u.ID.Hex())
	assert.Equal(t, "no-store", r.headers.Get("Cache-Control"))
}

// La lista pública de eventos pide coordenadas, rechaza parámetros malos sin 500 y no trae
// quién creó cada evento.
func TestAuditoriaSecurity_EventosPublicos(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	for q, want := range map[string]int{
		"":                                   http.StatusUnprocessableEntity,
		"lat=19.43":                          http.StatusUnprocessableEntity,
		"lat=91&lng=0":                       http.StatusUnprocessableEntity,
		"lat=19.43&lng=-99.13&radio_m=50001": http.StatusUnprocessableEntity,
		"lat=19.43&lng=-99.13&limite=51":     http.StatusUnprocessableEntity,
		"lat=19.43&lng=-99.13&cursor=abc":    http.StatusUnprocessableEntity,
	} {
		r := send(t, app, http.MethodGet, "/api/v1/eventos?"+q, nil, "")
		assert.Equal(t, want, r.status, "%s: %s", q, r.body)
	}
	tipo := send(t, app, http.MethodGet, "/api/v1/eventos?lat=19.43&lng=-99.13&tipo=%7B%22%24ne%22%3Anull%7D", nil, "")
	assert.Contains(t, []int{http.StatusBadRequest, http.StatusUnprocessableEntity}, tipo.status, tipo.body)

	ok := send(t, app, http.MethodGet, "/api/v1/eventos?lat=19.43&lng=-99.13", nil, "")
	require.Equal(t, http.StatusOK, ok.status, ok.body)
	for _, campo := range []string{"creado_por", "usuario_id", "correo"} {
		assert.NotContains(t, ok.body, campo)
	}
}

// Autocompletar solo muestra negocios aprobados y valida `q` y `limite`.
func TestAuditoriaSecurity_AutocompletarSoloAprobados(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	negocio := func(slug, name, state string) negocios.Business {
		b := negocios.Business{ID: bson.NewObjectID(), Slug: slug, QRCheckin: "qr-" + slug, Name: name, NormalizedName: strings.ToLower(name),
			Category: "restaurante", State: state, Location: negocios.Location{Type: "Point", Coordinates: []float64{-99.13, 19.43}}, CreatedAt: time.Now()}
		_, err := env.Mongo.DB().Collection(negocios.Collection).InsertOne(context.Background(), b)
		require.NoError(t, err)
		return b
	}
	aprobado := negocio("zarzamora-aprobada", "Zarzamora Aprobada", negocios.StatusApproved)
	oculto := negocio("zarzamora-oculta", "Zarzamora Oculta", "pendiente")

	r := send(t, app, http.MethodGet, "/api/v1/lugares/autocompletar?"+url.Values{"q": {"zarza"}}.Encode(), nil, "")
	require.Equal(t, http.StatusOK, r.status, r.body)
	assert.Contains(t, r.body, aprobado.ID.Hex())
	assert.NotContains(t, r.body, oculto.ID.Hex(), "un negocio no aprobado nunca sale")

	for q, want := range map[string]int{
		"q=a":                         http.StatusUnprocessableEntity,
		"q=" + strings.Repeat("a", 61): http.StatusUnprocessableEntity,
		"q=zarza&limite=21":           http.StatusUnprocessableEntity,
		"q=zarza&limite=0":            http.StatusUnprocessableEntity,
	} {
		resp := send(t, app, http.MethodGet, "/api/v1/lugares/autocompletar?"+q, nil, "")
		assert.Equal(t, want, resp.status, "%s: %s", q, resp.body)
	}
	inyeccion := send(t, app, http.MethodGet, "/api/v1/lugares/autocompletar?q=%28%2A%2B%3F%5B", nil, "")
	assert.NotEqual(t, http.StatusInternalServerError, inyeccion.status, "una expresión regular rota no tumba la ruta")
}

// Configuración es pública y exenta del 426: solo trae versiones y el modo mantenimiento.
func TestAuditoriaSecurity_ConfiguracionSoloVersiones(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	r := send(t, app, http.MethodGet, "/api/v1/configuracion", nil, "")
	require.Equal(t, http.StatusOK, r.status, r.body)
	assert.ElementsMatch(t, []string{"version_minima_app", "version_actual_app", "version_terminos", "version_aviso_privacidad", "mantenimiento"}, keys(jsonObj(t, r.body)))
	for _, secreto := range []string{env.Cfg.MongoURI, "mongodb", "redis", "neo4j", "clave", "secret"} {
		if secreto != "" {
			assert.NotContains(t, strings.ToLower(r.body), strings.ToLower(secreto))
		}
	}
}

// /ready solo dice el estado de cada dependencia: sin cadenas de conexión, hosts ni errores.
func TestAuditoriaSecurity_ReadySinDetallesInternos(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	r := send(t, app, http.MethodGet, "/ready", nil, "")
	require.Contains(t, []int{http.StatusOK, http.StatusServiceUnavailable}, r.status)
	assert.ElementsMatch(t, []string{"estado", "dependencias"}, keys(jsonObj(t, r.body)))
	for _, interno := range []string{"mongodb://", "redis://", "neo4j://", "bolt://", "localhost", "127.0.0.1", "error:"} {
		assert.NotContains(t, r.body, interno)
	}
}

// En producción no existen la documentación ni las sondas de errores: 404. Fuera de prod sí
// existen y las sondas siguen el formato uniforme de error, sin trazas.
func TestAuditoriaSecurity_DocsYSondasSoloFueraDeProd(t *testing.T) {
	prod := authEnv(t)
	prod.ClearRateLimits(t)
	app := prod.App()
	rutas := []string{"/docs", "/openapi.json", "/api/v1/probe/error", "/api/v1/probe/conflict", "/api/v1/probe/invalid", "/api/v1/probe/timeout", "/api/v1/probe/lenta"}
	for _, path := range rutas {
		assert.Equal(t, http.StatusNotFound, send(t, app, http.MethodGet, path, nil, "").status, path)
	}

	dev := authtest.New(t, mongoClient, redisClient, testutil.Config(config.EnvDev))
	dev.ClearRateLimits(t)
	devApp := dev.App()
	assert.Equal(t, http.StatusOK, send(t, devApp, http.MethodGet, "/openapi.json", nil, "").status)
	for _, path := range []string{"/api/v1/probe/error", "/api/v1/probe/conflict", "/api/v1/probe/invalid"} {
		r := send(t, devApp, http.MethodGet, path, nil, "")
		assert.GreaterOrEqual(t, r.status, 400, path)
		body := jsonObj(t, r.body)
		assert.Contains(t, body, "codigo", path)
		assert.NotContains(t, r.body, "goroutine", path)
		assert.NotContains(t, r.body, ".go:", path)
	}
}

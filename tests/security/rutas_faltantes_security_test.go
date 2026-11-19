package security

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/config"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// Rutas que no tenían prueba de seguridad propia (auditoría 2026-11-19). Lo que se prueba
// sale de docs/endpoints.md: las de JWT dan 401 sin token o con uno inválido y 403 con la
// cuenta en eliminación pendiente; las públicas (conteo, afluencia, clima y la semántica,
// de sesión opcional) prueban límite de peticiones, parámetros malos y que no filtren datos.

type ruta struct{ method, path string }

func (r ruta) String() string { return r.method + " " + r.path }

var (
	rutaPreferencias = ruta{http.MethodPatch, "/api/v1/usuarios/yo/preferencias"}
	rutaConsent      = ruta{http.MethodPost, "/api/v1/usuarios/yo/consentimientos"}
	rutaModo         = ruta{http.MethodPost, "/api/v1/usuarios/yo/modo"}
	rutaHistorial    = ruta{http.MethodDelete, "/api/v1/usuarios/yo/historial"}
	rutaSync         = ruta{http.MethodGet, "/api/v1/sync/cambios"}
	rutaRecientes    = ruta{http.MethodGet, "/api/v1/busquedas/recientes"}
	rutaBorrarRec    = ruta{http.MethodDelete, "/api/v1/busquedas/recientes"}
	rutaLogoutTodos  = ruta{http.MethodPost, "/api/v1/auth/logout-todos"}
)

func jsonObj(t *testing.T, raw string) map[string]any {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &out), raw)
	return out
}

// Sin token, vencido, sin firma o con alg none: 401 en todas las rutas con JWT.
func TestRutasFaltantesSecurity_TokenAusenteOInvalidoEs401(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	tokens := map[string]string{
		"sin token": "",
		"vencido":   testutil.ExpiredToken(t),
		"sin firma": testutil.UnsignedToken(t),
		"alg none":  testutil.AlgNoneToken(t),
	}
	for _, r := range []ruta{rutaPreferencias, rutaConsent, rutaModo, rutaHistorial, rutaSync, rutaRecientes, rutaBorrarRec, rutaLogoutTodos} {
		for name, token := range tokens {
			assert.Equal(t, http.StatusUnauthorized, send(t, app, r.method, r.path, nil, token).status, "%s %s", r, name)
		}
	}
}

// Con la cuenta en eliminación pendiente las rutas con guardia de cuenta activa dan 403;
// logout-todos sigue abierto (docs/endpoints.md, «Cuenta en eliminación pendiente»).
func TestRutasFaltantesSecurity_CuentaPendienteEs403SalvoLogoutTodos(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	u := env.CreateUser(t, "pendiente-faltantes@wherever.test")
	access, _ := tokens(t, login(t, app, u.Email, authtest.Password))
	_, err := env.Users.Update(context.Background(), u.ID, bson.D{{Key: "estado", Value: usuarios.StatusPendingDeletion}})
	require.NoError(t, err)

	bodies := map[ruta]any{
		rutaPreferencias: map[string]string{"tema": "oscuro"},
		rutaConsent:      map[string]string{"version_terminos": env.Cfg.VersionApp.Terminos, "version_aviso_privacidad": env.Cfg.VersionApp.Aviso},
		rutaModo:         map[string]string{"modo": "explorar"},
	}
	for _, r := range []ruta{rutaPreferencias, rutaConsent, rutaModo, rutaHistorial, rutaRecientes, rutaBorrarRec} {
		assert.Equal(t, http.StatusForbidden, send(t, app, r.method, r.path, bodies[r], access).status, r.String())
	}
	assert.Equal(t, http.StatusNoContent, send(t, app, rutaLogoutTodos.method, rutaLogoutTodos.path, nil, access).status)
}

// Modo negocio sin el rol comerciante: 403 (otro rol).
func TestRutasFaltantesSecurity_ModoNegocioSinRolComercianteEs403(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	u := env.CreateUser(t, "modo-faltantes@wherever.test")
	token := testutil.ValidTokenFor(t, u.ID.Hex(), "jti-modo-faltantes", middleware.RoleConsumer)

	assert.Equal(t, http.StatusForbidden, send(t, app, rutaModo.method, rutaModo.path, map[string]string{"modo": "negocio"}, token).status)
	assert.Equal(t, http.StatusOK, send(t, app, rutaModo.method, rutaModo.path, map[string]string{"modo": "explorar"}, token).status)
}

// Preferencias y consentimiento salen del `sub` firmado: lo que manda Beto, aunque traiga un
// id ajeno en el cuerpo, no toca el perfil de Ana.
func TestRutasFaltantesSecurity_PreferenciasYConsentimientoSoloDelDueno(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	ana := env.CreateUser(t, "ana-faltantes@wherever.test")
	beto := env.CreateUser(t, "beto-faltantes@wherever.test")
	anaToken := testutil.ValidTokenFor(t, ana.ID.Hex(), "jti-ana-faltantes", middleware.RoleConsumer)
	betoToken := testutil.ValidTokenFor(t, beto.ID.Hex(), "jti-beto-faltantes", middleware.RoleConsumer)
	perfil := func() map[string]any {
		t.Helper()
		r := send(t, app, http.MethodGet, "/api/v1/usuarios/yo", nil, anaToken)
		require.Equal(t, http.StatusOK, r.status, r.body)
		return jsonObj(t, r.body)
	}
	antes := perfil()

	r := send(t, app, rutaPreferencias.method, rutaPreferencias.path, map[string]any{"tema": "oscuro", "unidades": "mi", "id": ana.ID.Hex(), "usuario_id": ana.ID.Hex()}, betoToken)
	require.Equal(t, http.StatusOK, r.status, r.body)
	r = send(t, app, rutaConsent.method, rutaConsent.path, map[string]any{
		"version_terminos": env.Cfg.VersionApp.Terminos, "version_aviso_privacidad": env.Cfg.VersionApp.Aviso, "promociones_correo": true, "usuario_id": ana.ID.Hex(),
	}, betoToken)
	require.Equal(t, http.StatusCreated, r.status, r.body)
	assert.NotContains(t, r.body, ana.ID.Hex())

	despues := perfil()
	assert.Equal(t, antes["preferencias"], despues["preferencias"], "las preferencias de Beto no tocan a Ana")
	assert.Equal(t, antes["privacidad"], despues["privacidad"], "el consentimiento de Beto no toca a Ana")
}

// Búsquedas recientes e historial: nadie ve ni borra las de otro.
func TestRutasFaltantesSecurity_RecientesEHistorialSoloDelDueno(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	ana := env.CreateUser(t, "ana-recientes@wherever.test")
	beto := env.CreateUser(t, "beto-recientes@wherever.test")
	anaToken := testutil.ValidTokenFor(t, ana.ID.Hex(), "jti-ana-recientes", middleware.RoleConsumer)
	betoToken := testutil.ValidTokenFor(t, beto.ID.Hex(), "jti-beto-recientes", middleware.RoleConsumer)
	const frase = "farmacia-intima-de-ana"
	require.Equal(t, http.StatusOK, send(t, app, http.MethodGet, "/api/v1/lugares/buscar?"+url.Values{"q": {frase}}.Encode(), nil, anaToken).status)
	deAna := func() string {
		t.Helper()
		r := send(t, app, rutaRecientes.method, rutaRecientes.path, nil, anaToken)
		require.Equal(t, http.StatusOK, r.status, r.body)
		return r.body
	}
	require.Contains(t, deAna(), frase)

	r := send(t, app, rutaRecientes.method, rutaRecientes.path, nil, betoToken)
	require.Equal(t, http.StatusOK, r.status)
	assert.NotContains(t, r.body, frase, "Beto no ve las búsquedas de Ana")
	assert.Equal(t, "no-store", r.headers.Get("Cache-Control"))

	require.Equal(t, http.StatusNoContent, send(t, app, rutaBorrarRec.method, rutaBorrarRec.path, nil, betoToken).status)
	require.Equal(t, http.StatusNoContent, send(t, app, rutaHistorial.method, rutaHistorial.path, nil, betoToken).status)
	assert.Contains(t, deAna(), frase, "borrar lo de Beto no borra lo de Ana")
}

// El cursor de sincronización va firmado y ligado a su dueño: el de Ana no le sirve a Beto
// (422), uno alterado tampoco, y la descarga de Beto no trae nada de Ana.
func TestRutasFaltantesSecurity_SyncCursorAjenoOAlteradoEs422(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	ana := env.CreateUser(t, "ana-sync@wherever.test")
	beto := env.CreateUser(t, "beto-sync@wherever.test")
	anaToken := testutil.ValidTokenFor(t, ana.ID.Hex(), "jti-ana-sync", middleware.RoleConsumer)
	betoToken := testutil.ValidTokenFor(t, beto.ID.Hex(), "jti-beto-sync", middleware.RoleConsumer)

	r := send(t, app, rutaSync.method, rutaSync.path, nil, anaToken)
	require.Equal(t, http.StatusOK, r.status, r.body)
	assert.Equal(t, "no-store", r.headers.Get("Cache-Control"))
	siguiente := str(t, jsonObj(t, r.body)["siguiente"])
	require.NotEmpty(t, siguiente)

	ajeno := send(t, app, rutaSync.method, rutaSync.path+"?"+url.Values{"desde": {siguiente}}.Encode(), nil, betoToken)
	assert.Equal(t, http.StatusUnprocessableEntity, ajeno.status, ajeno.body)
	for name, malo := range cursoresAlterados(t, siguiente) {
		alterado := send(t, app, rutaSync.method, rutaSync.path+"?"+url.Values{"desde": {malo}}.Encode(), nil, anaToken)
		assert.Equal(t, http.StatusUnprocessableEntity, alterado.status, "%s: %s", name, alterado.body)
	}

	propia := send(t, app, rutaSync.method, rutaSync.path, nil, betoToken)
	require.Equal(t, http.StatusOK, propia.status)
	assert.NotContains(t, propia.body, ana.ID.Hex())
}

// logout-todos cierra solo las sesiones de quien lo pide, y todas: sus access tokens y sus
// refresh dejan de servir; los de otra persona siguen.
func TestRutasFaltantesSecurity_LogoutTodosSoloLasPropias(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	ana := env.CreateUser(t, "ana-logout@wherever.test")
	beto := env.CreateUser(t, "beto-logout@wherever.test")
	ana1, anaRefresh := tokens(t, login(t, app, ana.Email, authtest.Password))
	ana2, _ := tokens(t, login(t, app, ana.Email, authtest.Password))
	betoAccess, _ := tokens(t, login(t, app, beto.Email, authtest.Password))
	yo := func(token string) int {
		return send(t, app, http.MethodGet, "/api/v1/usuarios/yo", nil, token).status
	}

	require.Equal(t, http.StatusNoContent, send(t, app, rutaLogoutTodos.method, rutaLogoutTodos.path, nil, betoAccess).status)
	assert.Equal(t, http.StatusUnauthorized, yo(betoAccess))
	assert.Equal(t, http.StatusOK, yo(ana1), "cerrar las de Beto no toca a Ana")

	require.Equal(t, http.StatusNoContent, send(t, app, rutaLogoutTodos.method, rutaLogoutTodos.path, nil, ana1).status)
	assert.Equal(t, http.StatusUnauthorized, yo(ana1))
	assert.Equal(t, http.StatusUnauthorized, yo(ana2), "también la otra sesión")
	assert.Equal(t, http.StatusUnauthorized, send(t, app, http.MethodPost, "/api/v1/auth/refresh", map[string]string{"refresh_token": anaRefresh}, "").status)
}

// Conteo, afluencia y clima son públicas: el límite global las cubre con 429 y Retry-After.
func TestRutasFaltantesSecurity_PublicasConLimiteGlobal(t *testing.T) {
	cfg := testutil.Config(config.EnvProd)
	cfg.RateLimitGlobal = 3
	env := authtest.New(t, mongoClient, redisClient, cfg)
	app := env.App()
	place := lugarVisible(t, env, "afluencia-limite")
	for _, path := range []string{"/api/v1/lugares/conteo", "/api/v1/lugares/" + place.ID.Hex() + "/afluencia", "/api/v1/contexto/clima?lat=19.43&lng=-99.13"} {
		env.ClearRateLimits(t)
		for range cfg.RateLimitGlobal {
			require.Equal(t, http.StatusOK, send(t, app, http.MethodGet, path, nil, "").status, path)
		}
		over := send(t, app, http.MethodGet, path, nil, "")
		assert.Equal(t, http.StatusTooManyRequests, over.status, path)
		assert.NotEmpty(t, over.headers.Get("Retry-After"), path)
	}
}

// Parámetros malos en las públicas: 400/404/422, nunca 500; y sus respuestas no traen datos
// de personas (solo totales y barras agregadas).
func TestRutasFaltantesSecurity_PublicasParametrosMalosYSinDatosPersonales(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	place := lugarVisible(t, env, "afluencia-datos")
	afluencia := "/api/v1/lugares/" + place.ID.Hex() + "/afluencia"
	for path, want := range map[string]int{
		"/api/v1/lugares/conteo?cursor=abc":                          http.StatusBadRequest,
		"/api/v1/lugares/conteo?categoria=%7B%22%24ne%22%3Anull%7D":  http.StatusUnprocessableEntity,
		"/api/v1/lugares/conteo?lat=19.4":                            http.StatusUnprocessableEntity,
		afluencia + "?dia=-1":                                        http.StatusUnprocessableEntity,
		afluencia + "?dia=7":                                         http.StatusUnprocessableEntity,
		"/api/v1/lugares/%7B%22%24ne%22%3Anull%7D/afluencia":         http.StatusNotFound,
		"/api/v1/lugares/" + bson.NewObjectID().Hex() + "/afluencia": http.StatusNotFound,
		"/api/v1/contexto/clima?ciudad_id=%7B%22%24ne%22%3Anull%7D":  http.StatusUnprocessableEntity,
		"/api/v1/contexto/clima?lat=91&lng=0":                        http.StatusUnprocessableEntity,
		"/api/v1/lugares/semantica?q=a":                              http.StatusUnprocessableEntity,
		"/api/v1/lugares/semantica?q=cafe&limite=31":                 http.StatusUnprocessableEntity,
	} {
		assert.Equal(t, want, send(t, app, http.MethodGet, path, nil, "").status, path)
	}

	conteo := send(t, app, http.MethodGet, "/api/v1/lugares/conteo", nil, "")
	require.Equal(t, http.StatusOK, conteo.status)
	assert.ElementsMatch(t, []string{"total", "exacto"}, keys(jsonObj(t, conteo.body)))
	barras := send(t, app, http.MethodGet, afluencia+"?dia=2", nil, "")
	require.Equal(t, http.StatusOK, barras.status)
	assert.Subset(t, []string{"dia", "horas", "hora_actual", "suficiente"}, keys(jsonObj(t, barras.body)))
	assert.NotContains(t, barras.body, "usuario")
}

// Las coordenadas de quien pregunta son su ubicación: no van a los logs.
func TestRutasFaltantesSecurity_CoordenadasFueraDeLosLogs(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	const lat, lng = "19.4326171", "-99.1332117"
	for _, path := range []string{
		"/api/v1/lugares/conteo?lat=" + lat + "&lng=" + lng,
		"/api/v1/contexto/clima?lat=" + lat + "&lng=" + lng,
	} {
		require.Equal(t, http.StatusOK, send(t, app, http.MethodGet, path, nil, "").status, path)
	}
	logs := env.Logs.String()
	require.NotEmpty(t, logs)
	assert.NotContains(t, logs, lat)
	assert.NotContains(t, logs, lng)
}

// La semántica es de sesión opcional y cuenta por persona o, sin token, por IP: agotar el
// cupo anónimo de una IP no bloquea a quien entra con sesión.
func TestRutasFaltantesSecurity_SemanticaCuentaPorSujeto(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	ctx := context.Background()
	require.NoError(t, env.Redis.Set(ctx, "rate:semantica:0.0.0.0", 30, time.Minute).Err())
	path := "/api/v1/lugares/semantica?" + url.Values{"q": {"cafe tranquilo"}}.Encode()

	anon := send(t, app, http.MethodGet, path, nil, "")
	assert.Equal(t, http.StatusTooManyRequests, anon.status)
	assert.NotEmpty(t, anon.headers.Get("Retry-After"))
	u := env.CreateUser(t, "semantica-faltantes@wherever.test")
	conSesion := send(t, app, http.MethodGet, path, nil, testutil.ValidTokenFor(t, u.ID.Hex(), "jti-semantica-faltantes", middleware.RoleConsumer))
	assert.Equal(t, http.StatusOK, conSesion.status, conSesion.body)
	assert.NotContains(t, conSesion.body, u.ID.Hex())
	assert.NotContains(t, conSesion.body, u.Email)
}

// cursoresAlterados arma cursores con un cambio que sí altera bytes. Cambiar el último
// carácter de la firma no sirve: en Base64 sin relleno sus 2 bits bajos no cuentan y la firma
// decodificada puede quedar igual.
func cursoresAlterados(t *testing.T, cursor string) map[string]string {
	t.Helper()
	payload, sig, ok := strings.Cut(cursor, ".")
	require.True(t, ok, cursor)
	otro := func(c byte) string {
		if c == 'A' {
			return "B"
		}
		return "A"
	}
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	require.NoError(t, err)
	var c map[string]any
	require.NoError(t, json.Unmarshal(raw, &c))
	c["orden"] = "0:0:" + strconv.FormatInt(time.Now().Unix(), 10)
	forged, err := json.Marshal(c)
	require.NoError(t, err)
	return map[string]string{
		"firma alterada":     payload + "." + otro(sig[0]) + sig[1:],
		"contenido alterado": base64.RawURLEncoding.EncodeToString(forged) + "." + sig,
	}
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func lugarVisible(t *testing.T, env *authtest.Env, slug string) negocios.Business {
	t.Helper()
	place := negocios.Business{ID: bson.NewObjectID(), Slug: slug, QRCheckin: "qr-" + slug, Name: "Lugar " + slug, Category: "restaurante",
		State: negocios.StatusApproved, Location: negocios.Location{Type: "Point", Coordinates: []float64{-99.13, 19.43}}, CreatedAt: time.Now()}
	_, err := env.Mongo.DB().Collection(negocios.Collection).InsertOne(context.Background(), place)
	require.NoError(t, err)
	return place
}

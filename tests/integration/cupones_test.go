package integration

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/cupones"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/reservastest"
)

func cuponesDe(b negocios.Business) string { return "/api/v1/negocios/" + b.ID.Hex() + "/cupones" }

func cuerpoCupon(titulo, tipo string, valor any) map[string]any {
	now := time.Now().UTC()
	body := map[string]any{
		"titulo": titulo, "tipo": tipo,
		"vigencia_inicio": now.Add(-time.Hour).Format(time.RFC3339), "vigencia_fin": now.Add(48 * time.Hour).Format(time.RFC3339),
	}
	if valor != nil {
		body["valor"] = valor
	}
	return body
}

// crearCupon crea el cupón por HTTP y devuelve su id y versión.
func crearCupon(t *testing.T, app *fiber.App, b negocios.Business, token string, body map[string]any) (string, int) {
	t.Helper()
	resp := sendJSON(t, app, http.MethodPost, cuponesDe(b), body, token)
	requireStatus(t, resp, http.StatusCreated)
	out := decode(t, resp)
	v, ok := out["version"].(float64)
	require.True(t, ok)
	return str(t, out["id"]), int(v)
}

// guardarYCanjear guarda el cupón en la cartera y pide el código.
func guardarYCanjear(t *testing.T, app *fiber.App, cupon, token string) (cartera, codigo string) {
	t.Helper()
	resp := sendJSON(t, app, http.MethodPost, "/api/v1/cupones/"+cupon+"/guardar", nil, token)
	requireStatus(t, resp, http.StatusCreated)
	cartera = str(t, decode(t, resp)["id"])
	resp = sendJSON(t, app, http.MethodPost, "/api/v1/mis-cupones/"+cartera+"/canjear", nil, token)
	requireStatus(t, resp, http.StatusOK)
	out := decode(t, resp)
	codigo = str(t, out["codigo"])
	assert.Equal(t, "wherever://canje/"+codigo, out["qr"])
	return cartera, codigo
}

func validarHTTP(t *testing.T, app *fiber.App, b negocios.Business, token, codigo string) (int, map[string]any) {
	t.Helper()
	resp := sendJSON(t, app, http.MethodPost, cuponesDe(b)+"/validar", map[string]any{"codigo": codigo}, token)
	return resp.StatusCode, decode(t, resp)
}

func cuponDB(t *testing.T, env *authtest.Env, id string) cupones.Cupon {
	t.Helper()
	oid, err := bson.ObjectIDFromHex(id)
	require.NoError(t, err)
	c, err := cupones.NewRepository(env.Mongo.DB()).FindByID(context.Background(), oid)
	require.NoError(t, err)
	return c
}

func cuponesActivos(t *testing.T, env *authtest.Env, b negocios.Business) int {
	t.Helper()
	got, err := negocios.NewRepository(env.Mongo.DB()).FindByID(context.Background(), b.ID)
	require.NoError(t, err)
	return got.Coupons
}

func setCupon(t *testing.T, env *authtest.Env, col, id string, set bson.D) {
	t.Helper()
	oid, err := bson.ObjectIDFromHex(id)
	require.NoError(t, err)
	_, err = env.Mongo.DB().Collection(col).UpdateOne(context.Background(), bson.D{{Key: "_id", Value: oid}}, bson.D{{Key: "$set", Value: set}})
	require.NoError(t, err)
}

func TestCupones_CrearEditarBorrar(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	b, token := dueno(t, env, "duena-cup@wherever.test", reservastest.Config{})

	id, version := crearCupon(t, app, b, token, cuerpoCupon("15 % en comida", cupones.TipoPorcentaje, 15))
	assert.Equal(t, 1, cuponesActivos(t, env, b))

	// Relámpago: el servidor fija 3 horas e ignora las fechas.
	resp := sendJSON(t, app, http.MethodPost, cuponesDe(b), map[string]any{"titulo": "Ya", "tipo": "regalo", "relampago": true, "vigencia_fin": "2020-01-01T00:00:00Z"}, token)
	requireStatus(t, resp, http.StatusCreated)
	r := decode(t, resp)
	inicio, err := time.Parse(time.RFC3339, str(t, r["vigencia_inicio"]))
	require.NoError(t, err)
	fin, err := time.Parse(time.RFC3339, str(t, r["vigencia_fin"]))
	require.NoError(t, err)
	assert.Equal(t, 3*time.Hour, fin.Sub(inicio))
	assert.Nil(t, r["valor"])

	// Valor que no corresponde al tipo = 422.
	requireStatus(t, sendJSON(t, app, http.MethodPost, cuponesDe(b), cuerpoCupon("Sin monto", "monto", nil), token), http.StatusUnprocessableEntity)

	// Programado: empieza mañana.
	prog := cuerpoCupon("Mañana", "dos_por_uno", nil)
	prog["vigencia_inicio"] = time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	resp = sendJSON(t, app, http.MethodPost, cuponesDe(b), prog, token)
	requireStatus(t, resp, http.StatusCreated)
	assert.Equal(t, cupones.EstadoProgramado, decode(t, resp)["estado"])

	resp = sendJSON(t, app, http.MethodGet, cuponesDe(b), nil, token)
	requireStatus(t, resp, http.StatusOK)
	assert.Len(t, list(t, decode(t, resp)["items"]), 3)

	path := cuponesDe(b) + "/" + id
	requireStatus(t, sendJSON(t, app, http.MethodPatch, path, map[string]any{"titulo": "20 %"}, token), http.StatusBadRequest)
	stale := map[string]string{"If-Match": `"` + strconv.Itoa(version+5) + `"`}
	requireStatus(t, sendWithHeaders(t, app, http.MethodPatch, path, map[string]any{"titulo": "20 %"}, token, stale), http.StatusConflict)
	ok := map[string]string{"If-Match": `"` + strconv.Itoa(version) + `"`}
	resp = sendWithHeaders(t, app, http.MethodPatch, path, map[string]any{"titulo": "20 % en comida", "valor": 20, "estado": "pausado"}, token, ok)
	requireStatus(t, resp, http.StatusOK)
	r = decode(t, resp)
	assert.Equal(t, "pausado", r["estado"])
	assert.Equal(t, float64(20), r["valor"])
	assert.Equal(t, 1, cuponesActivos(t, env, b), "el relámpago sigue; el pausado ya no cuenta")

	requireStatus(t, sendJSON(t, app, http.MethodDelete, path, nil, token), http.StatusBadRequest)
	resp = sendWithHeaders(t, app, http.MethodDelete, path, nil, token, map[string]string{"If-Match": `"` + strconv.Itoa(version+1) + `"`})
	requireStatus(t, resp, http.StatusNoContent)
	assert.Empty(t, readAll(t, resp))
}

// Solo el comerciante dueño crea, lista, edita, borra, valida y ve estadísticas: el negocio
// ajeno responde 404 igual que si no existiera.
func TestCupones_PropiedadDelNegocio(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	b, token := dueno(t, env, "duena-prop-cup@wherever.test", reservastest.Config{})
	id, version := crearCupon(t, app, b, token, cuerpoCupon("2x1", "dos_por_uno", nil))
	_, ajeno := dueno(t, env, "ajeno-prop-cup@wherever.test", reservastest.Config{})

	ifm := map[string]string{"If-Match": `"` + strconv.Itoa(version) + `"`}
	for _, c := range []struct {
		method, path string
		body         any
	}{
		{http.MethodPost, cuponesDe(b), cuerpoCupon("Mío", "regalo", nil)},
		{http.MethodGet, cuponesDe(b), nil},
		{http.MethodGet, cuponesDe(b) + "/" + id + "/estadisticas", nil},
		{http.MethodPatch, cuponesDe(b) + "/" + id, map[string]any{"titulo": "Robado"}},
		{http.MethodDelete, cuponesDe(b) + "/" + id, nil},
		{http.MethodPost, cuponesDe(b) + "/validar", map[string]any{"codigo": "ABC234"}},
		{http.MethodPost, cuponesDe(b) + "/sugerir-titulo", map[string]any{"tipo": "regalo"}},
	} {
		resp := sendWithHeaders(t, app, c.method, c.path, c.body, ajeno, ifm)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode, c.method+" "+c.path)
	}
	c := cuponDB(t, env, id)
	assert.Equal(t, "2x1", c.Titulo)
	assert.Equal(t, version, c.Version)
}

func TestCupones_CercaYDetalle(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	b, token := dueno(t, env, "duena-cerca-cup@wherever.test", reservastest.Config{})
	pausado, tokenP := dueno(t, env, "pausado-cerca-cup@wherever.test", reservastest.Config{})
	visible, _ := crearCupon(t, app, b, token, cuerpoCupon("Visible", "regalo", nil))
	oculto, _ := crearCupon(t, app, pausado, tokenP, cuerpoCupon("Oculto", "regalo", nil))
	vencido, _ := crearCupon(t, app, b, token, cuerpoCupon("Vencido", "regalo", nil))
	// El negocio deja de estar publicado sin aviso; el cupón vencido no espera al trabajo.
	_, err := env.Mongo.DB().Collection(negocios.Collection).UpdateOne(context.Background(), bson.D{{Key: "_id", Value: pausado.ID}}, bson.D{{Key: "$set", Value: bson.D{{Key: "estado", Value: negocios.StatusPaused}}}})
	require.NoError(t, err)
	setCupon(t, env, cupones.Collection, vencido, bson.D{{Key: "vigencia_fin", Value: time.Now().Add(-time.Minute)}})

	resp := get(t, app, "/api/v1/cupones/cerca?lat=19.4326&lng=-99.1332&radio_km=2")
	requireStatus(t, resp, http.StatusOK)
	items := list(t, decode(t, resp)["items"])
	require.Len(t, items, 1)
	assert.Equal(t, visible, obj(t, items[0])["id"])
	assert.NotContains(t, obj(t, items[0]), "vistas", "sin contadores en lo público")

	resp = get(t, app, "/api/v1/cupones/cerca?lat=19.4326&lng=-99.1332&categoria=bar")
	requireStatus(t, resp, http.StatusOK)
	assert.Empty(t, list(t, decode(t, resp)["items"]))
	resp = get(t, app, "/api/v1/cupones/cerca?lat=20.0500&lng=-99.3400&radio_km=5")
	requireStatus(t, resp, http.StatusOK)
	assert.Empty(t, list(t, decode(t, resp)["items"]), "fuera del radio")
	requireStatus(t, get(t, app, "/api/v1/cupones/cerca?lat=19.4&lng=-99.1&radio_km=50"), http.StatusUnprocessableEntity)

	requireStatus(t, get(t, app, "/api/v1/cupones/"+visible), http.StatusOK)
	requireStatus(t, get(t, app, "/api/v1/cupones/"+oculto), http.StatusNotFound)
	requireStatus(t, get(t, app, "/api/v1/cupones/"+vencido), http.StatusNotFound)
	assert.Equal(t, 1, cuponDB(t, env, visible).Vistas, "solo el detalle cuenta vistas")
}

func TestCupones_CarteraCanjeYEstadisticas(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	b, token := dueno(t, env, "duena-cartera-cup@wherever.test", reservastest.Config{})
	id, version := crearCupon(t, app, b, token, cuerpoCupon("Café de olla gratis", "regalo", nil))
	_, persona := cliente(t, env, "cliente-cartera-cup@wherever.test")

	requireStatus(t, get(t, app, "/api/v1/cupones/"+id), http.StatusOK)
	cartera, codigo := guardarYCanjear(t, app, id, persona)
	requireStatus(t, sendJSON(t, app, http.MethodPost, "/api/v1/cupones/"+id+"/guardar", nil, persona), http.StatusConflict)

	// Pedir el código otra vez mientras vive devuelve el mismo.
	resp := sendJSON(t, app, http.MethodPost, "/api/v1/mis-cupones/"+cartera+"/canjear", nil, persona)
	requireStatus(t, resp, http.StatusOK)
	assert.Equal(t, codigo, decode(t, resp)["codigo"])

	resp = sendJSON(t, app, http.MethodGet, "/api/v1/mis-cupones?q=caf", nil, persona)
	requireStatus(t, resp, http.StatusOK)
	assert.Len(t, list(t, decode(t, resp)["items"]), 1)
	resp = sendJSON(t, app, http.MethodGet, "/api/v1/mis-cupones?q=pizza", nil, persona)
	requireStatus(t, resp, http.StatusOK)
	assert.Empty(t, list(t, decode(t, resp)["items"]))

	status, out := validarHTTP(t, app, b, token, codigo)
	require.Equal(t, http.StatusOK, status, out)
	assert.Equal(t, true, out["valido"])
	assert.Equal(t, "Café de olla gratis", obj(t, out["cupon"])["titulo"])
	// El reintento del mismo negocio recibe el mismo verde sin contar otra vez.
	status, out = validarHTTP(t, app, b, token, codigo)
	require.Equal(t, http.StatusOK, status, out)
	assert.Equal(t, true, out["repetido"])
	assert.Equal(t, 1, cuponDB(t, env, id).UsosActuales)

	resp = sendJSON(t, app, http.MethodGet, "/api/v1/mis-cupones?filtro=usados", nil, persona)
	requireStatus(t, resp, http.StatusOK)
	usados := list(t, decode(t, resp)["items"])
	require.Len(t, usados, 1)
	assert.Equal(t, "canjeado", obj(t, usados[0])["estado"])
	requireStatus(t, sendJSON(t, app, http.MethodPost, "/api/v1/mis-cupones/"+cartera+"/canjear", nil, persona), http.StatusConflict)

	resp = sendJSON(t, app, http.MethodGet, cuponesDe(b)+"/"+id+"/estadisticas", nil, token)
	requireStatus(t, resp, http.StatusOK)
	totales := obj(t, decode(t, resp)["totales"])
	assert.Equal(t, float64(1), totales["vistas"])
	assert.Equal(t, float64(1), totales["guardados"])
	assert.Equal(t, float64(1), totales["canjes"])

	// Con canjes: el título no cambia (409) y no se borra (409); el límite sí.
	ifm := map[string]string{"If-Match": `"` + strconv.Itoa(version) + `"`}
	requireStatus(t, sendWithHeaders(t, app, http.MethodPatch, cuponesDe(b)+"/"+id, map[string]any{"titulo": "Otro"}, token, ifm), http.StatusConflict)
	requireStatus(t, sendWithHeaders(t, app, http.MethodDelete, cuponesDe(b)+"/"+id, nil, token, ifm), http.StatusConflict)
	resp = sendWithHeaders(t, app, http.MethodPatch, cuponesDe(b)+"/"+id, map[string]any{"limite_usos": 1}, token, ifm)
	requireStatus(t, resp, http.StatusOK)
	assert.Equal(t, cupones.EstadoAgotado, decode(t, resp)["estado"])

	resumen, err := env.Cupones.ResumenCupones(context.Background(), b.ID, negocios.Periodo{Desde: time.Now().Add(-time.Hour), Hasta: time.Now().Add(time.Hour)})
	require.NoError(t, err)
	assert.Equal(t, 1, resumen.CanjeadosHoy)
	assert.Zero(t, resumen.Activos)

	requireStatus(t, sendJSON(t, app, http.MethodDelete, "/api/v1/mis-cupones/"+cartera, nil, persona), http.StatusNoContent)
	requireStatus(t, sendJSON(t, app, http.MethodDelete, "/api/v1/mis-cupones/"+cartera, nil, persona), http.StatusNotFound)
}

func TestCupones_ValidarRojos(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	b, token := dueno(t, env, "duena-rojo-cup@wherever.test", reservastest.Config{})
	otro, tokenOtro := dueno(t, env, "otro-rojo-cup@wherever.test", reservastest.Config{})
	_, persona := cliente(t, env, "cliente-rojo-cup@wherever.test")

	motivo := func(neg negocios.Business, tok, codigo, want string) {
		t.Helper()
		status, out := validarHTTP(t, app, neg, tok, codigo)
		require.Equal(t, http.StatusConflict, status, out)
		assert.Equal(t, false, out["valido"])
		assert.Equal(t, want, out["motivo"])
	}
	motivo(b, token, "ZZZZZZ", cupones.MotivoNoExiste)
	motivo(b, token, "0OOO11", cupones.MotivoNoExiste)

	id, _ := crearCupon(t, app, b, token, cuerpoCupon("Rojo", "regalo", nil))
	cartera, codigo := guardarYCanjear(t, app, id, persona)
	motivo(otro, tokenOtro, codigo, cupones.MotivoOtroNegocio)

	// Código expirado (5 min).
	setCupon(t, env, cupones.CarteraCollection, cartera, bson.D{{Key: "canje_expira_en", Value: time.Now().Add(-time.Second)}})
	motivo(b, token, codigo, cupones.MotivoVencido)

	// Fuera de horario: solo vale mañana.
	resp := sendJSON(t, app, http.MethodPost, "/api/v1/mis-cupones/"+cartera+"/canjear", nil, persona)
	requireStatus(t, resp, http.StatusOK)
	codigo = str(t, decode(t, resp)["codigo"])
	manana := (int(time.Now().In(negocios.MexicoCity()).Weekday()) + 1) % 7
	setCupon(t, env, cupones.Collection, id, bson.D{{Key: "dias_validos", Value: []int{manana}}})
	motivo(b, token, codigo, cupones.MotivoFuera)
	requireStatus(t, sendJSON(t, app, http.MethodPost, "/api/v1/mis-cupones/"+cartera+"/canjear", nil, persona), http.StatusOK)

	// Ya usado: canjeado hace más de 5 minutos ya no es reintento.
	setCupon(t, env, cupones.Collection, id, bson.D{{Key: "dias_validos", Value: []int{}}})
	status, out := validarHTTP(t, app, b, token, codigo)
	require.Equal(t, http.StatusOK, status, out)
	setCupon(t, env, cupones.CarteraCollection, cartera, bson.D{{Key: "canjeado_en", Value: time.Now().Add(-6 * time.Minute)}})
	motivo(b, token, codigo, cupones.MotivoYaUsado)
}

// 3 rondas de 10 escaneos simultáneos del mismo código: un solo canje por ronda y
// `usos_actuales` sube de uno en uno; los demás son el reintento (`repetido`).
func TestCupones_Concurrencia_CanjeUnaVez(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	b, token := dueno(t, env, "duena-conc-cup@wherever.test", reservastest.Config{})
	duenaClaims := authtest.ClaimsDe(b.OwnerID.Hex(), middleware.RoleConsumer, middleware.RoleMerchant)
	id, _ := crearCupon(t, app, b, token, cuerpoCupon("Concurrencia", "regalo", nil))
	for ronda := range 3 {
		_, persona := cliente(t, env, fmt.Sprintf("conc-%d-cup@wherever.test", ronda))
		_, codigo := guardarYCanjear(t, app, id, persona)
		var wg sync.WaitGroup
		var mu sync.Mutex
		nuevos, repetidos := 0, 0
		for range 10 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				out, err := env.Cupones.Validar(context.Background(), duenaClaims, b.ID.Hex(), cupones.ValidarRequest{Codigo: codigo})
				mu.Lock()
				defer mu.Unlock()
				switch {
				case err != nil:
					t.Errorf("ronda %d: %v", ronda, err)
				case out.Valido && !out.Repetido:
					nuevos++
				case out.Valido && out.Repetido:
					repetidos++
				default:
					t.Errorf("ronda %d: rojo %s", ronda, out.Motivo)
				}
			}()
		}
		wg.Wait()
		assert.Equal(t, 1, nuevos, "ronda %d", ronda)
		assert.Equal(t, 9, repetidos, "ronda %d", ronda)
		assert.Equal(t, ronda+1, cuponDB(t, env, id).UsosActuales, "ronda %d", ronda)
	}
}

// `limite_usos = 1` y dos códigos validados a la vez: uno verde y otro `agotado`; el perdedor
// vuelve a `en_canje` y el cupón queda con un uso.
func TestCupones_Concurrencia_LimiteUsos(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	b, token := dueno(t, env, "duena-lim-cup@wherever.test", reservastest.Config{})
	duenaClaims := authtest.ClaimsDe(b.OwnerID.Hex(), middleware.RoleConsumer, middleware.RoleMerchant)
	body := cuerpoCupon("Solo uno", "regalo", nil)
	body["limite_usos"] = 1
	id, _ := crearCupon(t, app, b, token, body)
	codigos := make([]string, 2)
	for i := range codigos {
		_, persona := cliente(t, env, fmt.Sprintf("lim-%d-cup@wherever.test", i))
		_, codigos[i] = guardarYCanjear(t, app, id, persona)
	}
	resultados := make([]cupones.ValidarResponse, 2)
	var wg sync.WaitGroup
	for i, codigo := range codigos {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, err := env.Cupones.Validar(context.Background(), duenaClaims, b.ID.Hex(), cupones.ValidarRequest{Codigo: codigo})
			assert.NoError(t, err)
			resultados[i] = out
		}()
	}
	wg.Wait()
	verdes, agotados := 0, 0
	for _, r := range resultados {
		if r.Valido {
			verdes++
		} else if r.Motivo == cupones.MotivoAgotado {
			agotados++
		}
	}
	assert.Equal(t, 1, verdes)
	assert.Equal(t, 1, agotados)
	c := cuponDB(t, env, id)
	assert.Equal(t, 1, c.UsosActuales)
	assert.Equal(t, cupones.EstadoAgotado, c.Estado)
	n, err := env.Mongo.DB().Collection(cupones.CarteraCollection).CountDocuments(context.Background(), bson.D{{Key: "estado", Value: cupones.CarteraCanjeado}})
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
}

func TestCupones_TrabajoYObservador(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	b, token := dueno(t, env, "duena-trabajo-cup@wherever.test", reservastest.Config{})
	_, persona := cliente(t, env, "cliente-trabajo-cup@wherever.test")
	prog := cuerpoCupon("Programado", "regalo", nil)
	prog["vigencia_inicio"] = time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	programado, _ := crearCupon(t, app, b, token, prog)
	vence, _ := crearCupon(t, app, b, token, cuerpoCupon("Vence", "regalo", nil))
	resp := sendJSON(t, app, http.MethodPost, "/api/v1/cupones/"+vence+"/guardar", nil, persona)
	requireStatus(t, resp, http.StatusCreated)
	cartera := str(t, decode(t, resp)["id"])

	// El inicio del programado llega y el otro termina.
	setCupon(t, env, cupones.Collection, programado, bson.D{{Key: "vigencia_inicio", Value: time.Now().Add(-time.Minute)}})
	setCupon(t, env, cupones.Collection, vence, bson.D{{Key: "vigencia_fin", Value: time.Now().Add(-time.Minute)}})
	setCupon(t, env, cupones.CarteraCollection, cartera, bson.D{{Key: "vence_en", Value: time.Now().Add(-time.Minute)}})
	require.NoError(t, env.Cupones.Mantener(context.Background()))
	require.NoError(t, env.Cupones.Mantener(context.Background()), "idempotente")
	assert.Equal(t, cupones.EstadoActivo, cuponDB(t, env, programado).Estado)
	assert.Equal(t, cupones.EstadoVencido, cuponDB(t, env, vence).Estado)
	assert.Equal(t, 1, cuponesActivos(t, env, b))
	resp = sendJSON(t, app, http.MethodGet, "/api/v1/mis-cupones", nil, persona)
	requireStatus(t, resp, http.StatusOK)
	assert.Empty(t, list(t, decode(t, resp)["items"]), "la cartera vencida sale de vigentes")

	obs := cupones.NewObservador(env.Cupones)
	require.NoError(t, obs.Pausado(context.Background(), b.ID))
	assert.Equal(t, cupones.EstadoPausado, cuponDB(t, env, programado).Estado)
	assert.Zero(t, cuponesActivos(t, env, b))
	require.NoError(t, obs.Reanudado(context.Background(), b.ID))
	assert.Equal(t, cupones.EstadoActivo, cuponDB(t, env, programado).Estado)
	assert.Equal(t, cupones.EstadoVencido, cuponDB(t, env, vence).Estado, "el vencido no revive")
	require.NoError(t, obs.DadoDeBaja(context.Background(), b.ID))
	require.NoError(t, obs.Reanudado(context.Background(), b.ID))
	assert.Equal(t, cupones.EstadoPausado, cuponDB(t, env, programado).Estado, "tras la baja no vuelve")
}

func TestCupones_ARCO(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	b, token := dueno(t, env, "duena-arco-cup@wherever.test", reservastest.Config{})
	u, persona := cliente(t, env, "cliente-arco-cup@wherever.test")
	usado, _ := crearCupon(t, app, b, token, cuerpoCupon("Usado", "regalo", nil))
	vigente, _ := crearCupon(t, app, b, token, cuerpoCupon("Vigente", "monto", 5000))
	_, codigo := guardarYCanjear(t, app, usado, persona)
	status, _ := validarHTTP(t, app, b, token, codigo)
	require.Equal(t, http.StatusOK, status)
	requireStatus(t, sendJSON(t, app, http.MethodPost, "/api/v1/cupones/"+vigente+"/guardar", nil, persona), http.StatusCreated)

	p := cupones.NewPrivacy(cupones.NewRepository(env.Mongo.DB()))
	assert.Equal(t, "cupones", p.Name())
	exported, err := p.Export(context.Background(), u.ID)
	require.NoError(t, err)
	raw := fmt.Sprintf("%+v", exported)
	assert.NotContains(t, raw, codigo, "nunca el código de canje")
	assert.Contains(t, raw, "Vigente")

	col := env.Mongo.DB().Collection(cupones.CarteraCollection)
	require.NoError(t, p.PurgeHistory(context.Background(), u.ID))
	n, err := col.CountDocuments(context.Background(), bson.D{{Key: "usuario_id", Value: u.ID}})
	require.NoError(t, err)
	assert.Equal(t, int64(1), n, "borrar historial deja los vigentes")

	// La cuenta de la duenaClaims se borra: sus validaciones quedan sin `validado_por`.
	_, otro := cliente(t, env, "otro-arco-cup@wherever.test")
	_, codigo = guardarYCanjear(t, app, usado, otro)
	status, _ = validarHTTP(t, app, b, token, codigo)
	require.Equal(t, http.StatusOK, status)
	require.NoError(t, p.PurgeAccount(context.Background(), b.OwnerID))
	n, err = col.CountDocuments(context.Background(), bson.D{{Key: "validado_por", Value: b.OwnerID}})
	require.NoError(t, err)
	assert.Zero(t, n)

	require.NoError(t, p.PurgeAccount(context.Background(), u.ID))
	n, err = col.CountDocuments(context.Background(), bson.D{{Key: "usuario_id", Value: u.ID}})
	require.NoError(t, err)
	assert.Zero(t, n)
}

func TestCupones_SugerirTitulo(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	b, token := dueno(t, env, "duena-ia-cup@wherever.test", reservastest.Config{})
	env.IA.Responder(`{"titulo":"2x1 en cafés de olla"}`)
	resp := sendJSON(t, app, http.MethodPost, cuponesDe(b)+"/sugerir-titulo", map[string]any{"tipo": "dos_por_uno", "descripcion": "Cafetería de barrio"}, token)
	requireStatus(t, resp, http.StatusOK)
	assert.Equal(t, "2x1 en cafés de olla", decode(t, resp)["titulo"])
	requireStatus(t, sendJSON(t, app, http.MethodPost, cuponesDe(b)+"/sugerir-titulo", map[string]any{"tipo": "porcentaje"}, token), http.StatusUnprocessableEntity)

	sinIA := cupones.NewService(cupones.NewRepository(env.Mongo.DB()), negocios.NewRepository(env.Mongo.DB()), nil, nil)
	_, err := sinIA.Sugerir(context.Background(), authtest.ClaimsDe(b.OwnerID.Hex(), middleware.RoleMerchant), b.ID.Hex(), cupones.SugerirRequest{Tipo: "regalo"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ia_no_disponible")
}

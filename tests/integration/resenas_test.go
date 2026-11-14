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

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/moderacion"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/resenas"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/reservastest"
)

// El negocio de reservastest está en (19.4326, -99.1332).
const latNegocio, lngNegocio = 19.4326, -99.1332

func cuerpoCheckin(b negocios.Business, lat, lng float64, precision int) map[string]any {
	return map[string]any{"negocio_id": b.ID.Hex(), "metodo": "ubicacion", "lat": lat, "lng": lng, "precision_m": precision}
}

// llegar hace un check-in válido por ubicación y devuelve su id.
func llegar(t *testing.T, app *fiber.App, b negocios.Business, token string) string {
	t.Helper()
	resp := sendJSON(t, app, http.MethodPost, "/api/v1/checkins", cuerpoCheckin(b, latNegocio+0.0003, lngNegocio, 15), token)
	requireStatus(t, resp, http.StatusCreated)
	return str(t, decode(t, resp)["id"])
}

func cuerpoResena(checkin string, estrellas int) map[string]any {
	return map[string]any{
		"checkin_id": checkin, "calificacion": estrellas, "chips": []string{"buena_comida"}, "comentario": "Muy rica la comida",
		"aspectos": map[string]any{"sabor": estrellas, "atencion": 4, "ambiente": 3, "espera": 2, "precio_calidad": 5},
	}
}

// resenar crea la reseña y devuelve id y versión.
func resenar(t *testing.T, app *fiber.App, token string, body map[string]any) (string, int) {
	t.Helper()
	resp := sendJSON(t, app, http.MethodPost, "/api/v1/resenas", body, token)
	requireStatus(t, resp, http.StatusCreated)
	out := decode(t, resp)
	v, ok := out["version"].(float64)
	require.True(t, ok)
	return str(t, out["id"]), int(v)
}

// visita crea una persona con un check-in válido en el negocio.
func visita(t *testing.T, env *authtest.Env, app *fiber.App, b negocios.Business, email string) (bson.ObjectID, string, string) {
	t.Helper()
	u, token := cliente(t, env, email)
	return u.ID, token, llegar(t, app, b, token)
}

func calificacion(t *testing.T, env *authtest.Env, id bson.ObjectID) negocios.Calificacion {
	t.Helper()
	b, err := negocios.NewRepository(env.Mongo.DB()).FindByID(context.Background(), id)
	require.NoError(t, err)
	return b.Rating
}

func resenaDB(t *testing.T, env *authtest.Env, id string) resenas.Resena {
	t.Helper()
	oid, err := bson.ObjectIDFromHex(id)
	require.NoError(t, err)
	r, err := resenas.NewRepository(env.Mongo.DB()).Buscar(context.Background(), oid)
	require.NoError(t, err)
	return r
}

func setDoc(t *testing.T, env *authtest.Env, col string, id any, set bson.D) {
	t.Helper()
	_, err := env.Mongo.DB().Collection(col).UpdateOne(context.Background(), bson.D{{Key: "_id", Value: id}}, bson.D{{Key: "$set", Value: set}})
	require.NoError(t, err)
}

func oidDe(t *testing.T, raw string) bson.ObjectID {
	t.Helper()
	id, err := bson.ObjectIDFromHex(raw)
	require.NoError(t, err)
	return id
}

// decidirCola aprueba o rechaza en la cola de nv-43 lo pendiente de esa reseña.
func decidirCola(t *testing.T, env *authtest.Env, resena, decision string) {
	t.Helper()
	var it moderacion.Item
	require.NoError(t, env.Mongo.DB().Collection(moderacion.ColeccionCola).FindOne(context.Background(),
		bson.D{{Key: "referencia", Value: resena}, {Key: "estado", Value: moderacion.EstadoPendiente}}).Decode(&it))
	admin := env.CreateAdmin(t, "admin-"+resena+"@wherever.test")
	_, err := env.Moderacion.Decidir(context.Background(), admin.ID.Hex(), it.ID.Hex(), it.Version, moderacion.DecisionRequest{Decision: decision, Motivo: "revisado"})
	require.NoError(t, err)
}

func TestCheckins_UbicacionQRyDuplicado(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	b, _ := dueno(t, env, "duena-chk@wherever.test", reservastest.Config{})
	otro, _ := dueno(t, env, "duena-chk2@wherever.test", reservastest.Config{})
	tercero, _ := dueno(t, env, "duena-chk3@wherever.test", reservastest.Config{})
	u, token := cliente(t, env, "cli-chk@wherever.test")

	// Fuera de radio (≈ 220 m) y precisión no creíble: 422 neutro, se guarda sin coordenadas.
	resp := sendJSON(t, app, http.MethodPost, "/api/v1/checkins", cuerpoCheckin(b, latNegocio+0.002, lngNegocio, 10), token)
	requireStatus(t, resp, http.StatusUnprocessableEntity)
	assert.Contains(t, readAll(t, resp), "aún no llegas")
	requireStatus(t, sendJSON(t, app, http.MethodPost, "/api/v1/checkins", cuerpoCheckin(b, latNegocio, lngNegocio, 500), token), http.StatusUnprocessableEntity)

	// QR de otro negocio = 422 igual; el suyo = 201 sin ubicación.
	qrAjeno := map[string]any{"negocio_id": b.ID.Hex(), "metodo": "qr_negocio", "codigo": otro.QRCheckin}
	requireStatus(t, sendJSON(t, app, http.MethodPost, "/api/v1/checkins", qrAjeno, token), http.StatusUnprocessableEntity)
	qr := map[string]any{"negocio_id": tercero.ID.Hex(), "metodo": "qr_negocio", "codigo": tercero.QRCheckin}
	requireStatus(t, sendJSON(t, app, http.MethodPost, "/api/v1/checkins", qr, token), http.StatusCreated)

	// Dentro del radio = 201; el segundo del día = 409 sin otro documento.
	resp = sendJSON(t, app, http.MethodPost, "/api/v1/checkins", cuerpoCheckin(b, latNegocio+0.0005, lngNegocio, 20), token)
	requireStatus(t, resp, http.StatusCreated)
	body := decode(t, resp)
	assertSnakeCase(t, body)
	assert.Equal(t, true, body["valido"])
	assert.InDelta(t, 56, body["distancia_m"], 2)
	requireStatus(t, sendJSON(t, app, http.MethodPost, "/api/v1/checkins", cuerpoCheckin(b, latNegocio, lngNegocio, 20), token), http.StatusConflict)

	// Negocio inexistente = 404.
	requireStatus(t, sendJSON(t, app, http.MethodPost, "/api/v1/checkins", map[string]any{"negocio_id": bson.NewObjectID().Hex(), "metodo": "qr_negocio", "codigo": "x"}, token), http.StatusNotFound)

	// Privacidad: ningún check-in guarda la ubicación de la persona.
	cur, err := env.Mongo.DB().Collection(resenas.CheckinsCollection).Find(context.Background(), bson.D{{Key: "usuario_id", Value: u.ID}})
	require.NoError(t, err)
	var docs []bson.M
	require.NoError(t, cur.All(context.Background(), &docs))
	require.Len(t, docs, 5)
	validos := 0
	for _, d := range docs {
		for _, k := range []string{"lat", "lng", "ubicacion", "ubicacion_usuario", "coordenadas"} {
			assert.NotContains(t, d, k)
		}
		if d["valido"] == true {
			validos++
		}
	}
	assert.Equal(t, 2, validos)

	resp = sendJSON(t, app, http.MethodGet, "/api/v1/checkins?limite=2", nil, token)
	requireStatus(t, resp, http.StatusOK)
	page := decode(t, resp)
	assert.Len(t, list(t, page["items"]), 2)
	resp = sendJSON(t, app, http.MethodGet, "/api/v1/checkins?limite=2&cursor="+str(t, page["siguiente_cursor"]), nil, token)
	requireStatus(t, resp, http.StatusOK)
	assert.Len(t, list(t, decode(t, resp)["items"]), 2)
}

func TestCheckins_SaltoImposible(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	cdmx, _ := dueno(t, env, "duena-salto1@wherever.test", reservastest.Config{})
	qro, _ := dueno(t, env, "duena-salto2@wherever.test", reservastest.Config{})
	setDoc(t, env, negocios.Collection, qro.ID, bson.D{{Key: "ubicacion", Value: negocios.Location{Type: "Point", Coordinates: []float64{-100.3899, 20.5888}}}})
	u, token := cliente(t, env, "cli-salto@wherever.test")
	llegar(t, app, cdmx, token)

	// Querétaro (≈ 180 km) un instante después: ubicación falsa, mismo 422 neutro.
	requireStatus(t, sendJSON(t, app, http.MethodPost, "/api/v1/checkins", cuerpoCheckin(qro, 20.5888, -100.3899, 10), token), http.StatusUnprocessableEntity)
	var c resenas.Checkin
	require.NoError(t, env.Mongo.DB().Collection(resenas.CheckinsCollection).FindOne(context.Background(),
		bson.D{{Key: "usuario_id", Value: u.ID}, {Key: "negocio_id", Value: qro.ID}}).Decode(&c))
	assert.False(t, c.Valido)
	assert.Equal(t, resenas.MotivoUbicacionFalsa, c.MotivoRechazo)
}

func TestCheckins_Concurrencia_UnoPorDia(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	b, _ := dueno(t, env, "duena-chk-conc@wherever.test", reservastest.Config{})
	u, token := cliente(t, env, "cli-chk-conc@wherever.test")
	var wg sync.WaitGroup
	codigos := make([]int, 5)
	for i := range 5 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			codigos[i] = sendJSON(t, app, http.MethodPost, "/api/v1/checkins", cuerpoCheckin(b, latNegocio, lngNegocio, 10), token).StatusCode
		}(i)
	}
	wg.Wait()
	creados := 0
	for _, c := range codigos {
		if c == http.StatusCreated {
			creados++
		} else {
			assert.Equal(t, http.StatusConflict, c)
		}
	}
	assert.Equal(t, 1, creados)
	n, err := env.Mongo.DB().Collection(resenas.CheckinsCollection).CountDocuments(context.Background(), bson.D{{Key: "usuario_id", Value: u.ID}, {Key: "valido", Value: true}})
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
}

func TestResenas_CrearPublicarYCache(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	b, _ := dueno(t, env, "duena-res@wherever.test", reservastest.Config{})
	_, token := cliente(t, env, "cli-res@wherever.test")

	// Sin check-in propio = 403.
	_, ajeno, checkAjeno := visita(t, env, app, b, "cli-res-otro@wherever.test")
	requireStatus(t, sendJSON(t, app, http.MethodPost, "/api/v1/resenas", cuerpoResena(checkAjeno, 5), token), http.StatusForbidden)
	_ = ajeno

	checkin := llegar(t, app, b, token)
	resp := sendJSON(t, app, http.MethodGet, "/api/v1/resenas/pendientes", nil, token)
	requireStatus(t, resp, http.StatusOK)
	pend := list(t, decode(t, resp)["items"])
	require.Len(t, pend, 1)
	assert.Equal(t, checkin, obj(t, pend[0])["checkin_id"])
	require.NotEmpty(t, b.Name)
	assert.Equal(t, b.Name, obj(t, pend[0])["negocio_nombre"])

	// La ficha queda en caché antes de reseñar.
	requireStatus(t, sendJSON(t, app, http.MethodGet, "/api/v1/lugares/"+b.ID.Hex(), nil, ""), http.StatusOK)
	clave := "cache:negocio:" + b.ID.Hex()
	n, err := env.Redis.Exists(context.Background(), clave).Result()
	require.NoError(t, err)
	require.Equal(t, int64(1), n)

	// Chips fuera del catálogo = 422.
	malo := cuerpoResena(checkin, 5)
	malo["chips"] = []string{"inventado"}
	requireStatus(t, sendJSON(t, app, http.MethodPost, "/api/v1/resenas", malo, token), http.StatusUnprocessableEntity)

	id, _ := resenar(t, app, token, cuerpoResena(checkin, 5))
	r := resenaDB(t, env, id)
	assert.Equal(t, resenas.EstadoPublicada, r.Estado)
	assert.True(t, r.Contada)
	cal := calificacion(t, env, b.ID)
	assert.Equal(t, 1, cal.TotalResenas)
	assert.Equal(t, 5.0, cal.Promedio)
	assert.Equal(t, 5.0, cal.Aspectos.Sabor)
	assert.Equal(t, 2.0, cal.Aspectos.Espera)
	n, err = env.Redis.Exists(context.Background(), clave).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(0), n, "cambiar la calificación borra la ficha de la caché")

	// La misma visita no da otra reseña; «Por reseñar» queda vacío.
	requireStatus(t, sendJSON(t, app, http.MethodPost, "/api/v1/resenas", cuerpoResena(checkin, 1), token), http.StatusConflict)
	resp = sendJSON(t, app, http.MethodGet, "/api/v1/resenas/pendientes", nil, token)
	requireStatus(t, resp, http.StatusOK)
	assert.Empty(t, list(t, decode(t, resp)["items"]))

	// Otra visita (otro día) dentro de 30 días = 409.
	setDoc(t, env, resenas.CheckinsCollection, oidDe(t, checkin), bson.D{{Key: "dia", Value: "2000-01-01"}})
	otraVisita := llegar(t, app, b, token)
	requireStatus(t, sendJSON(t, app, http.MethodPost, "/api/v1/resenas", cuerpoResena(otraVisita, 1), token), http.StatusConflict)

	// El dueño (que también es consumidor) no reseña su propio negocio.
	duenoTok := merchantToken(t, b.OwnerID, "jti-duena-res-propia")
	propia := llegar(t, app, b, duenoTok)
	requireStatus(t, sendJSON(t, app, http.MethodPost, "/api/v1/resenas", cuerpoResena(propia, 5), duenoTok), http.StatusForbidden)

	resp = sendJSON(t, app, http.MethodGet, "/api/v1/resenas/mias", nil, token)
	requireStatus(t, resp, http.StatusOK)
	mias := list(t, decode(t, resp)["items"])
	require.Len(t, mias, 1)
	assert.Equal(t, true, obj(t, mias[0])["editable"])
}

func TestResenas_ModeracionCola(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	b, _ := dueno(t, env, "duena-mod@wherever.test", reservastest.Config{})

	// Rechazado: 422 y la visita sigue libre.
	_, token, checkin := visita(t, env, app, b, "cli-mod1@wherever.test")
	env.ModeradorResenas.Fijar(moderacion.VeredictoRechazado)
	requireStatus(t, sendJSON(t, app, http.MethodPost, "/api/v1/resenas", cuerpoResena(checkin, 1), token), http.StatusUnprocessableEntity)

	// Sin comentario no se llama al moderador.
	sinTexto := cuerpoResena(checkin, 4)
	delete(sinTexto, "comentario")
	llamadas := env.ModeradorResenas.Llamadas()
	id, _ := resenar(t, app, token, sinTexto)
	assert.Equal(t, llamadas, env.ModeradorResenas.Llamadas())
	assert.Equal(t, resenas.EstadoPublicada, resenaDB(t, env, id).Estado)

	// Cuarentena: en moderación, no cuenta, entra a la cola; aprobar publica y cuenta.
	env.ModeradorResenas.Fijar(moderacion.VeredictoCuarentena)
	_, token2, checkin2 := visita(t, env, app, b, "cli-mod2@wherever.test")
	id2, _ := resenar(t, app, token2, cuerpoResena(checkin2, 2))
	assert.Equal(t, resenas.EstadoEnModeracion, resenaDB(t, env, id2).Estado)
	assert.Equal(t, 1, calificacion(t, env, b.ID).TotalResenas)
	decidirCola(t, env, id2, moderacion.EstadoAprobado)
	r := resenaDB(t, env, id2)
	assert.Equal(t, resenas.EstadoPublicada, r.Estado)
	assert.True(t, r.Contada)
	cal := calificacion(t, env, b.ID)
	assert.Equal(t, 2, cal.TotalResenas)
	assert.Equal(t, 3.0, cal.Promedio)

	// Rechazar en la cola deja `rechazada`, sin contar.
	_, token3, checkin3 := visita(t, env, app, b, "cli-mod3@wherever.test")
	id3, _ := resenar(t, app, token3, cuerpoResena(checkin3, 1))
	decidirCola(t, env, id3, moderacion.EstadoRechazado)
	assert.Equal(t, resenas.EstadoRechazada, resenaDB(t, env, id3).Estado)
	assert.Equal(t, 2, calificacion(t, env, b.ID).TotalResenas)
	env.ModeradorResenas.Fijar(moderacion.VeredictoAprobado)
}

func TestResenas_SemillaSinSuma(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	b, _ := dueno(t, env, "duena-semilla@wherever.test", reservastest.Config{})
	semilla := negocios.Calificacion{Promedio: 4.0, TotalResenas: 10, Aspectos: negocios.Aspectos{Sabor: 4, Atencion: 4, Ambiente: 4, Espera: 4, PrecioCalidad: 4}}
	setDoc(t, env, negocios.Collection, b.ID, bson.D{{Key: "calificacion", Value: semilla}})
	_, token, checkin := visita(t, env, app, b, "cli-semilla@wherever.test")
	resenar(t, app, token, cuerpoResena(checkin, 5))
	cal := calificacion(t, env, b.ID)
	assert.Equal(t, 11, cal.TotalResenas)
	assert.Equal(t, 4.1, cal.Promedio)        // (40 + 5) / 11
	assert.Equal(t, 4.1, cal.Aspectos.Sabor)  // (40 + 5) / 11
	assert.Equal(t, 3.8, cal.Aspectos.Espera) // (40 + 2) / 11
}

func TestResenas_EditarBorrarYUtil(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	b, _ := dueno(t, env, "duena-edit@wherever.test", reservastest.Config{})
	_, token, checkin := visita(t, env, app, b, "cli-edit@wherever.test")
	id, version := resenar(t, app, token, cuerpoResena(checkin, 5))
	path := "/api/v1/resenas/" + id

	requireStatus(t, sendJSON(t, app, http.MethodPatch, path, map[string]any{"calificacion": 3}, token), http.StatusBadRequest)
	stale := map[string]string{"If-Match": `"` + strconv.Itoa(version+3) + `"`}
	requireStatus(t, sendWithHeaders(t, app, http.MethodPatch, path, map[string]any{"calificacion": 3}, token, stale), http.StatusConflict)
	ifm := map[string]string{"If-Match": `"` + strconv.Itoa(version) + `"`}
	resp := sendWithHeaders(t, app, http.MethodPatch, path, map[string]any{"calificacion": 3}, token, ifm)
	requireStatus(t, resp, http.StatusOK)
	assert.Equal(t, `"`+strconv.Itoa(version+1)+`"`, resp.Header.Get("ETag"))
	cal := calificacion(t, env, b.ID)
	assert.Equal(t, 1, cal.TotalResenas)
	assert.Equal(t, 3.0, cal.Promedio, "editar aplica la diferencia")

	// Otra persona: 404 al editar y borrar.
	_, otro := cliente(t, env, "cli-edit-otro@wherever.test")
	ifm2 := map[string]string{"If-Match": `"` + strconv.Itoa(version+1) + `"`}
	requireStatus(t, sendWithHeaders(t, app, http.MethodPatch, path, map[string]any{"calificacion": 1}, otro, ifm2), http.StatusNotFound)
	requireStatus(t, sendJSON(t, app, http.MethodDelete, path, nil, otro), http.StatusNotFound)

	// «Me fue útil» idempotente.
	for range 2 {
		resp = sendJSON(t, app, http.MethodPut, path+"/util", map[string]any{"util": true}, otro)
		requireStatus(t, resp, http.StatusOK)
		assert.Equal(t, float64(1), decode(t, resp)["total"])
	}
	for range 2 {
		resp = sendJSON(t, app, http.MethodPut, path+"/util", map[string]any{"util": false}, otro)
		requireStatus(t, resp, http.StatusOK)
		assert.Equal(t, float64(0), decode(t, resp)["total"])
	}

	// Pasadas 48 h = 403.
	setDoc(t, env, resenas.Collection, oidDe(t, id), bson.D{{Key: "creado_en", Value: time.Now().UTC().Add(-49 * time.Hour)}})
	requireStatus(t, sendWithHeaders(t, app, http.MethodPatch, path, map[string]any{"calificacion": 1}, token, ifm2), http.StatusForbidden)

	// Borrar descuenta y es idempotente; la visita no se reutiliza.
	resp = sendJSON(t, app, http.MethodDelete, path, nil, token)
	requireStatus(t, resp, http.StatusNoContent)
	assert.Empty(t, readAll(t, resp))
	// Idempotente (spec nv-62): repetir el borrado del autor vuelve a dar 204 y no descuenta dos veces.
	requireStatus(t, sendJSON(t, app, http.MethodDelete, path, nil, token), http.StatusNoContent)
	cal = calificacion(t, env, b.ID)
	assert.Equal(t, 0, cal.TotalResenas)
	assert.Equal(t, 0.0, cal.Promedio)
	requireStatus(t, sendJSON(t, app, http.MethodPost, "/api/v1/resenas", cuerpoResena(checkin, 5), token), http.StatusConflict)
}

func TestResenas_ReportesOcultan(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	b, _ := dueno(t, env, "duena-rep@wherever.test", reservastest.Config{})
	_, autor, checkin := visita(t, env, app, b, "cli-rep@wherever.test")
	id, _ := resenar(t, app, autor, cuerpoResena(checkin, 1))
	path := "/api/v1/resenas/" + id + "/reportes"

	requireStatus(t, sendJSON(t, app, http.MethodPost, path, map[string]any{"motivo": "spam"}, autor), http.StatusForbidden)
	requireStatus(t, sendJSON(t, app, http.MethodPost, path, map[string]any{"motivo": "nada"}, autor), http.StatusUnprocessableEntity)
	for i := range 3 {
		_, tok := cliente(t, env, fmt.Sprintf("cli-rep-%d@wherever.test", i))
		requireStatus(t, sendJSON(t, app, http.MethodPost, path, map[string]any{"motivo": "falsa"}, tok), http.StatusCreated)
		if i == 0 {
			requireStatus(t, sendJSON(t, app, http.MethodPost, path, map[string]any{"motivo": "falsa"}, tok), http.StatusConflict)
		}
	}
	r := resenaDB(t, env, id)
	assert.Equal(t, resenas.EstadoOculta, r.Estado)
	assert.False(t, r.Contada)
	assert.Equal(t, 0, calificacion(t, env, b.ID).TotalResenas)
	n, err := env.Mongo.DB().Collection(moderacion.ColeccionCola).CountDocuments(context.Background(), bson.D{{Key: "referencia", Value: id}, {Key: "falla", Value: resenas.FallaReportes}})
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	// Oculta: ya no se reporta ni sale en la lista; el administrador la restablece.
	_, tarde := cliente(t, env, "cli-rep-tarde@wherever.test")
	requireStatus(t, sendJSON(t, app, http.MethodPost, path, map[string]any{"motivo": "spam"}, tarde), http.StatusNotFound)
	decidirCola(t, env, id, moderacion.EstadoAprobado)
	assert.Equal(t, resenas.EstadoPublicada, resenaDB(t, env, id).Estado)
	assert.Equal(t, 1, calificacion(t, env, b.ID).TotalResenas)
}

func TestResenas_RespuestaResumenYPanel(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	b, duenoTok := dueno(t, env, "duena-resp@wherever.test", reservastest.Config{})
	_, ajenoTok := dueno(t, env, "duena-resp-ajena@wherever.test", reservastest.Config{})
	_, token, checkin := visita(t, env, app, b, "cli-resp@wherever.test")
	id, _ := resenar(t, app, token, cuerpoResena(checkin, 4))
	base := "/api/v1/negocios/" + b.ID.Hex() + "/resenas/"

	requireStatus(t, sendJSON(t, app, http.MethodPut, base+id+"/respuesta", map[string]any{"texto": "Gracias"}, ajenoTok), http.StatusNotFound)
	requireStatus(t, sendJSON(t, app, http.MethodPut, base+id+"/respuesta", map[string]any{"texto": "Gracias"}, token), http.StatusForbidden)
	resp := sendJSON(t, app, http.MethodPut, base+id+"/respuesta", map[string]any{"texto": "¡Gracias por venir!"}, duenoTok)
	requireStatus(t, resp, http.StatusOK)
	assert.Equal(t, resenas.EstadoPublicada, obj(t, decode(t, resp)["respuesta_negocio"])["estado"])

	env.IA.Responder(`{"texto":"Gracias por tu visita, te esperamos pronto."}`)
	resp = sendJSON(t, app, http.MethodPost, base+id+"/sugerir-respuesta", nil, duenoTok)
	requireStatus(t, resp, http.StatusOK)
	assert.Equal(t, "Gracias por tu visita, te esperamos pronto.", decode(t, resp)["texto"])
	for _, p := range env.IA.Peticiones() {
		assert.NotContains(t, p.TextoUsuario+p.Contexto, "cli-resp")
	}

	resp = sendJSON(t, app, http.MethodGet, base+"resumen?periodo=mes", nil, duenoTok)
	requireStatus(t, resp, http.StatusOK)
	res := decode(t, resp)
	// `distribucion` usa las estrellas 1..5 como llaves (contrato de docs/endpoints.md): son datos,
	// no nombres de campo, así que se revisa el resto de la respuesta.
	sinDistribucion := make(map[string]any, len(res))
	for k, v := range res {
		if k != "distribucion" {
			sinDistribucion[k] = v
		}
	}
	assertSnakeCase(t, sinDistribucion)
	actual := obj(t, res["actual"])
	assert.Equal(t, float64(1), actual["total"])
	assert.Equal(t, 4.0, actual["promedio"])
	assert.Equal(t, float64(1), obj(t, res["distribucion"])["4"])
	requireStatus(t, sendJSON(t, app, http.MethodGet, base+"resumen?periodo=semana", nil, duenoTok), http.StatusUnprocessableEntity)
	requireStatus(t, sendJSON(t, app, http.MethodGet, base+"resumen", nil, ajenoTok), http.StatusNotFound)

	// P-50: la tarjeta del panel sale de las reseñas.
	env.Panel.WithFuentes(negocios.PanelFuente{Resenas: env.Resenas})
	resp = sendJSON(t, app, http.MethodGet, "/api/v1/negocios/"+b.ID.Hex()+"/panel", nil, duenoTok)
	requireStatus(t, resp, http.StatusOK)
	panel := decode(t, resp)
	assert.Equal(t, float64(1), panel["llegadas_hoy"])
	assert.Equal(t, 4.0, panel["calificacion_semana"])
	assert.Equal(t, float64(0), panel["resenas_por_responder"])
}

func TestResenas_ListaPublica(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	b, _ := dueno(t, env, "duena-lista@wherever.test", reservastest.Config{})
	u1, t1, c1 := visita(t, env, app, b, "cli-lista1@wherever.test")
	_, err := env.Users.Update(context.Background(), u1, bson.D{{Key: "nombre_usuario", Value: "viajera_lista"}})
	require.NoError(t, err)
	_, t2, c2 := visita(t, env, app, b, "cli-lista2@wherever.test")
	env.ImagenesNegocio.Publicar(u1, "foto-lista-1")

	con := cuerpoResena(c1, 5)
	con["fotos"] = []string{"foto-lista-1"}
	con["comentario"] = "Tacos deliciosos y servicio rápido"
	idFoto, _ := resenar(t, app, t1, con)
	sin := cuerpoResena(c2, 2)
	sin["comentario"] = "La espera fue larga"
	sin["aspectos"] = map[string]any{"sabor": 2, "atencion": 2, "ambiente": 2, "espera": 1, "precio_calidad": 2}
	resenar(t, app, t2, sin)

	// Foto ajena = 422.
	_, t3, c3 := visita(t, env, app, b, "cli-lista3@wherever.test")
	ajena := cuerpoResena(c3, 3)
	ajena["fotos"] = []string{"foto-lista-1"}
	requireStatus(t, sendJSON(t, app, http.MethodPost, "/api/v1/resenas", ajena, t3), http.StatusUnprocessableEntity)

	base := "/api/v1/lugares/" + b.ID.Hex() + "/resenas"
	resp := sendJSON(t, app, http.MethodGet, base+"?orden=recientes", nil, "")
	requireStatus(t, resp, http.StatusOK)
	items := list(t, decode(t, resp)["items"])
	require.Len(t, items, 2)
	for _, it := range items {
		m := obj(t, it)
		assertSnakeCase(t, m)
		for _, k := range []string{"checkin_id", "correo", "telefono", "usuario_id"} {
			assert.NotContains(t, m, k)
		}
	}
	for q, want := range map[string]int{"?estrellas=5": 1, "?aspecto=sabor": 1, "?con_fotos=true": 1, "?q=tacos": 1, "?estrellas=3": 0} {
		resp = sendJSON(t, app, http.MethodGet, base+q, nil, "")
		requireStatus(t, resp, http.StatusOK)
		got := list(t, decode(t, resp)["items"])
		require.Len(t, got, want, q)
		if want == 1 && q != "?estrellas=3" {
			assert.Equal(t, idFoto, obj(t, got[0])["id"], q)
		}
	}
	resp = sendJSON(t, app, http.MethodGet, base+"?estrellas=5", nil, "")
	autor := obj(t, obj(t, list(t, decode(t, resp)["items"])[0])["autor"])
	assert.Equal(t, "viajera_lista", autor["nombre_usuario"])
	requireStatus(t, sendJSON(t, app, http.MethodGet, base+"?orden=mejores", nil, ""), http.StatusUnprocessableEntity)
	requireStatus(t, sendJSON(t, app, http.MethodGet, "/api/v1/lugares/"+bson.NewObjectID().Hex()+"/resenas", nil, ""), http.StatusNotFound)

	// Paginación por útiles.
	resp = sendJSON(t, app, http.MethodGet, base+"?limite=1", nil, "")
	requireStatus(t, resp, http.StatusOK)
	page := decode(t, resp)
	resp = sendJSON(t, app, http.MethodGet, base+"?limite=1&cursor="+str(t, page["siguiente_cursor"]), nil, "")
	requireStatus(t, resp, http.StatusOK)
	assert.Len(t, list(t, decode(t, resp)["items"]), 1)
}

// ARCO: al eliminar la cuenta se borran check-ins y marcas de útil y las reseñas quedan
// anónimas, contadas y sin fotos; la exportación trae check-ins y reseñas.
func TestResenas_ARCO(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	b, _ := dueno(t, env, "duena-arco-res@wherever.test", reservastest.Config{})
	u, token, checkin := visita(t, env, app, b, "cli-arco-res@wherever.test")
	env.ImagenesNegocio.Publicar(u, "foto-arco")
	body := cuerpoResena(checkin, 4)
	body["fotos"] = []string{"foto-arco"}
	id, _ := resenar(t, app, token, body)
	_, otroTok, otroCheckin := visita(t, env, app, b, "cli-arco-otro@wherever.test")
	otroID, _ := resenar(t, app, otroTok, cuerpoResena(otroCheckin, 2))
	requireStatus(t, sendJSON(t, app, http.MethodPut, "/api/v1/resenas/"+otroID+"/util", map[string]any{"util": true}, token), http.StatusOK)
	requireStatus(t, sendJSON(t, app, http.MethodPost, "/api/v1/resenas/"+otroID+"/reportes", map[string]any{"motivo": "spam"}, token), http.StatusCreated)
	// Una visita sin reseña para «borrar historial».
	setDoc(t, env, resenas.CheckinsCollection, oidDe(t, checkin), bson.D{{Key: "dia", Value: "2000-01-01"}})
	llegar(t, app, b, token)

	repo := resenas.NewRepository(env.Mongo.DB())
	exp, err := resenas.NewCheckinsPrivacy(repo).Export(context.Background(), u)
	require.NoError(t, err)
	assert.Contains(t, fmt.Sprint(exp), b.ID.Hex())
	exp, err = resenas.NewPrivacy(repo).Export(context.Background(), u)
	require.NoError(t, err)
	assert.Contains(t, fmt.Sprint(exp), "Muy rica la comida")

	p := resenas.NewPrivacy(repo)
	require.NoError(t, p.PurgeHistory(context.Background(), u))
	cs, err := repo.CheckinsDe(context.Background(), u)
	require.NoError(t, err)
	require.Len(t, cs, 1, "queda solo la visita reseñada")

	require.NoError(t, p.PurgeAccount(context.Background(), u))
	cs, err = repo.CheckinsDe(context.Background(), u)
	require.NoError(t, err)
	assert.Empty(t, cs)
	r := resenaDB(t, env, id)
	assert.Nil(t, r.UsuarioID)
	assert.Empty(t, r.Fotos)
	assert.Equal(t, "Muy rica la comida", r.Comentario)
	assert.Equal(t, resenas.EstadoPublicada, r.Estado)
	assert.Equal(t, 2, calificacion(t, env, b.ID).TotalResenas, "anonimizar no mueve la calificación")
	otra := resenaDB(t, env, otroID)
	require.Len(t, otra.Reportes, 1)
	assert.Nil(t, otra.Reportes[0].Por)
	assert.Equal(t, 1, otra.Util, "el contador queda")
	n, err := env.Mongo.DB().Collection(resenas.UtilesCollection).CountDocuments(context.Background(), bson.D{{Key: "usuario_id", Value: u}})
	require.NoError(t, err)
	assert.Zero(t, n)

	resp := sendJSON(t, app, http.MethodGet, "/api/v1/lugares/"+b.ID.Hex()+"/resenas?estrellas=4", nil, "")
	requireStatus(t, resp, http.StatusOK)
	autor := obj(t, obj(t, list(t, decode(t, resp)["items"])[0])["autor"])
	assert.Equal(t, resenas.AutorAnonimo, autor["nombre_usuario"])
}

// Cinco POST simultáneos con la misma visita, tres rondas: una reseña y +1 por ronda.
func TestResenas_Concurrencia_UnaPorVisita(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	b, _ := dueno(t, env, "duena-conc-res@wherever.test", reservastest.Config{})
	for ronda := range 3 {
		_, token, checkin := visita(t, env, app, b, fmt.Sprintf("cli-conc-res-%d@wherever.test", ronda))
		var wg sync.WaitGroup
		codigos := make([]int, 5)
		for i := range 5 {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				codigos[i] = sendJSON(t, app, http.MethodPost, "/api/v1/resenas", cuerpoResena(checkin, 4), token).StatusCode
			}(i)
		}
		wg.Wait()
		creadas := 0
		for _, c := range codigos {
			if c == http.StatusCreated {
				creadas++
			} else {
				assert.Equal(t, http.StatusConflict, c)
			}
		}
		assert.Equal(t, 1, creadas, "ronda %d", ronda)
		assert.Equal(t, ronda+1, calificacion(t, env, b.ID).TotalResenas)
	}
}

// Veinte reseñas publicadas a la vez dejan la suma exacta: nada se pierde.
func TestResenas_Concurrencia_RecalculoExacto(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	b, _ := dueno(t, env, "duena-recalc@wherever.test", reservastest.Config{})
	// Directo al service: 20 escrituras HTTP simultáneas desde una IP chocan con el límite Writes.
	type visitaT struct {
		id      bson.ObjectID
		checkin string
	}
	vs := make([]visitaT, 20)
	for i := range vs {
		id, _, c := visita(t, env, app, b, fmt.Sprintf("cli-recalc-%d@wherever.test", i))
		vs[i] = visitaT{id, c}
	}
	var wg sync.WaitGroup
	var suma, espera int
	for i, v := range vs {
		estrellas := i%5 + 1
		suma += estrellas
		espera += 2
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := resenas.CrearRequest{CheckinID: v.checkin, Calificacion: estrellas, Aspectos: resenas.AspectosRequest{Sabor: estrellas, Atencion: 4, Ambiente: 3, Espera: 2, PrecioCalidad: 5}}
			out, err := env.Resenas.Crear(context.Background(), authtest.ClaimsDe(v.id.Hex(), middleware.RoleConsumer), req)
			assert.NoError(t, err)
			assert.Equal(t, resenas.EstadoPublicada, out.Estado)
		}()
	}
	wg.Wait()
	cal := calificacion(t, env, b.ID)
	assert.Equal(t, 20, cal.TotalResenas)
	assert.Equal(t, resenas.Promedio(int64(suma), 20), cal.Promedio)
	assert.Equal(t, resenas.Promedio(int64(suma), 20), cal.Aspectos.Sabor)
	assert.Equal(t, resenas.Promedio(int64(espera), 20), cal.Aspectos.Espera)
}

package integration

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/reservas"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/authtest"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/reservastest"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// cliente crea una persona con el correo verificado y su token de consumidor.
func cliente(t *testing.T, env *authtest.Env, email string) (usuarios.User, string) {
	t.Helper()
	u := env.CreateUser(t, email)
	require.NoError(t, env.Users.MarkEmailVerified(context.Background(), u.ID))
	return u, testutil.ValidTokenFor(t, u.ID.Hex(), "jti-"+email, middleware.RoleConsumer)
}

// dueno crea al comerciante y su negocio listo para reservar.
func dueno(t *testing.T, env *authtest.Env, email string, c reservastest.Config) (negocios.Business, string) {
	t.Helper()
	u := env.CreateUser(t, email)
	b := reservastest.Negocio(t, env.Mongo.DB(), u.ID, c)
	return b, merchantToken(t, u.ID, "jti-"+email)
}

func reservar(t *testing.T, app *fiber.App, token, llave string, body map[string]any) *http.Response {
	t.Helper()
	headers := map[string]string{}
	if llave != "" {
		headers["Idempotency-Key"] = llave
	}
	return sendWithHeaders(t, app, http.MethodPost, "/api/v1/reservas", body, token, headers)
}

func cuerpoReserva(b negocios.Business, hora time.Time, personas int) map[string]any {
	return map[string]any{"negocio_id": b.ID.Hex(), "fecha_hora": hora.Format(time.RFC3339), "personas": personas, "nota": "mesa en terraza"}
}

func llave(n int) string { return fmt.Sprintf("2f1c8a4e-1b7d-4c2a-9e3f-%012d", n) }

func libresA(t *testing.T, app *fiber.App, b negocios.Business, hora time.Time) int {
	t.Helper()
	resp := get(t, app, "/api/v1/lugares/"+b.ID.Hex()+"/disponibilidad?fecha="+reservastest.FechaManana()+"&personas=1")
	requireStatus(t, resp, http.StatusOK)
	body := decode(t, resp)
	horarios, ok := body["horarios"].([]any)
	require.True(t, ok)
	for _, h := range horarios {
		row := obj(t, h)
		if row["hora"] == hora.Format(time.RFC3339) {
			libres, ok := row["lugares_libres"].(float64)
			require.True(t, ok)
			return int(libres)
		}
	}
	require.Fail(t, "no está el horario", hora.String())
	return 0
}

func contar(t *testing.T, env *authtest.Env, filter bson.D) int64 {
	t.Helper()
	n, err := env.Mongo.DB().Collection(reservas.Collection).CountDocuments(context.Background(), filter)
	require.NoError(t, err)
	return n
}

func TestReservas_DisponibilidadCrearEIdempotencia(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	b, _ := dueno(t, env, "duena-res@wherever.test", reservastest.Config{Capacidad: 10})
	_, token := cliente(t, env, "cliente-res@wherever.test")
	hora := reservastest.Manana(14)

	assert.Equal(t, 10, libresA(t, app, b, hora))

	resp := reservar(t, app, token, "", cuerpoReserva(b, hora, 4))
	requireStatus(t, resp, http.StatusBadRequest)

	resp = reservar(t, app, token, llave(1), cuerpoReserva(b, hora, 4))
	requireStatus(t, resp, http.StatusCreated)
	assert.Equal(t, `"1"`, resp.Header.Get(fiber.HeaderETag))
	body := decode(t, resp)
	assertSnakeCase(t, body)
	assert.Equal(t, reservas.EstadoConfirmada, body["estado"])
	assert.Len(t, body["codigo"], 6)
	id := str(t, body["id"])

	again := reservar(t, app, token, llave(1), cuerpoReserva(b, hora, 4))
	requireStatus(t, again, http.StatusOK)
	assert.Equal(t, id, decode(t, again)["id"])
	assert.Equal(t, int64(1), contar(t, env, bson.D{{Key: "negocio_id", Value: b.ID}}), "la misma llave no crea otra")
	assert.Equal(t, 6, libresA(t, app, b, hora))

	// Sin cupo para 7: 409.
	resp = reservar(t, app, token, llave(2), cuerpoReserva(b, hora, 7))
	requireStatus(t, resp, http.StatusConflict)

	// Una hora que no es turno: 422.
	resp = reservar(t, app, token, llave(3), cuerpoReserva(b, hora.Add(30*time.Minute), 1))
	requireStatus(t, resp, http.StatusUnprocessableEntity)

	// Mis reservas y detalle.
	resp = sendJSON(t, app, http.MethodGet, "/api/v1/reservas?cuando=proximas", nil, token)
	requireStatus(t, resp, http.StatusOK)
	items, ok := decode(t, resp)["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 1)
	resp = sendJSON(t, app, http.MethodGet, "/api/v1/reservas/"+id, nil, token)
	requireStatus(t, resp, http.StatusOK)
	assert.Equal(t, false, decode(t, resp)["puede_cambiar"], "el negocio no permite cambios")
}

func TestReservas_Rechazos(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	hora := reservastest.Manana(15)
	sin, _ := dueno(t, env, "sin-res@wherever.test", reservastest.Config{SinReservas: true})
	con, _ := dueno(t, env, "anticipo-res@wherever.test", reservastest.Config{Anticipo: negocios.Anticipo{Tipo: "fijo", Valor: 100}})

	noVerificado := env.CreateUser(t, "sin-verificar@wherever.test")
	token := testutil.ValidTokenFor(t, noVerificado.ID.Hex(), "jti-sin-verificar", middleware.RoleConsumer)
	resp := reservar(t, app, token, llave(10), cuerpoReserva(con, hora, 2))
	requireStatus(t, resp, http.StatusForbidden)

	_, token = cliente(t, env, "rechazos@wherever.test")
	resp = reservar(t, app, token, llave(11), cuerpoReserva(sin, hora, 2))
	requireStatus(t, resp, http.StatusUnprocessableEntity)
	assert.Contains(t, readAll(t, resp), "no_acepta_reservas")

	resp = reservar(t, app, token, llave(12), cuerpoReserva(con, hora, 2))
	requireStatus(t, resp, http.StatusUnprocessableEntity)
	assert.Contains(t, readAll(t, resp), "anticipo_no_disponible", "hasta nv-60")

	resp = get(t, app, "/api/v1/lugares/"+sin.ID.Hex()+"/disponibilidad?fecha="+reservastest.FechaManana()+"&personas=2")
	requireStatus(t, resp, http.StatusOK)
	assert.Empty(t, decode(t, resp)["horarios"])
	assert.Zero(t, contar(t, env, bson.D{{Key: "negocio_id", Value: bson.D{{Key: "$in", Value: bson.A{sin.ID, con.ID}}}}}))
}

func TestReservas_PendienteCambiarCancelar(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	b, merchant := dueno(t, env, "manual-res@wherever.test", reservastest.Config{Capacidad: 4, Manual: true, PermiteCambios: true})
	_, token := cliente(t, env, "cambia-res@wherever.test")
	_, otro := cliente(t, env, "otro-res@wherever.test")
	hora := reservastest.Manana(13)

	resp := reservar(t, app, token, llave(20), cuerpoReserva(b, hora, 2))
	requireStatus(t, resp, http.StatusCreated)
	body := decode(t, resp)
	assert.Equal(t, reservas.EstadoPendiente, body["estado"])
	id := str(t, body["id"])
	path := "/api/v1/reservas/" + id

	// Llena las 13:00 con otra persona para que el cambio a 4 no quepa.
	requireStatus(t, reservar(t, app, otro, llave(21), cuerpoReserva(b, hora, 2)), http.StatusCreated)

	resp = sendJSON(t, app, http.MethodPatch, path, map[string]any{"personas": 3}, token)
	requireStatus(t, resp, http.StatusBadRequest)
	resp = sendWithHeaders(t, app, http.MethodPatch, path, map[string]any{"personas": 3}, token, map[string]string{"If-Match": `"1"`})
	requireStatus(t, resp, http.StatusConflict)

	resp = sendWithHeaders(t, app, http.MethodPatch, path, map[string]any{"fecha_hora": hora.Add(time.Hour).Format(time.RFC3339), "personas": 4}, token, map[string]string{"If-Match": `"1"`})
	requireStatus(t, resp, http.StatusOK)
	body = decode(t, resp)
	assert.Equal(t, float64(4), body["personas"])
	assert.Equal(t, float64(2), body["version"])

	resp = sendWithHeaders(t, app, http.MethodPatch, path, map[string]any{"personas": 1}, token, map[string]string{"If-Match": `"1"`})
	requireStatus(t, resp, http.StatusConflict)

	// La otra persona no ve esta reserva.
	requireStatus(t, sendJSON(t, app, http.MethodGet, path, nil, otro), http.StatusNotFound)

	// El negocio la confirma.
	resp = sendJSON(t, app, http.MethodPatch, "/api/v1/negocios/"+b.ID.Hex()+"/reservas/"+id, map[string]any{"estado": "confirmada"}, merchant)
	requireStatus(t, resp, http.StatusOK)
	assert.Equal(t, reservas.EstadoConfirmada, decode(t, resp)["estado"])

	resp = sendJSON(t, app, http.MethodGet, path+"/reembolso", nil, token)
	requireStatus(t, resp, http.StatusOK)
	body = decode(t, resp)
	assert.Equal(t, float64(0), body["reembolso_centavos"], "sin pago")
	assert.Equal(t, "flexible", obj(t, body["politica"])["tipo"])

	resp = sendJSON(t, app, http.MethodPost, path+"/cancelar", map[string]any{"motivo": "cambio de planes"}, token)
	requireStatus(t, resp, http.StatusOK)
	assert.Equal(t, reservas.EstadoCancelada, decode(t, resp)["estado"])
	requireStatus(t, sendJSON(t, app, http.MethodPost, path+"/cancelar", nil, token), http.StatusConflict)
	assert.Equal(t, 4, libresA(t, app, b, hora.Add(time.Hour)), "cancelar libera el cupo")
}

func TestReservas_AgendaEscanearYBloqueos(t *testing.T) {
	env := authEnv(t)
	app := env.App()
	b, merchant := dueno(t, env, "agenda-res@wherever.test", reservastest.Config{Capacidad: 10})
	otroNegocio, _ := dueno(t, env, "agenda-otro@wherever.test", reservastest.Config{})
	u, _ := cliente(t, env, "agenda-cliente@wherever.test")
	_, err := env.Users.Update(context.Background(), u.ID, bson.D{{Key: "nombre", Value: "Ángela Ruiz"}})
	require.NoError(t, err)

	repo := reservas.NewRepository(env.Mongo.DB())
	sembrar := func(codigo string, negocio bson.ObjectID, hora time.Time, estado string) {
		now := time.Now().UTC()
		require.NoError(t, repo.Insert(context.Background(), &reservas.Reserva{ID: bson.NewObjectID(), Codigo: codigo, UsuarioID: u.ID, NegocioID: negocio,
			FechaHora: hora, Personas: 2, Estado: estado, Politica: reservas.PoliticaDe("flexible"), Historial: []reservas.Cambio{}, Version: 1, CreadoEn: now, ActualizadoEn: now}))
	}
	y, m, d := time.Now().In(negocios.MexicoCity()).Date()
	hoy := time.Date(y, m, d, 12, 0, 0, 0, negocios.MexicoCity()).UTC()
	sembrar("HOY234", b.ID, hoy, reservas.EstadoConfirmada)
	sembrar("MAN234", b.ID, reservastest.Manana(12), reservas.EstadoConfirmada)
	sembrar("OTR234", otroNegocio.ID, hoy, reservas.EstadoConfirmada)

	base := "/api/v1/negocios/" + b.ID.Hex()
	resp := sendJSON(t, app, http.MethodGet, base+"/reservas?q=angela", nil, merchant)
	requireStatus(t, resp, http.StatusOK)
	raw := readAll(t, resp)
	assert.Contains(t, raw, "HOY234")
	assert.Contains(t, raw, "Ángela Ruiz")
	assert.NotContains(t, raw, "agenda-cliente@", "sin correo del cliente")
	resp = sendJSON(t, app, http.MethodGet, base+"/reservas?q=zz", nil, merchant)
	requireStatus(t, resp, http.StatusOK)
	assert.Empty(t, decode(t, resp)["items"])

	escanear := func(codigo string) int {
		return sendJSON(t, app, http.MethodPost, base+"/reservas/escanear", map[string]any{"codigo": codigo}, merchant).StatusCode
	}
	assert.Equal(t, http.StatusOK, escanear("HOY234"))
	assert.Equal(t, http.StatusConflict, escanear("HOY234"), "ya usado")
	assert.Equal(t, http.StatusNotFound, escanear("MAN234"), "otro día")
	assert.Equal(t, http.StatusNotFound, escanear("OTR234"), "otro negocio")
	assert.Equal(t, http.StatusNotFound, escanear("ZZZ234"), "no existe")

	// Bloqueo de las 16:00 de mañana: 0 lugares y POST = 409; borrarlo las abre.
	hora := reservastest.Manana(16)
	resp = sendJSON(t, app, http.MethodPost, base+"/bloqueos", map[string]any{"desde": hora.Format(time.RFC3339), "hasta": hora.Add(time.Hour).Format(time.RFC3339), "motivo": "evento_privado"}, merchant)
	requireStatus(t, resp, http.StatusCreated)
	bid := str(t, decode(t, resp)["id"])
	assert.Equal(t, 0, libresA(t, app, b, hora))
	_, token := cliente(t, env, "bloqueo-cliente@wherever.test")
	requireStatus(t, reservar(t, app, token, llave(30), cuerpoReserva(b, hora, 1)), http.StatusConflict)

	resp = sendJSON(t, app, http.MethodPost, base+"/bloqueos", map[string]any{"desde": hora.Format(time.RFC3339), "hasta": hora.AddDate(0, 0, 32).Format(time.RFC3339)}, merchant)
	requireStatus(t, resp, http.StatusUnprocessableEntity)

	resp = sendJSON(t, app, http.MethodDelete, base+"/bloqueos/"+bid, nil, merchant)
	requireStatus(t, resp, http.StatusNoContent)
	assert.Empty(t, readAll(t, resp), "204 sin cuerpo")
	assert.Equal(t, 10, libresA(t, app, b, hora))
	requireStatus(t, sendJSON(t, app, http.MethodDelete, base+"/bloqueos/"+bid, nil, merchant), http.StatusNotFound)
}

func TestReservas_BarridoYPrivacidad(t *testing.T) {
	env := authEnv(t)
	b, _ := dueno(t, env, "barrido-res@wherever.test", reservastest.Config{})
	u, _ := cliente(t, env, "barrido-cliente@wherever.test")
	repo := reservas.NewRepository(env.Mongo.DB())
	now := time.Now().UTC()
	vencio := now.Add(-time.Minute)
	sembrar := func(hora time.Time, estado string, expira *time.Time) bson.ObjectID {
		r := reservas.Reserva{ID: bson.NewObjectID(), Codigo: fmt.Sprintf("B%05d", hora.Unix()%100000), UsuarioID: u.ID, NegocioID: b.ID, FechaHora: hora, Personas: 2,
			Nota: "alergia a nueces", Estado: estado, BloqueoExpiraEn: expira, Historial: []reservas.Cambio{}, Version: 1, CreadoEn: now, ActualizadoEn: now}
		require.NoError(t, repo.Insert(context.Background(), &r))
		return r.ID
	}
	apartado := sembrar(reservastest.Manana(10), reservas.EstadoBloqueada, &vencio)
	ayer := sembrar(now.AddDate(0, 0, -2), reservas.EstadoConfirmada, nil)
	futura := sembrar(reservastest.Manana(11), reservas.EstadoConfirmada, nil)

	expiradas, ausentes, err := env.Reservas.Barrer(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(1), expiradas)
	assert.Equal(t, int64(1), ausentes)
	for id, estado := range map[bson.ObjectID]string{apartado: reservas.EstadoExpirada, ayer: reservas.EstadoNoAsistio, futura: reservas.EstadoConfirmada} {
		r, err := repo.FindByID(context.Background(), id)
		require.NoError(t, err)
		assert.Equal(t, estado, r.Estado)
	}
	again, _, err := env.Reservas.Barrer(context.Background())
	require.NoError(t, err)
	assert.Zero(t, again, "idempotente")

	p := reservas.NewPrivacy(repo)
	exported, err := p.Export(context.Background(), u.ID)
	require.NoError(t, err)
	assert.Len(t, exported, 3)

	require.NoError(t, p.PurgeAccount(context.Background(), u.ID))
	r, err := repo.FindByID(context.Background(), futura)
	require.NoError(t, err)
	assert.Equal(t, reservas.EstadoCancelada, r.Estado, "las futuras se cancelan")
	assert.Empty(t, r.Nota)
	r, err = repo.FindByID(context.Background(), ayer)
	require.NoError(t, err)
	assert.Equal(t, reservas.EstadoNoAsistio, r.Estado, "las pasadas se conservan")
	assert.Empty(t, r.Nota, "sin nota")
}

func TestReservas_LimitePorHora(t *testing.T) {
	env := authEnv(t)
	env.ClearRateLimits(t)
	app := env.App()
	b, _ := dueno(t, env, "limite-res@wherever.test", reservastest.Config{Capacidad: 50})
	_, token := cliente(t, env, "limite-cliente@wherever.test")
	for i := range 10 {
		requireStatus(t, reservar(t, app, token, llave(100+i), cuerpoReserva(b, reservastest.Manana(8+i), 1)), http.StatusCreated)
	}
	requireStatus(t, reservar(t, app, token, llave(200), cuerpoReserva(b, reservastest.Manana(20), 1)), http.StatusTooManyRequests)
}

// 20 personas piden el último lugar a la vez, en 3 rondas: gana una por ronda y el cupo nunca
// queda negativo.
func TestReservas_Concurrencia_UnSoloGanador(t *testing.T) {
	env := authEnv(t)
	b, _ := dueno(t, env, "concurrencia-res@wherever.test", reservastest.Config{Capacidad: 3})
	personas := make([]*middleware.Claims, 20)
	for i := range personas {
		u, _ := cliente(t, env, fmt.Sprintf("concurrente-%02d@wherever.test", i))
		personas[i] = authtest.ClaimsDe(u.ID.Hex(), middleware.RoleConsumer)
	}
	for ronda := range 3 {
		hora := reservastest.Manana(9 + ronda)
		// Dos de los tres lugares ya están tomados.
		_, _, err := env.Reservas.Crear(context.Background(), personas[0], llave(300+ronda), reservas.CrearRequest{NegocioID: b.ID.Hex(), FechaHora: hora.Format(time.RFC3339), Personas: 2})
		require.NoError(t, err)

		var wg sync.WaitGroup
		var mu sync.Mutex
		ganadores, ocupados := 0, 0
		for i, cl := range personas {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, _, err := env.Reservas.Crear(context.Background(), cl, llave(1000+ronda*100+i), reservas.CrearRequest{NegocioID: b.ID.Hex(), FechaHora: hora.Format(time.RFC3339), Personas: 1})
				mu.Lock()
				defer mu.Unlock()
				var ae *apperr.Error
				switch {
				case err == nil:
					ganadores++
				case errors.As(err, &ae) && ae.Kind == apperr.KindConflict:
					ocupados++
				default:
					t.Errorf("ronda %d: %v", ronda, err)
				}
			}()
		}
		wg.Wait()
		assert.Equal(t, 1, ganadores, "ronda %d", ronda)
		assert.Equal(t, 19, ocupados, "ronda %d", ronda)
		rs, err := reservas.NewRepository(env.Mongo.DB()).DelHorario(context.Background(), b.ID, hora)
		require.NoError(t, err)
		total := 0
		for _, r := range rs {
			total += r.Personas
		}
		assert.Equal(t, 3, total, "nunca más que la capacidad")
	}
}
